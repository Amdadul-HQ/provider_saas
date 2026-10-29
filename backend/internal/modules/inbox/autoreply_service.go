package inbox

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/apperr"
)

// whatsAppTemplate returns the approved template configured for a system-message purpose,
// or a validation error that tells the seller what to configure.
func (s *service) whatsAppTemplate(ctx context.Context, tenantID uuid.UUID, purpose string) (WATemplateSpec, error) {
	if purpose == "" {
		return WATemplateSpec{}, apperr.Validation("the 24-hour WhatsApp window is closed: free-form messages cannot be sent until the customer writes again")
	}
	t, err := s.members.Get(ctx, tenantID)
	if err != nil {
		return WATemplateSpec{}, err
	}
	spec, ok := WhatsAppTemplateFor(t.Settings, t.Locale, purpose)
	if !ok {
		return spec, apperr.Validation("the 24-hour WhatsApp window is closed; set an approved template for \"" + purpose +
			"\" in settings.whatsapp_templates to message this customer")
	}
	return spec, nil
}

// queueAutoReply (inside the ingest tx) queues inbox.autoreply for an inbound text when the
// tenant has active rules. The job runs in the worker, so the webhook answers immediately.
func (s *service) queueAutoReply(ctx context.Context, tx pgx.Tx, tid uuid.UUID, conv Conversation, m Message) error {
	if m.Kind != "text" || m.Body == nil || strings.TrimSpace(*m.Body) == "" || conv.Status == StatusSpam || s.queue == nil {
		return nil
	}
	ok, err := s.repo.HasActiveAutoReplyRules(ctx, tx, tid)
	if err != nil || !ok {
		return err
	}
	_, err = s.queue.Enqueue(ctx, JobAutoReply, AutoReplyPayload{TenantID: tid, MessageID: m.ID}, s.now(), JobAutoReply+":"+m.ID.String())
	return err
}

func (s *service) AutoReplyRules(ctx context.Context, tenantID uuid.UUID) (out []AutoReplyRule, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.AutoReplyRules(ctx, tx, tenantID, false)
		return err
	})
	return out, err
}

func (s *service) UpsertAutoReplyRule(ctx context.Context, tenantID uuid.UUID, id *uuid.UUID, in AutoReplyInput) (r AutoReplyRule, err error) {
	if err := in.Validate(); err != nil {
		return r, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if in.TemplateID != nil {
			if _, err := s.repo.Template(ctx, tx, tenantID, *in.TemplateID); err != nil {
				return apperr.ValidationField("templateId", "template not found")
			}
		}
		if id == nil {
			r, err = s.repo.InsertAutoReplyRule(ctx, tx, tenantID, in)
		} else {
			r, err = s.repo.UpdateAutoReplyRule(ctx, tx, tenantID, *id, in)
		}
		return err
	})
	return r, err
}

func (s *service) DeleteAutoReplyRule(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.DeleteAutoReplyRule(ctx, tx, tenantID, id)
		if err == nil && !ok {
			return apperr.NotFound("auto-reply rule")
		}
		return err
	})
}

// replyBody is the rule's template body (if the template still exists and is active),
// else its own body.
func (s *service) replyBody(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, r AutoReplyRule) (string, error) {
	if r.TemplateID != nil {
		t, err := s.repo.Template(ctx, tx, tenantID, *r.TemplateID)
		if err == nil && t.IsActive {
			return t.Body, nil
		}
		if err != nil && !errors.Is(err, apperr.ErrNotFound) {
			return "", err
		}
	}
	if r.Body != nil {
		return *r.Body, nil
	}
	return "", nil
}

// openNow reports whether the business is inside its business hours now.
func (s *service) openNow(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	t, err := s.members.Get(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return ParseBusinessHours(t.Settings).Open(TenantLocal(s.now(), t.Timezone)), nil
}

func (s *service) TestAutoReply(ctx context.Context, tenantID uuid.UUID, text, platform string) (res AutoReplyTest, err error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rules, err := s.repo.AutoReplyRules(ctx, tx, tenantID, true)
		if err != nil {
			return err
		}
		open, err := s.openNow(ctx, tenantID)
		if err != nil {
			return err
		}
		r, ok := FirstMatch(rules, text, platform, open)
		if !ok {
			return nil
		}
		body, err := s.replyBody(ctx, tx, tenantID, r)
		if err != nil {
			return err
		}
		rendered := strings.TrimSpace(Render(body, map[string]string{"customer": "Customer"}))
		res = AutoReplyTest{Matched: true, Rule: &r, RenderedBody: &rendered}
		return nil
	})
	return res, err
}

// AutoReply answers one inbound message when an active rule matches, at most once per
// conversation per cooldown. The reply is a system message (kind template, no sender)
// queued as inbox.send and sent right after commit.
func (s *service) AutoReply(ctx context.Context, p AutoReplyPayload) error {
	var sent Message
	var jobID int64
	err := s.tx(ctx, p.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		m, err := s.repo.GetMessage(ctx, tx, p.TenantID, p.MessageID, false)
		if errors.Is(err, apperr.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if m.Direction != Inbound || m.Body == nil {
			return nil
		}
		// Row lock: concurrent auto replies on one conversation serialise on the cooldown.
		conv, err := s.repo.GetConversation(ctx, tx, p.TenantID, m.ConversationID, true)
		if err != nil {
			return err
		}
		now := s.now()
		if conv.Status == StatusSpam || !conv.CanReply(now) {
			return nil
		}
		rules, err := s.repo.AutoReplyRules(ctx, tx, p.TenantID, true)
		if err != nil || len(rules) == 0 {
			return err
		}
		open, err := s.openNow(ctx, p.TenantID)
		if err != nil {
			return err
		}
		r, ok := FirstMatch(rules, *m.Body, conv.Platform, open)
		if !ok {
			return nil
		}
		if conv.LastAutoReplyAt != nil && now.Sub(*conv.LastAutoReplyAt) < time.Duration(r.CooldownMinutes)*time.Minute {
			return nil
		}
		body, err := s.replyBody(ctx, tx, p.TenantID, r)
		if err != nil {
			return err
		}
		vars, err := s.defaultVars(ctx, p.TenantID, conv)
		if err != nil {
			return err
		}
		body = strings.TrimSpace(Render(body, vars))
		if body == "" {
			return nil
		}
		if sent, jobID, err = s.queueMessage(ctx, tx, p.TenantID, conv, "template", body, "", nil, nil, "", nil, false); err != nil {
			return err
		}
		return s.repo.MarkAutoReplied(ctx, tx, p.TenantID, conv.ID, sent.ID, r.ID)
	})
	if err != nil || jobID == 0 {
		return err
	}
	s.runInline(ctx, p.TenantID, sent, jobID)
	return nil
}
