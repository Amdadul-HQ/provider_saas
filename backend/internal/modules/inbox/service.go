package inbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/social"
	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
	"provider_saas/internal/platform/phone"
)

// JobSend is the job kind that delivers one queued outbound message.
const JobSend = "inbox.send"

// SendPayload is the inbox.send job payload.
type SendPayload struct {
	TenantID  uuid.UUID `json:"tenant_id"`
	MessageID uuid.UUID `json:"message_id"`
	Tag       string    `json:"tag,omitempty"`
	// WATemplate sends an approved WhatsApp template instead of free text (outside 24h).
	WATemplate *WATemplateSpec `json:"wa_template,omitempty"`
	// SealedBody is the real text of a message whose stored body is redacted (credential
	// delivery). Sealed with vault.SealTransient (purpose = message id); the worker removes
	// it from the job row once the job finishes (jobs.SealedPayloadKey).
	SealedBody string `json:"sealed_body,omitempty"`
}

// OutsideWindowTag is the Messenger message tag used for system messages (renewal
// reminders, invoices, delivery) outside the 24h window.
const OutsideWindowTag = "ACCOUNT_UPDATE"

// Dependencies (wired in app.Container).
type (
	Social interface {
		ResolveInbound(ctx context.Context, platform, externalID string) (social.Account, error)
		AccountByID(ctx context.Context, id uuid.UUID) (social.Account, error)
		Get(ctx context.Context, tenantID, id uuid.UUID) (social.Account, error)
		Send(ctx context.Context, a social.Account, thread, body string, att []social.Attachment) (string, error)
	}
	Customers interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (customer.Customer, error)
		Create(ctx context.Context, tenantID, actorID uuid.UUID, in customer.CreateInput) (customer.Customer, error)
		FindIdentity(ctx context.Context, tenantID uuid.UUID, platform, externalID string) (customer.Identity, error)
		FindByPhone(ctx context.Context, tenantID uuid.UUID, phone string) (customer.Customer, error)
		AddIdentity(ctx context.Context, tenantID, customerID uuid.UUID, platform, externalID string, handle *string, socialAccountID *uuid.UUID) (customer.Identity, error)
		IdentitiesByCustomers(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]customer.Identity, error)
		LinkIdentity(ctx context.Context, tenantID, customerID, identityID uuid.UUID) (customer.Customer, error)
		Touch(ctx context.Context, tenantID, customerID uuid.UUID, at time.Time) error
	}
	Notifier interface {
		Notify(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, kind, title string, body, link *string) error
	}
	Members interface {
		Membership(ctx context.Context, userID, tenantID uuid.UUID) (tenant.Membership, error)
		// Get returns the tenant (settings: whatsapp_templates, business_hours; timezone).
		Get(ctx context.Context, tenantID uuid.UUID) (tenant.Tenant, error)
	}
	Queue interface {
		Enqueue(ctx context.Context, kind string, payload any, runAt time.Time, dedupeKey string) (int64, error)
	}
	// InlineRunner runs a just-committed job immediately (jobs.Worker.RunByID).
	InlineRunner func(ctx context.Context, jobID int64) (bool, error)
	// Sealer is the vault's transient encryption (SECURITY H3).
	Sealer interface {
		SealTransient(tenantID uuid.UUID, purpose string, plaintext []byte) (string, error)
		OpenTransient(tenantID uuid.UUID, purpose, sealed string) ([]byte, error)
	}
)

