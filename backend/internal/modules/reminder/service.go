package reminder

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/modules/inbox"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/bn"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/money"
	"provider_saas/internal/platform/pagination"
)

// Job kinds.
const (
	JobPlan     = "reminder.plan"
	JobDispatch = "reminder.dispatch"
)

// Dependencies (wired in app.Container).
type (
	Messenger interface {
		SendSystem(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) (inbox.Message, error)
	}
	Templates interface {
		Template(ctx context.Context, tenantID, id uuid.UUID) (inbox.Template, error)
		TemplateByKey(ctx context.Context, tenantID uuid.UUID, key string) (inbox.Template, error)
	}
	// Portal creates the {{link}} magic link (storefront.Service).
	Portal interface {
		CreatePortalLink(ctx context.Context, tenantID, customerID uuid.UUID, sendVia string) (string, error)
	}
	Tenants interface {
		Get(ctx context.Context, tenantID uuid.UUID) (tenant.Tenant, error)
		Notify(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, kind, title string, body, link *string) error
	}
	Subscriptions interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (subscription.Subscription, error)
	}
)

type Deps struct {
	Messenger     Messenger
	Templates     Templates
	Portal        Portal
	Tenants       Tenants
	Subscriptions Subscriptions
}

// Service is the reminder module API.
type Service interface {
	Rules(ctx context.Context, tenantID uuid.UUID) ([]Rule, error)
	RulesByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Rule, error)
	UpsertRule(ctx context.Context, tenantID uuid.UUID, id *uuid.UUID, in RuleInput) (Rule, error)
	DeleteRule(ctx context.Context, tenantID, id uuid.UUID) error
	List(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) ([]Reminder, error)
	SendNow(ctx context.Context, tenantID, subscriptionID uuid.UUID, templateID *uuid.UUID, channel string) (Reminder, error)
	Cancel(ctx context.Context, tenantID, id uuid.UUID) error

	// Plan materialises reminders for one tenant (reminder.plan job). Returns rows created.
	Plan(ctx context.Context, tenantID uuid.UUID, now time.Time) (int, error)
	// Dispatch sends due reminders of one tenant (reminder.dispatch job).
	Dispatch(ctx context.Context, tenantID uuid.UUID, now time.Time) (sent, failed int, err error)
	TenantsWithDue(ctx context.Context, now time.Time) ([]uuid.UUID, error)
}

type service struct {
	pool *pgxpool.Pool
	repo Repository
	d    Deps
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, d Deps) Service {
	return &service{pool: pool, d: d, now: time.Now}
}

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func (s *service) Rules(ctx context.Context, tenantID uuid.UUID) (out []Rule, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.Rules(ctx, tx, tenantID, false)
		return err
	})
	return out, err
}

func (s *service) RulesByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Rule, error) {
	out := map[uuid.UUID]Rule{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rs, err := s.repo.RulesByIDs(ctx, tx, tenantID, ids)
		for _, r := range rs {
			out[r.ID] = r
		}
		return err
	})
	return out, err
}

func (s *service) UpsertRule(ctx context.Context, tenantID uuid.UUID, id *uuid.UUID, in RuleInput) (r Rule, err error) {
	if err := in.Validate(); err != nil {
		return r, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if in.TemplateID != nil {
			if _, err := s.d.Templates.Template(ctx, tenantID, *in.TemplateID); err != nil {
				return apperr.ValidationField("templateId", "template not found")
			}
		}
		if id == nil {
			r, err = s.repo.InsertRule(ctx, tx, tenantID, in)
			return err
		}
		if r, err = s.repo.UpdateRule(ctx, tx, tenantID, *id, in); err != nil {
			return err
		}
		// Timing/channel may have changed: re-plan from scratch on the next planner run.
		return s.repo.CancelScheduledForRule(ctx, tx, tenantID, *id)
	})
	return r, err
}

