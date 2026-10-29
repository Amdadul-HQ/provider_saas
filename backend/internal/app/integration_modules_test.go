//go:build integration

package app_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/app"
	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/delivery"
	"provider_saas/internal/modules/inbox"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/reminder"
	"provider_saas/internal/modules/social"
	"provider_saas/internal/modules/storefront"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/ticket"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/jobs"
	"provider_saas/internal/platform/pagination"
)

func uniq(prefix string) string { return prefix + strings.ReplaceAll(uuid.NewString()[:8], "-", "") }

// telegramBot connects a bot through the Fake client (META_APP_ID is empty in tests).
func telegramBot(t *testing.T, f tenantFx) social.Account {
	t.Helper()
	id := 800000000 + time.Now().UnixNano()%100000000
	a, err := testC.Social.ConnectTelegram(f.ctx, f.tid, f.uid, fmt.Sprintf("%d:AAItTestToken_%s", id, uniq("x")))
	if err != nil {
		t.Fatal(err)
	}
	if !a.WebhookVerified || a.TokenSecretID == nil {
		t.Fatalf("telegram account = %+v", a)
	}
	return a
}

func whatsAppNumber(t *testing.T, f tenantFx) social.Account {
	t.Helper()
	a, err := testC.Social.ConnectWhatsApp(f.ctx, f.tid, f.uid, social.ConnectWhatsAppInput{PhoneNumberID: uniq("pn"),
		BusinessAccountID: "waba-it", AccessToken: "EAAG-it-whatsapp-access-token-123456", DisplayName: "IT WhatsApp"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func count(t *testing.T, tid uuid.UUID, sql string, args ...any) int {
	t.Helper()
	var n int
	err := db.WithTenantTx(context.Background(), testC.Pool, tid, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, sql, append([]any{tid}, args...)...).Scan(&n)
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIngestCreatesCustomerAndConversationIdempotently(t *testing.T) {
	f := newTenant(t, "Ingest Shop")
	bot := telegramBot(t, f)
	msg := social.InboundMessage{Platform: social.PlatformTelegram, AccountExternal: bot.ExternalID, ThreadExternal: "424242",
		MessageExternal: "1001", SenderName: "Tanvir Ahmed", SenderUsername: "tanvir_bd", Kind: "text", Body: "Netflix price?", SentAt: time.Now()}
	r1, err := testC.Inbox.Ingest(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if !r1.CustomerCreated || r1.Duplicate || r1.ConversationID == uuid.Nil {
		t.Fatalf("first ingest = %+v", r1)
	}
	// Redelivery of the same webhook: nothing new.
	r2, err := testC.Inbox.Ingest(context.Background(), msg)
	if err != nil || !r2.Duplicate || r2.ConversationID != r1.ConversationID || r2.CustomerID != r1.CustomerID {
		t.Fatalf("redelivery = %+v %v", r2, err)
	}
	// A second message from the same sender joins the same customer + conversation.
	msg.MessageExternal, msg.Body = "1002", "Spotify?"
	r3, err := testC.Inbox.Ingest(context.Background(), msg)
	if err != nil || r3.CustomerCreated || r3.ConversationID != r1.ConversationID {
		t.Fatalf("second message = %+v %v", r3, err)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM customers WHERE tenant_id = $1`); n != 1 {
		t.Fatalf("customers = %d", n)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM messages WHERE tenant_id = $1`); n != 2 {
		t.Fatalf("messages = %d", n)
	}
	c, err := testC.Customers.Get(f.ctx, f.tid, r1.CustomerID)
	if err != nil || c.Source != "telegram" || c.PreferredChannel == nil || *c.PreferredChannel != "telegram" || c.Telegram == nil || *c.Telegram != "@tanvir_bd" {
		t.Fatalf("customer = %+v %v", c, err)
	}
	conv, err := testC.Inbox.Conversation(f.ctx, f.tid, r1.ConversationID)
	if err != nil || conv.UnreadCount != 2 || conv.LastMessagePreview == nil || *conv.LastMessagePreview != "Spotify?" || conv.CustomerID == nil {
		t.Fatalf("conversation = %+v %v", conv, err)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM notifications WHERE tenant_id = $1 AND kind = 'inbox.message'`); n != 2 {
		t.Fatalf("notifications = %d", n)
	}
	// customerId filter (frontend customer page)
	items, _, total, err := testC.Inbox.Conversations(f.ctx, f.tid, inbox.Filter{CustomerID: &r1.CustomerID}, pagination.Page{First: 10})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("filter by customer: %d %v", total, err)
	}
	// WhatsApp sender with a known phone is attached to the existing customer.
	wa := whatsAppNumber(t, f)
	known := mustCustomer(t, f, "Karim") // phone 01712345678
	rw, err := testC.Inbox.Ingest(context.Background(), social.InboundMessage{Platform: social.PlatformWhatsApp, AccountExternal: wa.ExternalID,
		ThreadExternal: "8801712345678", SenderPhone: "8801712345678", MessageExternal: "wamid.it1", SenderName: "Karim WA", Body: "hi"})
	if err != nil || rw.CustomerCreated || rw.CustomerID != known.ID {
		t.Fatalf("whatsapp ingest = %+v %v", rw, err)
	}
	// Unknown page -> ErrUnknownAccount (webhook answers 200 and skips it).
	_, err = testC.Inbox.Ingest(context.Background(), social.InboundMessage{Platform: social.PlatformFacebook, AccountExternal: "no-such-page",
		ThreadExternal: "1", MessageExternal: "m1"})
	if !errors.Is(err, social.ErrUnknownAccount) {
		t.Fatalf("unknown account: %v", err)
	}
}

func TestCanReplyWindowAndSend(t *testing.T) {
	f := newTenant(t, "Window Shop")
	wa := whatsAppNumber(t, f)
	old, err := testC.Inbox.Ingest(context.Background(), social.InboundMessage{Platform: social.PlatformWhatsApp, AccountExternal: wa.ExternalID,
		ThreadExternal: "8801900000001", MessageExternal: "wamid.old", Body: "old", SentAt: time.Now().Add(-25 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	conv, _ := testC.Inbox.Conversation(f.ctx, f.tid, old.ConversationID)
	if conv.CanReply(time.Now()) {
		t.Fatal("25h-old WhatsApp thread must not be repliable")
	}
	body := "hello"
	if _, err := testC.Inbox.Send(f.ctx, f.tid, f.uid, inbox.SendInput{ConversationID: conv.ID, Body: &body}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("send outside window: %v", err)
	}
	// System messages (reminders) on WhatsApp outside the window need a template -> validation error too.
	if _, err := testC.Inbox.SendSystem(f.ctx, f.tid, *conv.CustomerID, "whatsapp", "renew"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("system send outside window: %v", err)
	}
	// Customer writes again: window opens; the message is sent inline through the fake client.
	if _, err := testC.Inbox.Ingest(context.Background(), social.InboundMessage{Platform: social.PlatformWhatsApp, AccountExternal: wa.ExternalID,
		ThreadExternal: "8801900000001", MessageExternal: "wamid.new", Body: "price?", SentAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	conv, _ = testC.Inbox.Conversation(f.ctx, f.tid, conv.ID)
	if !conv.CanReply(time.Now()) {
		t.Fatal("fresh inbound must open the window")
	}
	tpl, err := testC.Inbox.UpsertTemplate(f.ctx, f.tid, nil, inbox.TemplateInput{Key: "it.hello", Name: "hi", Body: "Hi {{customer}}, {{product}} {{amount}}", Category: "quick_reply", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	m, err := testC.Inbox.Send(f.ctx, f.tid, f.uid, inbox.SendInput{ConversationID: conv.ID, TemplateID: &tpl.ID,
		Variables: map[string]string{"product": "Netflix", "amount": "৳350.00"}})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != inbox.MsgSent || m.ExternalID == nil || m.Body == nil || !strings.HasPrefix(*m.Body, "Hi WhatsApp user 0001, Netflix ৳350.00") {
		t.Fatalf("sent message = %+v body=%v", m, deref(m.Body))
	}
	conv, _ = testC.Inbox.Conversation(f.ctx, f.tid, conv.ID)
	if conv.UnreadCount != 0 {
		t.Fatalf("reply must mark read, unread=%d", conv.UnreadCount)
	}
	// Telegram can always reply.
	bot := telegramBot(t, f)
	tg, _ := testC.Inbox.Ingest(context.Background(), social.InboundMessage{Platform: social.PlatformTelegram, AccountExternal: bot.ExternalID,
		ThreadExternal: "77", MessageExternal: "1", Body: "x", SentAt: time.Now().Add(-72 * time.Hour)})
	m2, err := testC.Inbox.Send(f.ctx, f.tid, f.uid, inbox.SendInput{ConversationID: tg.ConversationID, Body: &body})
	if err != nil || m2.Status != inbox.MsgSent {
		t.Fatalf("telegram send: %+v %v", m2, err)
	}
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestResolveReplacementMovesSlotAndBansOld(t *testing.T) {
	f := newTenant(t, "Replacement Shop")
	p, plan, m := sharedProduct(t, f, 3)
	tpl := "{{customer}}: {{credentials}}"
	if _, err := testC.Catalog.Update(f.ctx, f.tid, p.ID, catalog.UpdateInput{DeliveryTemplate: &tpl}); err != nil {
		t.Fatal(err)
	}
	c := mustCustomer(t, f, "Rahim")
	sub, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID})
	if err != nil || sub.SlotID == nil {
		t.Fatalf("create: %+v %v", sub, err)
	}
	oldSlot := *sub.SlotID
	if _, err := testC.Subscriptions.MarkDelivered(f.ctx, f.tid, sub.ID, nil); err != nil {
		t.Fatal(err)
	}
	reason := "banned"
	tk, err := testC.Tickets.Create(f.ctx, f.tid, f.uid, ticket.CreateInput{CustomerID: c.ID, SubscriptionID: &sub.ID, Kind: "replacement",
		Title: "account banned", Reason: &reason})
	if err != nil || tk.OldSlotID == nil || *tk.OldSlotID != oldSlot || tk.Number <= 0 {
		t.Fatalf("ticket = %+v %v", tk, err)
	}
	res, err := testC.Tickets.ResolveReplacement(f.ctx, f.tid, f.uid, tk.ID, nil, "banned", 3)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != ticket.StatusResolved || res.ResolvedAt == nil || res.NewSlotID == nil || *res.NewSlotID == oldSlot {
		t.Fatalf("resolved ticket = %+v", res)
	}
	after, _ := testC.Subscriptions.Get(f.ctx, f.tid, sub.ID)
	if after.SlotID == nil || *after.SlotID != *res.NewSlotID || after.DeliveredAt != nil ||
		!after.ExpiresAt.Equal(sub.ExpiresAt.AddDate(0, 0, 3)) {
		t.Fatalf("subscription after = %+v", after)
	}
	slots, _ := testC.Inventory.SlotsByMasters(f.ctx, f.tid, []uuid.UUID{m.ID})
	for _, s := range slots[m.ID] {
		switch s.ID {
		case oldSlot:
			if s.Status != inventory.SlotBanned || s.SubscriptionID != nil {
				t.Fatalf("old slot = %+v", s)
			}
		case *res.NewSlotID:
			if s.Status != inventory.SlotAssigned || s.SubscriptionID == nil || *s.SubscriptionID != sub.ID {
				t.Fatalf("new slot = %+v", s)
			}
		}
	}
	if n := count(t, f.tid, `SELECT count(*) FROM jobs WHERE tenant_id = $1 AND kind = 'delivery.send' AND dedupe_key = $2`, "delivery.send:"+sub.ID.String()); n != 1 {
		t.Fatalf("delivery job not queued (%d)", n)
	}
	cs, _ := testC.Tickets.CommentsByTickets(f.ctx, f.tid, []uuid.UUID{tk.ID})
	if len(cs[tk.ID]) != 1 || !strings.Contains(cs[tk.ID][0].Body, "banned") {
		t.Fatalf("system comment = %+v", cs[tk.ID])
	}
	if _, err := testC.Tickets.ResolveReplacement(f.ctx, f.tid, f.uid, tk.ID, nil, "banned", 0); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("double resolve: %v", err)
	}
	st, err := testC.Tickets.Stats(f.ctx, f.tid)
	if err != nil || st.ResolvedThisMonth != 1 || st.ReplacementsThisMonth != 1 {
		t.Fatalf("stats = %+v %v", st, err)
	}
}

func publishedStore(t *testing.T, f tenantFx) storefront.Storefront {
	t.Helper()
	pub := true
	slug := uniq("it-")
	sf, err := testC.Storefront.Upsert(f.ctx, f.tid, storefront.Input{Slug: &slug, IsPublished: &pub,
		PaymentNumbers: []any{map[string]any{"method": "bkash", "number": "01700000000"}}})
	if err != nil {
		t.Fatal(err)
	}
	return sf
}

func TestPlaceOrderThenConfirmCreatesSubscriptionOnFreeSlot(t *testing.T) {
	f := newTenant(t, "Order Shop")
	p, plan, m := sharedProduct(t, f, 2)
	sf := publishedStore(t, f)

	pub, err := testC.Storefront.Public(context.Background(), sf.Slug)
	if err != nil || len(pub.Products) != 1 || !pub.Products[0].InStock || pub.Products[0].Plans[0].Price != plan.Price {
		t.Fatalf("public storefront = %+v %v", pub, err)
	}
	trx := "ITORD" + strings.ToUpper(uniq(""))
	bkash := "bkash"
	po, err := testC.Storefront.PlaceOrder(context.Background(), uniq("ip"), storefront.PlaceOrderInput{Slug: sf.Slug, ProductID: p.ID, PlanID: plan.ID,
		CustomerName: "Online Buyer", CustomerPhone: "+880 1811-223344", PaymentMethod: &bkash, TrxID: &trx})
	if err != nil {
		t.Fatal(err)
	}
	if po.Status != storefront.OrderAwaitingPayment || po.Amount != plan.Price || po.ProductName != "Netflix" {
		t.Fatalf("public order = %+v", po)
	}
	if got, err := testC.Storefront.PublicOrder(context.Background(), sf.Slug, po.Number, "01811223344"); err != nil || got.Number != po.Number {
		t.Fatalf("public order lookup: %+v %v", got, err)
	}
	if _, err := testC.Storefront.PublicOrder(context.Background(), sf.Slug, po.Number, "01999999999"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("lookup with wrong phone: %v", err)
	}
	orders, err := testC.Storefront.Orders(f.ctx, f.tid, storefront.OrderFilter{}, nil, 10)
	if err != nil || len(orders) != 1 || orders[0].PaymentID == nil || orders[0].CustomerID == nil {
		t.Fatalf("orders = %+v %v", orders, err)
	}
	o := orders[0]
	pay, _ := testC.Billing.Payment(f.ctx, f.tid, *o.PaymentID)
	if pay.Status != billing.PaymentPending || pay.InvoiceID != nil {
		t.Fatalf("order payment = %+v", pay)
	}

	conf, err := testC.Storefront.ConfirmOrder(f.ctx, f.tid, f.uid, o.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if conf.Status != storefront.OrderDelivered || conf.SubscriptionID == nil {
		t.Fatalf("confirmed = %+v", conf)
	}
	sub, _ := testC.Subscriptions.Get(f.ctx, f.tid, *conf.SubscriptionID)
	if sub.SlotID == nil || sub.DeliveredAt == nil || sub.CustomerID != *o.CustomerID {
		t.Fatalf("subscription = %+v", sub)
	}
	slots, _ := testC.Inventory.SlotsByMasters(f.ctx, f.tid, []uuid.UUID{m.ID})
	assigned := 0
	for _, s := range slots[m.ID] {
		if s.SubscriptionID != nil && *s.SubscriptionID == sub.ID && s.Status == inventory.SlotAssigned {
			assigned++
		}
	}
	if assigned != 1 {
		t.Fatal("slot not assigned to the order's subscription")
	}
	pay, _ = testC.Billing.Payment(f.ctx, f.tid, *o.PaymentID)
	if pay.Status != billing.PaymentVerified || pay.InvoiceID == nil {
		t.Fatalf("payment after confirm = %+v", pay)
	}
	inv, _ := testC.Billing.Invoice(f.ctx, f.tid, *pay.InvoiceID)
	if inv.Status != billing.InvoicePaid || inv.PaidAmount != plan.Price {
		t.Fatalf("invoice = %+v", inv)
	}
	cust, _ := testC.Customers.Get(f.ctx, f.tid, *o.CustomerID)
	if cust.DueBalance != 0 {
		t.Fatalf("due after paid order = %d", cust.DueBalance)
	}
	if _, err := testC.Storefront.ConfirmOrder(f.ctx, f.tid, f.uid, o.ID, nil, nil); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("double confirm: %v", err)
	}
	// A second order without TrxID can be cancelled; a reused TrxID is refused.
	if _, err := testC.Storefront.PlaceOrder(context.Background(), uniq("ip"), storefront.PlaceOrderInput{Slug: sf.Slug, ProductID: p.ID,
		PlanID: plan.ID, CustomerName: "Second", CustomerPhone: "01811223355", PaymentMethod: &bkash, TrxID: &trx}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("reused trx: %v", err)
	}
	// Unpublished / unknown storefront is not found.
	if _, err := testC.Storefront.Public(context.Background(), "no-such-store-xyz"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("unknown store: %v", err)
	}
}

func TestPortalTokenValidThenExpired(t *testing.T) {
	f := newTenant(t, "Portal Shop")
	publishedStore(t, f)
	p, plan, _ := sharedProduct(t, f, 2)
	c := mustCustomer(t, f, "Portal Customer")
	sub, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID, CreateInvoice: true})
	if err != nil {
		t.Fatal(err)
	}
	url, err := testC.Storefront.CreatePortalLink(f.ctx, f.tid, c.ID, "")
	if err != nil || !strings.Contains(url, "/p/") {
		t.Fatalf("link = %q %v", url, err)
	}
	token := url[strings.LastIndex(url, "/")+1:]
	v, err := testC.Storefront.Portal(context.Background(), token)
	if err != nil || v.CustomerName != "Portal Customer" || len(v.Subscriptions) != 1 || len(v.Invoices) != 1 || v.DueBalance != plan.Price {
		t.Fatalf("portal = %+v %v", v, err)
	}
	if _, err := testC.Storefront.Portal(context.Background(), token); err != nil {
		t.Fatalf("token must stay usable until expiry: %v", err)
	}
	ro, err := testC.Storefront.PortalRequestRenewal(context.Background(), token, sub.ID, nil, nil)
	if err != nil || ro.Status != storefront.OrderAwaitingPayment || ro.Amount != plan.Price {
		t.Fatalf("renewal order = %+v %v", ro, err)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM orders WHERE tenant_id = $1 AND source = 'portal'`); n != 1 {
		t.Fatalf("portal orders = %d", n)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM portal_tokens WHERE tenant_id = $1 AND token_hash = $2`, storefront.HashToken(token)); n != 1 {
		t.Fatal("only the sha256 of the token is stored")
	}
	// Expire it.
	if err := db.WithTenantTx(context.Background(), testC.Pool, f.tid, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE portal_tokens SET expires_at = now() - interval '1 minute' WHERE tenant_id = $1`, f.tid)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Storefront.Portal(context.Background(), token); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expired token: %v", err)
	}
	if _, err := testC.Storefront.Portal(context.Background(), "garbage-token-garbage-token-garbage"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("garbage token: %v", err)
	}
}

func TestDeliveryJobRendersCredentialsAndMasksNote(t *testing.T) {
	f := newTenant(t, "Delivery Shop")
	p, plan, _ := sharedProduct(t, f, 2)
	tpl := "প্রিয় {{customer}}, {{product}} ✅\n{{credentials}}\nমেয়াদ: {{expiry}}"
	if _, err := testC.Catalog.Update(f.ctx, f.tid, p.ID, catalog.UpdateInput{DeliveryTemplate: &tpl}); err != nil {
		t.Fatal(err)
	}
	bot := telegramBot(t, f)
	in, err := testC.Inbox.Ingest(context.Background(), social.InboundMessage{Platform: social.PlatformTelegram, AccountExternal: bot.ExternalID,
		ThreadExternal: "991", MessageExternal: "1", SenderName: "Buyer", Body: "Netflix chai"})
	if err != nil {
		t.Fatal(err)
	}
	trx := "ITDLV" + strings.ToUpper(uniq(""))
	sub, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: in.CustomerID, ProductID: p.ID, PlanID: &plan.ID,
		CreateInvoice: true, MarkPaid: &billing.PaymentInput{Method: "bkash", Amount: plan.Price, TrxID: &trx}})
	if err != nil {
		t.Fatal(err)
	}
	// Paying the invoice queued delivery.send (billing PaidHook).
	var jobID int64
	err = db.WithTenantTx(context.Background(), testC.Pool, f.tid, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id FROM jobs WHERE tenant_id = $1 AND dedupe_key = $2 AND done_at IS NULL`, f.tid, delivery.JobSend+":"+sub.ID.String()).Scan(&jobID)
	})
	if err != nil {
		t.Fatalf("delivery job not queued: %v", err)
	}
	w := jobs.NewWorker(testC.Pool, testC.Log)
	testC.RegisterJobs(w)
	if ran, err := w.RunByID(context.Background(), jobID); err != nil || !ran {
		t.Fatalf("run delivery job: %v %v", ran, err)
	}
	after, _ := testC.Subscriptions.Get(f.ctx, f.tid, sub.ID)
	if after.DeliveredAt == nil || after.DeliveryNote == nil || strings.Contains(*after.DeliveryNote, "s3cret-pass") {
		t.Fatalf("delivered = %+v note=%v", after, deref(after.DeliveryNote))
	}
	msgs, _ := testC.Inbox.Messages(f.ctx, f.tid, in.ConversationID, nil, 10)
	last := msgs[len(msgs)-1]
	if last.Direction != inbox.Outbound || last.Body == nil || !strings.Contains(*last.Body, "Password: s3cret-pass") || last.Status != inbox.MsgSent {
		t.Fatalf("delivery message = %+v body=%s", last, deref(last.Body))
	}
	if n := count(t, f.tid, `SELECT count(*) FROM audit_logs WHERE tenant_id = $1 AND action = 'delivery.render' AND entity_id = $2`, sub.ID); n != 1 {
		t.Fatalf("delivery.render audit rows = %d", n)
	}
	// Idempotent: running delivery again does nothing.
	if err := testC.Delivery.Deliver(context.Background(), delivery.Payload{TenantID: f.tid, SubscriptionID: sub.ID}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM messages WHERE tenant_id = $1 AND direction = 'outbound'`); n != 1 {
		t.Fatalf("outbound messages = %d", n)
	}
}

func TestReminderPlanAndDispatch(t *testing.T) {
	f := newTenant(t, "Reminder Shop")
	p, plan, _ := sharedProduct(t, f, 2)
	bot := telegramBot(t, f)
	in, _ := testC.Inbox.Ingest(context.Background(), social.InboundMessage{Platform: social.PlatformTelegram, AccountExternal: bot.ExternalID,
		ThreadExternal: "5150", MessageExternal: "1", SenderName: "Reminder Buyer", Body: "hi"})
	starts := time.Now().AddDate(0, 0, -28) // 30-day plan -> expires in 2 days
	sub, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: in.CustomerID, ProductID: p.ID, PlanID: &plan.ID, StartsAt: &starts})
	if err != nil {
		t.Fatal(err)
	}
	hour := 10
	rule, err := testC.Reminders.UpsertRule(f.ctx, f.tid, nil, reminder.RuleInput{Name: "2 days before", Trigger: reminder.BeforeExpiry,
		OffsetDays: 2, SendHour: hour, Channel: "preferred", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	// Plan "as of" two days before expiry at 09:00 local -> the 10:00 reminder is inside the horizon.
	loc := time.FixedZone("Asia/Dhaka", 6*3600)
	exp := sub.ExpiresAt.In(loc)
	planAt := time.Date(exp.Year(), exp.Month(), exp.Day()-2, 9, 0, 0, 0, loc)
	n, err := testC.Reminders.Plan(f.ctx, f.tid, planAt)
	if err != nil || n < 1 {
		t.Fatalf("plan created %d: %v", n, err)
	}
	if again, _ := testC.Reminders.Plan(f.ctx, f.tid, planAt); again != 0 {
		t.Fatalf("planner not idempotent: %d", again)
	}
	rems, _ := testC.Reminders.List(f.ctx, f.tid, reminder.Filter{CustomerID: &in.CustomerID}, nil, 10)
	var mine *reminder.Reminder
	for i := range rems {
		if rems[i].RuleID != nil && *rems[i].RuleID == rule.ID {
			mine = &rems[i]
		}
	}
	if mine == nil || mine.Status != reminder.StatusScheduled || mine.ScheduledAt.In(loc).Hour() != hour {
		t.Fatalf("planned reminders = %+v", rems)
	}
	sent, failed, err := testC.Reminders.Dispatch(f.ctx, f.tid, mine.ScheduledAt.Add(time.Minute))
	if err != nil || sent < 1 {
		t.Fatalf("dispatch sent=%d failed=%d err=%v", sent, failed, err)
	}
	rems, _ = testC.Reminders.List(f.ctx, f.tid, reminder.Filter{Status: strp(reminder.StatusSent)}, nil, 10)
	if len(rems) == 0 || rems[0].RenderedBody == nil || !strings.Contains(*rems[0].RenderedBody, "Reminder Buyer") || !strings.Contains(*rems[0].RenderedBody, "/p/") {
		t.Fatalf("sent reminder = %+v", rems)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM messages WHERE tenant_id = $1 AND direction = 'outbound' AND status = 'queued'`); n < 1 {
		t.Fatal("dispatch must queue an outbound message")
	}
	// sendReminderNow goes out immediately.
	now, err := testC.Reminders.SendNow(f.ctx, f.tid, sub.ID, nil, "telegram")
	if err != nil || now.Status != reminder.StatusSent {
		t.Fatalf("send now = %+v %v", now, err)
	}
}

func TestWebhooksSignatureAndSMS(t *testing.T) {
	cfg := testC.Cfg
	cfg.MetaAppSecret, cfg.MetaVerifyToken = "it-app-secret", "it-verify"
	c2, err := app.NewWithPool(cfg, testC.Log, testC.Pool)
	if err != nil {
		t.Fatal(err)
	}
	h := c2.HTTPHandler()
	f := newTenant(t, "Webhook Shop")
	wa, err := c2.Social.ConnectWhatsApp(f.ctx, f.tid, f.uid, social.ConnectWhatsAppInput{PhoneNumberID: uniq("pn"), BusinessAccountID: "w",
		AccessToken: "EAAG-it-whatsapp-access-token-123456", DisplayName: "WA"})
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"object":"whatsapp_business_account","entry":[{"id":"w","changes":[{"field":"messages","value":{"messaging_product":"whatsapp",
		"metadata":{"phone_number_id":%q},"contacts":[{"profile":{"name":"Webhook Buyer"},"wa_id":"8801555000111"}],
		"messages":[{"from":"8801555000111","id":"wamid.webhook1","timestamp":"%d","type":"text","text":{"body":"hello"}}]}}]}]}`, wa.ExternalID, time.Now().Unix())
	post := func(path, b string, hdr map[string]string) int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(b))
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := post("/webhooks/meta", body, nil); code != http.StatusForbidden {
		t.Fatalf("unsigned meta webhook: %d", code)
	}
	if code := post("/webhooks/meta", body, map[string]string{"X-Hub-Signature-256": social.SignMetaBody("wrong", []byte(body))}); code != http.StatusForbidden {
		t.Fatalf("badly signed meta webhook: %d", code)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM customers WHERE tenant_id = $1`); n != 0 {
		t.Fatal("rejected webhook created data")
	}
	if code := post("/webhooks/meta", body, map[string]string{"X-Hub-Signature-256": social.SignMetaBody("it-app-secret", []byte(body))}); code != http.StatusOK {
		t.Fatalf("signed meta webhook: %d", code)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM customers WHERE tenant_id = $1 AND name = 'Webhook Buyer' AND whatsapp = '01555000111'`); n != 1 {
		t.Fatalf("webhook customer = %d", n)
	}
	// Telegram: wrong / missing secret token -> 403.
	bot, err := c2.Social.ConnectTelegram(f.ctx, f.tid, f.uid, fmt.Sprintf("%d:AAItWebhookToken_%s", 900000000+time.Now().UnixNano()%1000000, uniq("t")))
	if err != nil {
		t.Fatal(err)
	}
	upd := `{"update_id":1,"message":{"message_id":7,"from":{"id":55,"is_bot":false,"first_name":"TG"},"chat":{"id":55,"type":"private"},"date":1730190000,"text":"hi"}}`
	path := "/webhooks/telegram/" + bot.ID.String()
	if code := post(path, upd, map[string]string{"X-Telegram-Bot-Api-Secret-Token": "nope"}); code != http.StatusForbidden {
		t.Fatalf("telegram wrong secret: %d", code)
	}
	if code := post(path, upd, map[string]string{"X-Telegram-Bot-Api-Secret-Token": c2.Social.WebhookSecret(bot.ID)}); code != http.StatusOK {
		t.Fatalf("telegram ok: %d", code)
	}
	// SMS forwarder: 401 without/with a bad token; the tenant token ingests into payment_inbox.
	sms := "You have received Tk 350.00 from 01711000001. Fee Tk 0.00. Balance Tk 1,000.00. TrxID ITSMS12345 at 29/10/2026 10:00"
	if code := post("/webhooks/payments/sms", sms, nil); code != http.StatusUnauthorized {
		t.Fatalf("sms no token: %d", code)
	}
	if code := post("/webhooks/payments/sms", sms, map[string]string{"Authorization": "Bearer sms_" + strings.Repeat("0", 32) + "_x"}); code != http.StatusUnauthorized {
		t.Fatalf("sms bad token: %d", code)
	}
	tok, hint, created, err := c2.Tenants.EnsureSMSWebhookToken(context.Background(), f.tid)
	if err != nil || !created || !strings.HasSuffix(tok, hint[len(hint)-4:]) {
		t.Fatalf("ensure token: %v %v", created, err)
	}
	if again, _, created2, _ := c2.Tenants.EnsureSMSWebhookToken(context.Background(), f.tid); created2 || again != "" {
		t.Fatal("token must be created once and never shown again")
	}
	if code := post("/webhooks/payments/sms", sms, map[string]string{"Authorization": "Bearer " + tok}); code != http.StatusOK {
		t.Fatalf("sms with token: %d", code)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM payment_inbox WHERE tenant_id = $1 AND trx_id = 'ITSMS12345'`); n != 1 {
		t.Fatalf("payment_inbox rows = %d", n)
	}
	tn, _ := c2.Tenants.Get(context.Background(), f.tid)
	if tn.Settings["sms_webhook_token_hint"] != hint {
		t.Fatalf("settings = %v", tn.Settings)
	}
}