// Service is the inbox module API.
type Service interface {
	Sender
	Ingest(ctx context.Context, msg social.InboundMessage) (IngestResult, error)
	Send(ctx context.Context, tenantID, userID uuid.UUID, in SendInput) (Message, error)
	// SendSystem queues a message to a customer on a channel ("" = preferred) on behalf of the business.
	SendSystem(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) (Message, error)
	// SendSystemSensitive is SendSystem for a message that contains a secret: messages.body
	// and the conversation preview get storedBody (redacted); secretBody is sent to the
	// platform from a vault-sealed job payload and never persisted in plaintext.
	SendSystemSensitive(ctx context.Context, tenantID, customerID uuid.UUID, channel, storedBody, secretBody string) (Message, error)
	// Deliver is the inbox.send job: calls the platform and records the result.
	Deliver(ctx context.Context, p SendPayload, final bool) error

	Conversations(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) ([]Conversation, bool, int, error)
	Conversation(ctx context.Context, tenantID, id uuid.UUID) (Conversation, error)
	Messages(ctx context.Context, tenantID, conversationID uuid.UUID, before *uuid.UUID, limit int) ([]Message, error)
	Counts(ctx context.Context, tenantID, userID uuid.UUID) (Counts, error)
	OpenCountsByAccount(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error)
	Assign(ctx context.Context, tenantID, id uuid.UUID, userID *uuid.UUID) (Conversation, error)
	SetStatus(ctx context.Context, tenantID, id uuid.UUID, status string) (Conversation, error)
	MarkRead(ctx context.Context, tenantID, id uuid.UUID) (Conversation, error)
	AddLabel(ctx context.Context, tenantID, id uuid.UUID, label string) (Conversation, error)
	RemoveLabel(ctx context.Context, tenantID, id uuid.UUID, label string) (Conversation, error)
	LinkCustomer(ctx context.Context, tenantID, id, customerID uuid.UUID) (Conversation, error)

	Templates(ctx context.Context, tenantID uuid.UUID, category *string) ([]Template, error)
	Template(ctx context.Context, tenantID, id uuid.UUID) (Template, error)
	TemplateByKey(ctx context.Context, tenantID uuid.UUID, key string) (Template, error)
	TemplatesByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Template, error)
	UpsertTemplate(ctx context.Context, tenantID uuid.UUID, id *uuid.UUID, in TemplateInput) (Template, error)
	DeleteTemplate(ctx context.Context, tenantID, id uuid.UUID) error

	// Auto-reply rules (FAQ bot).
	AutoReplyRules(ctx context.Context, tenantID uuid.UUID) ([]AutoReplyRule, error)
	UpsertAutoReplyRule(ctx context.Context, tenantID uuid.UUID, id *uuid.UUID, in AutoReplyInput) (AutoReplyRule, error)
	DeleteAutoReplyRule(ctx context.Context, tenantID, id uuid.UUID) error
	TestAutoReply(ctx context.Context, tenantID uuid.UUID, text, platform string) (AutoReplyTest, error)
	// AutoReply is the inbox.autoreply job.
	AutoReply(ctx context.Context, p AutoReplyPayload) error
}

type service struct {
	pool      *pgxpool.Pool
	repo      Repository
	social    Social
	customers Customers
	notifier  Notifier
	members   Members
	queue     Queue
	inline    InlineRunner
	sealer    Sealer
	now       func() time.Time
}