func (s *service) DeleteRule(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.DeleteRule(ctx, tx, tenantID, id)
		if err == nil && !ok {
			return apperr.NotFound("reminder rule")
		}
		return err
	})
}

func (s *service) List(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) (out []Reminder, err error) {
	if f.Status != nil {
		v := strings.ToLower(*f.Status)
		f.Status = &v
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.repo.List(ctx, tx, tenantID, f, page)
		out, _ = pagination.Trim(rows, page.First)
		return err
	})
	return out, err
}

func (s *service) Cancel(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.Get(ctx, tx, tenantID, id); err != nil {
			return err
		}
		ok, err := s.repo.Cancel(ctx, tx, tenantID, id)
		if err == nil && !ok {
			return apperr.Validation("only scheduled reminders can be cancelled")
		}
		return err
	})
}

// ---- planner

func (s *service) Plan(ctx context.Context, tenantID uuid.UUID, now time.Time) (created int, err error) {
	t, err := s.d.Tenants.Get(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	loc := bn.Location(t.Timezone)
	lo, hi := now.Add(-PlanGrace), now.Add(PlanHorizon)
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rules, err := s.repo.Rules(ctx, tx, tenantID, true)
		if err != nil {
			return err
		}
		for _, r := range rules {
			shift := DayShift(r.Trigger, r.OffsetDays)
			// Anchor window: send time in [lo, hi] <=> anchor in [lo - shift, hi - shift] (±1 day for local-day rounding).
			from := lo.AddDate(0, 0, -shift-1)
			to := hi.AddDate(0, 0, -shift+1)
			var anchors []Anchor
			switch r.Trigger {
			case BeforeExpiry, OnExpiry:
				anchors, err = s.repo.SubscriptionAnchors(ctx, tx, tenantID, r.ProductID, false, from, to)
			case AfterExpiry:
				anchors, err = s.repo.SubscriptionAnchors(ctx, tx, tenantID, r.ProductID, true, from, to)
			case InvoiceDue, InvoiceOverdue:
				anchors, err = s.repo.InvoiceAnchors(ctx, tx, tenantID, from, to)
			case MasterExpiry:
				anchors, err = s.repo.MasterAnchors(ctx, tx, tenantID, r.ProductID, from, to)
			}
			if err != nil {
				return err
			}
			for _, a := range anchors {
				at := SendAt(a.At, shift, r.SendHour, loc)
				if at.Before(lo) || at.After(hi) {
					continue
				}
				// A "before expiry" reminder after the subscription ended is pointless.
				if (r.Trigger == BeforeExpiry || r.Trigger == InvoiceDue) && !at.Before(a.At) {
					continue
				}
				rem := Reminder{RuleID: &r.ID, CustomerID: a.CustomerID, Channel: r.Channel, ScheduledAt: at}
				switch r.Trigger {
				case BeforeExpiry, OnExpiry, AfterExpiry:
					rem.SubscriptionID = &a.ID
				case InvoiceDue, InvoiceOverdue:
					rem.InvoiceID = &a.ID
				case MasterExpiry:
					rem.MasterAccountID, rem.Channel = &a.ID, ChannelInApp
				}
				ok, err := s.repo.Upsert(ctx, tx, tenantID, rem)
				if err != nil {
					return err
				}
				if ok {
					created++
				}
			}
		}
		return nil
	})
	return created, err
}

// ---- dispatcher