// NewService builds the inbox. inline may be nil (jobs then run only in cmd/worker);
// sealer may be nil only when SendSystemSensitive is never used.
func NewService(pool *pgxpool.Pool, soc Social, customers Customers, notifier Notifier, members Members, queue Queue, inline InlineRunner, sealer Sealer) Service {
	return &service{pool: pool, social: soc, customers: customers, notifier: notifier, members: members, queue: queue, inline: inline,
		sealer: sealer, now: time.Now}
}

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func strp(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// ---- ingest

func (s *service) Ingest(ctx context.Context, msg social.InboundMessage) (IngestResult, error) {
	if err := msg.Validate(); err != nil {
		return IngestResult{}, err
	}
	var acct social.Account
	var err error
	if msg.AccountID != nil {
		acct, err = s.social.AccountByID(ctx, *msg.AccountID)
	} else {
		acct, err = s.social.ResolveInbound(ctx, msg.Platform, msg.AccountExternal)
	}
	if errors.Is(err, apperr.ErrNotFound) {
		return IngestResult{}, fmt.Errorf("%w: %s %s", social.ErrUnknownAccount, msg.Platform, msg.AccountExternal)
	}
	if err != nil {
		return IngestResult{}, err
	}
	if msg.SentAt.IsZero() {
		msg.SentAt = s.now()
	}
	// Two first messages from a brand-new sender can race on the identity unique key;
	// the loser retries once and then finds the identity.
	var res IngestResult
	for attempt := 0; attempt < 2; attempt++ {
		res, err = s.ingest(ctx, acct, msg)
		if !errors.Is(err, apperr.ErrConflict) {
			break
		}
	}
	return res, err
}

func (s *service) ingest(ctx context.Context, acct social.Account, msg social.InboundMessage) (res IngestResult, err error) {
	tid := acct.TenantID
	err = s.tx(ctx, tid, func(ctx context.Context, tx pgx.Tx) error {
		cust, ident, created, err := s.resolveCustomer(ctx, tid, acct, msg)
		if err != nil {
			return err
		}
		res.CustomerID, res.CustomerCreated = cust.ID, created
		conv, err := s.repo.UpsertConversation(ctx, tx, tid, acct.ID, msg.Platform, msg.ThreadExternal, &cust.ID, &ident.ID)
		if err != nil {
			return err
		}
		res.ConversationID = conv.ID
		kind := msg.Kind
		if kind == "" {
			kind = "text"
		}
		m, ok, err := s.repo.InsertMessage(ctx, tx, tid, Message{ConversationID: conv.ID, Direction: Inbound, ExternalID: &msg.MessageExternal,
			Kind: kind, Body: strp(msg.Body), Attachments: msg.Attachments, Status: MsgReceived, SentAt: msg.SentAt})
		if err != nil {
			return err
		}
		if !ok {
			res.Duplicate = true
			return nil
		}
		res.MessageID = m.ID
		preview := Preview(kind, m.Body)
		if err := s.repo.BumpInbound(ctx, tx, tid, conv.ID, msg.SentAt, preview); err != nil {
			return err
		}
		if conv.CustomerID != nil {
			if err := s.customers.Touch(ctx, tid, *conv.CustomerID, msg.SentAt); err != nil {
				return err
			}
		}
		if err := s.queueAutoReply(ctx, tx, tid, conv, m); err != nil {
			return err
		}
		name := cust.Name
		link := "/inbox/" + conv.ID.String()
		return s.notifier.Notify(ctx, tid, conv.AssignedTo, "inbox.message", "নতুন মেসেজ — "+name, strp(preview), &link)
	})
	return res, err
}

// resolveCustomer finds the identity's customer or creates both (customer source = platform).
func (s *service) resolveCustomer(ctx context.Context, tid uuid.UUID, acct social.Account, msg social.InboundMessage) (customer.Customer, customer.Identity, bool, error) {
	ident, err := s.customers.FindIdentity(ctx, tid, msg.Platform, msg.ThreadExternal)
	if err == nil {
		c, err := s.customers.Get(ctx, tid, ident.CustomerID)
		return c, ident, false, err
	}
	if !errors.Is(err, apperr.ErrNotFound) {
		return customer.Customer{}, ident, false, err
	}
	var cust customer.Customer
	created := false
	// WhatsApp ids are phone numbers: attach to an existing customer with that phone.
	waPhone := ""
	if msg.Platform == social.PlatformWhatsApp {
		if p, ok := phone.Normalize(firstNonEmpty(msg.SenderPhone, msg.ThreadExternal)); ok {
			waPhone = p
			if c, err := s.customers.FindByPhone(ctx, tid, p); err == nil {
				cust = c
			} else if !errors.Is(err, apperr.ErrNotFound) {
				return cust, ident, false, err
			}
		}
	}
	if cust.ID == uuid.Nil {
		name := strings.TrimSpace(msg.SenderName)
		if name == "" {
			name = DefaultName(msg.Platform, msg.ThreadExternal)
		}
		pc := msg.Platform
		in := customer.CreateInput{Name: name, Source: msg.Platform, PreferredChannel: &pc}
		if waPhone != "" {
			in.Phone, in.Whatsapp = &waPhone, &waPhone
		}
		if msg.Platform == social.PlatformTelegram && msg.SenderUsername != "" {
			in.Telegram = strp("@" + msg.SenderUsername)
		}
		if cust, err = s.customers.Create(ctx, tid, uuid.Nil, in); err != nil {
			return cust, ident, false, err
		}
		created = true
	}
	handle := strp(firstNonEmpty(msg.SenderName, msg.SenderUsername))
	ident, err = s.customers.AddIdentity(ctx, tid, cust.ID, msg.Platform, msg.ThreadExternal, handle, &acct.ID)
	return cust, ident, created, err
}

func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

// ---- outbound

func (s *service) Send(ctx context.Context, tenantID, userID uuid.UUID, in SendInput) (out Message, err error) {
	ownTx := !inTx(ctx)
	var jobID int64
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		conv, err := s.repo.GetConversation(ctx, tx, tenantID, in.ConversationID, true)
		if err != nil {
			return err
		}
		body := ""
		if in.Body != nil {
			body = strings.TrimSpace(*in.Body)
		}
		kind := "text"
		if in.TemplateID != nil {
			tpl, err := s.repo.Template(ctx, tx, tenantID, *in.TemplateID)
			if err != nil {
				return err
			}
			if body == "" {
				body = tpl.Body
			}
			kind = "template"
		}
		vars, err := s.defaultVars(ctx, tenantID, conv)
		if err != nil {
			return err
		}
		for k, v := range in.Variables {
			vars[strings.ToLower(k)] = v
		}
		body = strings.TrimSpace(Render(body, vars))
		if body == "" && len(in.Attachments) == 0 {
			return apperr.ValidationField("body", "message is empty")
		}
		if !conv.CanReply(s.now()) {
			if conv.Platform == social.PlatformWhatsApp {
				return apperr.Validation("the 24-hour WhatsApp window has closed: free-form replies are blocked until the customer writes again; " +
					"only approved templates (reminders, deliveries, campaigns) can be sent")
			}
			return apperr.Validation("the 24-hour reply window for this " + conv.Platform + " conversation has closed; wait for the customer to message again")
		}
		if kind == "text" && body == "" {
			kind = in.Attachments[0].Type
		}
		uid := &userID
		if userID == uuid.Nil {
			uid = nil
		}
		out, jobID, err = s.queueMessage(ctx, tx, tenantID, conv, kind, body, "", in.Attachments, uid, "", nil, true)
		return err
	})
	if err != nil || !ownTx {
		return out, err
	}
	return s.runInline(ctx, tenantID, out, jobID), nil
}

func inTx(ctx context.Context) bool {
	_, ok := db.TxFromContext(ctx)
	return ok
}

// runInline sends a just-committed message immediately; the worker retries on failure.
func (s *service) runInline(ctx context.Context, tenantID uuid.UUID, m Message, jobID int64) Message {
	if s.inline == nil || jobID == 0 {
		return m
	}
	if _, err := s.inline(ctx, jobID); err != nil {
		return m
	}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.GetMessage(ctx, tx, tenantID, m.ID, false)
		if err == nil {
			m = cur
		}
		return err
	})
	_ = err
	return m
}

// queueMessage stores the outbound message (body) and queues inbox.send. A non-empty
// secretBody is what is actually sent: it is sealed into the job payload, bound to the
// new message id, and never written to messages/conversations.
func (s *service) queueMessage(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, conv Conversation, kind, body, secretBody string, att []social.Attachment, by *uuid.UUID, tag string, wa *WATemplateSpec, markRead bool) (Message, int64, error) {
	now := s.now()
	m, _, err := s.repo.InsertMessage(ctx, tx, tenantID, Message{ConversationID: conv.ID, Direction: Outbound, Kind: kind, Body: strp(body),
		Attachments: att, SentBy: by, Status: MsgQueued, SentAt: now})
	if err != nil {
		return m, 0, err
	}
	if err := s.repo.BumpOutbound(ctx, tx, tenantID, conv.ID, now, Preview(kind, m.Body), markRead); err != nil {
		return m, 0, err
	}
	p := SendPayload{TenantID: tenantID, MessageID: m.ID, Tag: tag, WATemplate: wa}
	if secretBody != "" {
		if s.sealer == nil {
			return m, 0, errors.New("inbox: no sealer configured for sensitive messages")
		}
		if p.SealedBody, err = s.sealer.SealTransient(tenantID, m.ID.String(), []byte(secretBody)); err != nil {
			return m, 0, err
		}
	}
	id, err := s.queue.Enqueue(ctx, JobSend, p, now, JobSend+":"+m.ID.String())
	return m, id, err
}