// TenantsWithDue fans the dispatcher out. reminders is a tenant table (fail-closed RLS),
// so each active tenant is checked inside its own tenant transaction.
func (s *service) TenantsWithDue(ctx context.Context, now time.Time) (out []uuid.UUID, err error) {
	var all []uuid.UUID
	err = db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		all, err = s.repo.ActiveTenants(ctx, tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	for _, tid := range all {
		var due bool
		if err := s.tx(ctx, tid, func(ctx context.Context, tx pgx.Tx) (err error) {
			due, err = s.repo.HasDue(ctx, tx, tid, now)
			return err
		}); err != nil {
			return out, err
		}
		if due {
			out = append(out, tid)
		}
	}
	return out, nil
}

const dispatchBatch = 100

// Dispatch renders and queues due reminders in one tenant transaction. Messages are
// queued as inbox.send jobs (joined tx), so no external API call happens while rows are locked.
func (s *service) Dispatch(ctx context.Context, tenantID uuid.UUID, now time.Time) (sent, failed int, err error) {
	t, err := s.d.Tenants.Get(ctx, tenantID)
	if err != nil {
		return 0, 0, err
	}
	loc := bn.Location(t.Timezone)
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		due, err := s.repo.Due(ctx, tx, tenantID, now, dispatchBatch)
		if err != nil {
			return err
		}
		rules := map[uuid.UUID]Rule{}
		if len(due) > 0 {
			rs, err := s.repo.Rules(ctx, tx, tenantID, false)
			if err != nil {
				return err
			}
			for _, r := range rs {
				rules[r.ID] = r
			}
		}
		for _, rem := range due {
			var rule *Rule
			if rem.RuleID != nil {
				if r, ok := rules[*rem.RuleID]; ok {
					rule = &r
				}
			}
			status, body, errMsg := s.dispatchOne(ctx, tx, tenantID, rem, rule, nil, loc)
			var sentAt *time.Time
			if status == StatusSent {
				sent++
				sentAt = &now
			} else if status == StatusFailed {
				failed++
			}
			if err := s.repo.SetResult(ctx, tx, tenantID, rem.ID, status, body, errMsg, sentAt); err != nil {
				return err
			}
		}
		return nil
	})
	return sent, failed, err
}

// dispatchOne renders and sends one reminder; it never returns an error (the outcome is
// recorded on the row). A savepoint keeps one failing reminder from aborting the batch.
func (s *service) dispatchOne(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, rem Reminder, rule *Rule, templateID *uuid.UUID, loc *time.Location) (status string, body, errMsg *string) {
	fail := func(err error) (string, *string, *string) {
		e := err.Error()
		return StatusFailed, nil, &e
	}
	sp, err := tx.Begin(ctx) // savepoint
	if err != nil {
		return fail(err)
	}
	status, body, errMsg = s.render(ctx, sp, tenantID, rem, rule, templateID, loc)
	if status != StatusScheduled {
		_ = sp.Rollback(ctx)
		return status, body, errMsg
	}
	if err := s.deliver(ctx, tenantID, rem, *body); err != nil {
		_ = sp.Rollback(ctx)
		e := err.Error()
		return StatusFailed, body, &e
	}
	if err := sp.Commit(ctx); err != nil {
		return fail(err)
	}
	return StatusSent, body, nil
}

// render returns StatusScheduled + body when the reminder should be sent, else skipped/failed.
func (s *service) render(ctx context.Context, q pgx.Tx, tenantID uuid.UUID, rem Reminder, rule *Rule, templateID *uuid.UUID, loc *time.Location) (string, *string, *string) {
	skip := func(why string) (string, *string, *string) { return StatusSkipped, nil, &why }
	rc, err := s.repo.LoadContext(ctx, q, tenantID, rem)
	if errors.Is(err, apperr.ErrNotFound) {
		return skip("target no longer exists")
	}
	if err != nil {
		e := err.Error()
		return StatusFailed, nil, &e
	}
	if rem.SubscriptionID != nil && !rc.SubscriptionOK {
		return skip("subscription renewed or cancelled")
	}
	if rem.InvoiceID != nil && !rc.InvoiceOpen {
		return skip("invoice already paid")
	}
	tpl := ""
	trigger := ""
	if rule != nil {
		trigger = rule.Trigger
		if templateID == nil {
			templateID = rule.TemplateID
		}
	}
	if templateID != nil {
		t, err := s.d.Templates.Template(ctx, tenantID, *templateID)
		if err == nil && t.IsActive {
			tpl = t.Body
		}
	}
	if tpl == "" {
		if trigger == "" {
			trigger = BeforeExpiry
			if rc.SubscriptionSts == "expired" {
				trigger = AfterExpiry
			}
		}
		tpl = DefaultBody(trigger)
	}
	vars := map[string]string{"customer": rc.CustomerName, "product": rc.ProductName}
	if rc.ExpiresAt != nil {
		vars["expiry"] = bn.Date(*rc.ExpiresAt, loc)
	}
	if rc.Amount > 0 {
		vars["amount"] = money.Format(rc.Amount, rc.Currency, true)
	}
	if rem.CustomerID != nil && strings.Contains(tpl, "link") {
		link, err := s.d.Portal.CreatePortalLink(ctx, tenantID, *rem.CustomerID, "")
		if err != nil {
			e := "portal link: " + err.Error()
			return StatusFailed, nil, &e
		}
		vars["link"] = link
	}
	b := strings.TrimSpace(inbox.Render(tpl, vars))
	return StatusScheduled, &b, nil
}