func (s *service) defaultVars(ctx context.Context, tenantID uuid.UUID, conv Conversation) (map[string]string, error) {
	vars := map[string]string{}
	if conv.CustomerID != nil {
		c, err := s.customers.Get(ctx, tenantID, *conv.CustomerID)
		if err != nil && !errors.Is(err, apperr.ErrNotFound) {
			return nil, err
		}
		vars["customer"] = c.Name
	}
	return vars, nil
}

func (s *service) SendToCustomer(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) error {
	_, err := s.SendSystem(ctx, tenantID, customerID, channel, body)
	return err
}

func (s *service) SendSystem(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) (Message, error) {
	return s.sendSystem(ctx, tenantID, customerID, channel, body, "")
}

func (s *service) SendSystemSensitive(ctx context.Context, tenantID, customerID uuid.UUID, channel, storedBody, secretBody string) (Message, error) {
	secretBody = strings.TrimSpace(secretBody)
	if secretBody == "" {
		return Message{}, apperr.ValidationField("body", "message is empty")
	}
	return s.sendSystem(ctx, tenantID, customerID, channel, storedBody, secretBody)
}

func (s *service) sendSystem(ctx context.Context, tenantID, customerID uuid.UUID, channel, body, secretBody string) (out Message, err error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return out, apperr.ValidationField("body", "message is empty")
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "preferred" {
		channel = ""
	}
	if channel != "" && !social.ValidPlatform(channel) {
		return out, apperr.Validation("channel " + channel + " is not supported for messages yet")
	}
	ownTx := !inTx(ctx)
	var jobID int64
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cust, err := s.customers.Get(ctx, tenantID, customerID)
		if err != nil {
			return err
		}
		conv, err := s.conversationFor(ctx, tx, tenantID, cust, channel)
		if err != nil {
			return err
		}
		tag, kind := "", "text"
		var wa *WATemplateSpec
		if !conv.CanReply(s.now()) {
			switch {
			case social.MessageTagAllowed(conv.Platform):
				tag = OutsideWindowTag
			case conv.Platform == social.PlatformWhatsApp:
				spec, err := s.whatsAppTemplate(ctx, tenantID, PurposeFrom(ctx))
				if err != nil {
					return err
				}
				spec.Customer, wa, kind = cust.Name, &spec, "template"
			default:
				return apperr.Validation("the 24-hour " + conv.Platform + " window is closed for " + cust.Name + "; an approved template is required")
			}
		}
		out, jobID, err = s.queueMessage(ctx, tx, tenantID, conv, kind, body, secretBody, nil, nil, tag, wa, false)
		return err
	})
	if err != nil || !ownTx {
		return out, err
	}
	return s.runInline(ctx, tenantID, out, jobID), nil
}

// conversationFor picks the thread to message a customer on: channel ("" = preferred,
// then most recent). Falls back to opening a conversation from a channel identity.
func (s *service) conversationFor(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, cust customer.Customer, channel string) (Conversation, error) {
	tryChannels := []string{channel}
	if channel == "" {
		tryChannels = nil
		if cust.PreferredChannel != nil && social.ValidPlatform(*cust.PreferredChannel) {
			tryChannels = append(tryChannels, *cust.PreferredChannel)
		}
		tryChannels = append(tryChannels, "")
	}
	for _, ch := range tryChannels {
		conv, err := s.repo.LatestForCustomer(ctx, tx, tenantID, cust.ID, ch)
		if err == nil {
			return conv, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return conv, err
		}
	}
	ids, err := s.customers.IdentitiesByCustomers(ctx, tenantID, []uuid.UUID{cust.ID})
	if err != nil {
		return Conversation{}, err
	}
	for _, ch := range tryChannels {
		for _, id := range ids[cust.ID] {
			if id.SocialAccountID == nil || (ch != "" && id.Platform != ch) || !social.ValidPlatform(id.Platform) {
				continue
			}
			idID := id.ID
			return s.repo.UpsertConversation(ctx, tx, tenantID, *id.SocialAccountID, id.Platform, id.ExternalID, &cust.ID, &idID)
		}
	}
	what := "any connected channel"
	if channel != "" {
		what = channel
	}
	return Conversation{}, apperr.Validation(cust.Name + " cannot be reached on " + what + " yet (no conversation)")
}

// permanent reports errors that retrying will not fix.
func permanent(err error) bool {
	var ae *social.APIError
	if errors.As(err, &ae) && ae.Status >= 400 && ae.Status < 500 && ae.Status != http.StatusTooManyRequests {
		return true
	}
	return errors.Is(err, apperr.ErrValidation) || errors.Is(err, social.ErrUnsupported) || social.IsAuthError(err)
}

func (s *service) Deliver(ctx context.Context, p SendPayload, final bool) error {
	var m Message
	var conv Conversation
	err := s.tx(ctx, p.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if m, err = s.repo.GetMessage(ctx, tx, p.TenantID, p.MessageID, false); err != nil {
			return err
		}
		conv, err = s.repo.GetConversation(ctx, tx, p.TenantID, m.ConversationID, false)
		return err
	})
	if errors.Is(err, apperr.ErrNotFound) {
		return nil // deleted meanwhile
	}
	if err != nil {
		return err
	}
	if m.Status != MsgQueued {
		return nil // already delivered (inline run or earlier attempt)
	}
	acct, err := s.social.Get(ctx, p.TenantID, conv.SocialAccountID)
	var extID string
	if err == nil {
		sctx := ctx
		if p.Tag != "" {
			sctx = social.WithMessageTag(ctx, p.Tag)
		}
		body := ""
		if m.Body != nil {
			body = *m.Body
		}
		if p.SealedBody != "" { // the stored body is redacted; send the sealed original
			if s.sealer == nil {
				err = errors.New("inbox: no sealer configured")
			} else if pt, oerr := s.sealer.OpenTransient(p.TenantID, m.ID.String(), p.SealedBody); oerr != nil {
				err = fmt.Errorf("%w: sealed message body cannot be opened", apperr.ErrValidation)
			} else {
				body = string(pt)
			}
		}
		if err == nil {
			if p.WATemplate != nil {
				sctx = social.WithWhatsAppTemplate(sctx, p.WATemplate.Resolve(body))
			}
			extID, err = s.social.Send(sctx, acct, conv.ExternalThreadID, body, m.Attachments)
		}
	}
	status, errMsg, ext := MsgSent, (*string)(nil), strp(extID)
	if err != nil {
		if !final && !permanent(err) {
			msg := "retrying: " + social.SafeError(err)
			_ = s.tx(ctx, p.TenantID, func(ctx context.Context, tx pgx.Tx) error {
				_, e := s.repo.SetMessageResult(ctx, tx, p.TenantID, m.ID, MsgQueued, nil, &msg)
				return e
			})
			return err
		}
		status, ext = MsgFailed, nil
		e := social.SafeError(err)
		errMsg = &e
	}
	return s.tx(ctx, p.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.repo.SetMessageResult(ctx, tx, p.TenantID, m.ID, status, ext, errMsg)
		return err
	})
}

// ---- conversations

func (s *service) Conversations(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) (items []Conversation, more bool, total int, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.repo.List(ctx, tx, tenantID, f, page)
		if err != nil {
			return err
		}
		items, more = pagination.Trim(rows, page.First)
		total, err = s.repo.Count(ctx, tx, tenantID, f)
		return err
	})
	return items, more, total, err
}

func (s *service) Conversation(ctx context.Context, tenantID, id uuid.UUID) (c Conversation, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		c, err = s.repo.GetConversation(ctx, tx, tenantID, id, false)
		return err
	})
	return c, err
}

func (s *service) Messages(ctx context.Context, tenantID, conversationID uuid.UUID, before *uuid.UUID, limit int) (out []Message, err error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.Messages(ctx, tx, tenantID, conversationID, before, limit)
		return err
	})
	return out, err
}

func (s *service) Counts(ctx context.Context, tenantID, userID uuid.UUID) (c Counts, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		c, err = s.repo.Counts(ctx, tx, tenantID, userID)
		return err
	})
	return c, err
}

func (s *service) OpenCountsByAccount(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (out map[uuid.UUID]int, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.OpenCountsByAccount(ctx, tx, tenantID, ids)
		return err
	})
	return out, err
}

func (s *service) update(ctx context.Context, tenantID, id uuid.UUID, set string, args ...any) (c Conversation, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		c, err = s.repo.Update(ctx, tx, tenantID, id, set, args...)
		return err
	})
	return c, err
}