func (s *service) deliver(ctx context.Context, tenantID uuid.UUID, rem Reminder, body string) error {
	switch rem.Channel {
	case ChannelInApp:
		title := "রিমাইন্ডার"
		link := "/reminders"
		return s.d.Tenants.Notify(ctx, tenantID, nil, "reminder", title, &body, &link)
	case "sms", "email":
		return apperr.Validation(rem.Channel + " reminders are not supported yet; use preferred or a social channel")
	}
	if rem.CustomerID == nil {
		return apperr.Validation("reminder has no customer")
	}
	ch := rem.Channel
	if ch == ChannelPreferred {
		ch = ""
	}
	_, err := s.d.Messenger.SendSystem(ctx, tenantID, *rem.CustomerID, ch, body)
	return err
}

// SendNow renders and sends a reminder for a subscription immediately and records it.
func (s *service) SendNow(ctx context.Context, tenantID, subscriptionID uuid.UUID, templateID *uuid.UUID, channel string) (out Reminder, err error) {
	channel = strings.ToLower(channel)
	if channel == "" {
		channel = ChannelPreferred
	}
	if !validChannel[channel] {
		return out, apperr.ValidationField("channel", "invalid channel")
	}
	sub, err := s.d.Subscriptions.Get(ctx, tenantID, subscriptionID)
	if err != nil {
		return out, err
	}
	t, err := s.d.Tenants.Get(ctx, tenantID)
	if err != nil {
		return out, err
	}
	now := s.now()
	rem := Reminder{CustomerID: &sub.CustomerID, SubscriptionID: &sub.ID, Channel: channel, ScheduledAt: now}
	var status string
	var body, errMsg *string
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		status, body, errMsg = s.render(ctx, tx, tenantID, rem, nil, templateID, bn.Location(t.Timezone))
		if status == StatusSkipped { // explicit request: send even for renewed subscriptions
			status, errMsg = StatusScheduled, nil
			rc, err := s.repo.LoadContext(ctx, tx, tenantID, rem)
			if err != nil {
				return err
			}
			b := inbox.Render(DefaultBody(BeforeExpiry), map[string]string{"customer": rc.CustomerName, "product": rc.ProductName,
				"expiry": bn.Date(*rc.ExpiresAt, bn.Location(t.Timezone))})
			body = &b
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	if status == StatusScheduled {
		// Outside the tx: the message is sent inline right away.
		if derr := s.deliver(ctx, tenantID, rem, *body); derr != nil {
			if !errors.Is(derr, apperr.ErrValidation) {
				return out, derr
			}
			status, errMsg = StatusFailed, ptr(derr.Error())
		} else {
			status, rem.SentAt = StatusSent, &now
		}
	}
	rem.Status, rem.RenderedBody, rem.Error = status, body, errMsg
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.Insert(ctx, tx, tenantID, rem)
		return err
	})
	return out, err // a FAILED reminder (with error) is returned, not raised
}

func ptr(s string) *string { return &s }