func (s *service) Assign(ctx context.Context, tenantID, id uuid.UUID, userID *uuid.UUID) (Conversation, error) {
	if userID != nil {
		m, err := s.members.Membership(ctx, *userID, tenantID)
		if err != nil || m.Status != "active" {
			return Conversation{}, apperr.ValidationField("userId", "not an active member of this business")
		}
	}
	return s.update(ctx, tenantID, id, `assigned_to = $3`, userID)
}

func (s *service) SetStatus(ctx context.Context, tenantID, id uuid.UUID, status string) (Conversation, error) {
	status = strings.ToLower(status)
	if !ValidStatus(status) {
		return Conversation{}, apperr.ValidationField("status", "invalid status")
	}
	return s.update(ctx, tenantID, id, `status = $3`, status)
}

func (s *service) MarkRead(ctx context.Context, tenantID, id uuid.UUID) (Conversation, error) {
	return s.update(ctx, tenantID, id, `unread_count = 0`)
}

func (s *service) AddLabel(ctx context.Context, tenantID, id uuid.UUID, label string) (Conversation, error) {
	l, err := NormalizeLabel(label)
	if err != nil {
		return Conversation{}, err
	}
	return s.update(ctx, tenantID, id, `labels = CASE WHEN $3 = ANY(c.labels) THEN c.labels ELSE array_append(c.labels, $3) END`, l)
}

func (s *service) RemoveLabel(ctx context.Context, tenantID, id uuid.UUID, label string) (Conversation, error) {
	l, err := NormalizeLabel(label)
	if err != nil {
		return Conversation{}, err
	}
	return s.update(ctx, tenantID, id, `labels = array_remove(c.labels, $3)`, l)
}

// LinkCustomer attaches the thread (and its channel identity) to an existing customer.
func (s *service) LinkCustomer(ctx context.Context, tenantID, id, customerID uuid.UUID) (c Conversation, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		conv, err := s.repo.GetConversation(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if _, err := s.customers.Get(ctx, tenantID, customerID); err != nil {
			return err
		}
		if conv.IdentityID != nil {
			// Moves the identity and every conversation on it.
			if _, err := s.customers.LinkIdentity(ctx, tenantID, customerID, *conv.IdentityID); err != nil {
				return err
			}
		}
		c, err = s.repo.Update(ctx, tx, tenantID, id, `customer_id = $3`, customerID)
		return err
	})
	return c, err
}

// ---- templates

func (s *service) Templates(ctx context.Context, tenantID uuid.UUID, category *string) (out []Template, err error) {
	if category != nil {
		c := strings.ToLower(*category)
		category = &c
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.Templates(ctx, tx, tenantID, category)
		return err
	})
	return out, err
}

func (s *service) Template(ctx context.Context, tenantID, id uuid.UUID) (t Template, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		t, err = s.repo.Template(ctx, tx, tenantID, id)
		return err
	})
	return t, err
}

func (s *service) TemplateByKey(ctx context.Context, tenantID uuid.UUID, key string) (t Template, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		t, err = s.repo.TemplateByKey(ctx, tx, tenantID, key)
		return err
	})
	return t, err
}

func (s *service) TemplatesByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Template, error) {
	out := map[uuid.UUID]Template{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ts, err := s.repo.TemplatesByIDs(ctx, tx, tenantID, ids)
		for _, t := range ts {
			out[t.ID] = t
		}
		return err
	})
	return out, err
}

func (s *service) UpsertTemplate(ctx context.Context, tenantID uuid.UUID, id *uuid.UUID, in TemplateInput) (t Template, err error) {
	if err := in.Validate(); err != nil {
		return t, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if id == nil {
			t, err = s.repo.InsertTemplate(ctx, tx, tenantID, in)
		} else {
			t, err = s.repo.UpdateTemplate(ctx, tx, tenantID, *id, in)
		}
		if errors.Is(err, apperr.ErrConflict) {
			return apperr.ValidationField("key", "a template with this key already exists")
		}
		return err
	})
	return t, err
}

func (s *service) DeleteTemplate(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.DeleteTemplate(ctx, tx, tenantID, id)
		if err == nil && !ok {
			return apperr.NotFound("template")
		}
		return err
	})
}
