package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/content"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inbox"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/social"
	"provider_saas/internal/modules/storefront"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/ticket"
	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
	"provider_saas/internal/platform/vault"
)

// Demo channel ids (the Fake Meta client returns the same page id from /me/accounts).
const (
	DemoPageID    = "100200300400500"
	DemoBotID     = "7000000001"
	DemoStoreSlug = "demo"
	demoPageToken = "fake-page-token-demo-digital-store"
	demoBotToken  = "7000000001:AAFakeDemoTokenForSeedOnly_0000000000"
	seedOrderKey  = "seed"
)

// EngagementResult summarises the inbox/storefront/ticket/reminder/content seed.
type EngagementResult struct {
	Skipped        bool
	SocialAccounts int
	Conversations  int
	Messages       int
	Templates      int
	Tickets        int
	Reminders      int
	Orders         int
	Posts          int
	// SMSToken is the SMS-forwarder bearer token, returned only when the seed created it
	// (dev convenience; afterwards only the hint in tenant settings is visible).
	SMSToken string
}

type seedMsg struct {
	minsAgo int
	in      bool
	text    string
}

type seedConv struct {
	cust     int // index into seedCustomers
	telegram bool
	status   string
	labels   []string
	assign   bool
	msgs     []seedMsg
}

var seedConvs = []seedConv{
	{cust: 0, labels: []string{"payment"}, msgs: []seedMsg{
		{1500, true, "আসসালামু আলাইকুম, Netflix এর প্যাকেজ গুলো কি কি?"},
		{1495, false, "ওয়ালাইকুম আসসালাম! ১ মাস ৩৫০ টাকা, ৩ মাস ১০০০ টাকা।"},
		{1490, true, "৩ মাসেরটা নিব, bKash এ পাঠাচ্ছি"},
		{60, true, "TrxID: 9XK2L7M3QP পাঠিয়েছি, চেক করেন প্লিজ"},
	}},
	{cust: 2, assign: true, msgs: []seedMsg{
		{300, true, "Spotify family slot ache?"},
		{295, false, "জি আছে, ১৫০ টাকা/মাস"},
		{290, true, "ok 1 ta din"},
		{120, false, "পেমেন্ট করে TrxID দিন"},
		{30, true, "দিচ্ছি একটু পর"},
	}},
	{cust: 5, status: inbox.StatusPending, msgs: []seedMsg{
		{3000, true, "Canva Pro koto?"},
		{2990, false, "১ মাস ১০০ টাকা, ১ বছর ৯০০ টাকা"},
		{2980, true, "thanks, pore janabo"},
	}},
	{cust: 8, status: inbox.StatusClosed, labels: []string{"replacement"}, msgs: []seedMsg{
		{2000, true, "আমার Netflix কাজ করছে না, password চায়"},
		{1990, false, "দুঃখিত! আমরা চেক করছি, ১০ মিনিট সময় দিন"},
		{1900, false, "নতুন প্রোফাইল দেওয়া হয়েছে, এখন চেষ্টা করুন"},
		{1890, true, "এখন ঠিক আছে, ধন্যবাদ ❤️"},
	}},
	{cust: 11, labels: []string{"hot-lead"}, msgs: []seedMsg{
		{45, true, "ChatGPT Plus shared ache?"},
		{40, false, "জি, ৬৫০ টাকা/মাস"},
		{35, true, "ekhon nibo, number den"},
		{20, true, "??"},
	}},
	{cust: 3, telegram: true, msgs: []seedMsg{
		{720, true, "/start"},
		{719, false, "Demo Digital Store এ স্বাগতম! কিভাবে সাহায্য করতে পারি?"},
		{700, true, "ChatGPT renew করতে চাই"},
		{690, false, "রিনিউ লিংক পাঠাচ্ছি"},
		{10, true, "payment done, TrxID 7NGD8XYZ"},
	}},
	{cust: 9, telegram: true, msgs: []seedMsg{
		{5000, true, "/start"},
		{4990, false, "স্বাগতম!"},
		{4980, true, "YouTube Premium family ache?"},
	}},
	{cust: 15, telegram: true, msgs: []seedMsg{
		{200, true, "VPN ache?"},
		{190, false, "এখন VPN নেই, শীঘ্রই আসবে"},
		{180, true, "ok"},
		{175, false, "ধন্যবাদ!"},
		{170, true, "👍"},
		{5, true, "Netflix ৩ মাস কত?"},
	}},
}

// Three templates per category (bootstrap already created some keys; existing keys are kept).
var seedTemplates = []inbox.TemplateInput{
	{Key: "quick_reply.price", Name: "প্রাইস লিস্ট", Category: "quick_reply", Body: "আমাদের সব প্যাকেজ ও দাম দেখুন: {{link}}"},
	{Key: "quick_reply.payment", Name: "পেমেন্ট নির্দেশনা", Category: "quick_reply", Body: "bKash/Nagad এ Send Money করে TrxID টি এখানে পাঠান।"},
	{Key: "quick_reply.thanks", Name: "ধন্যবাদ", Category: "quick_reply", Body: "ধন্যবাদ {{customer}}! আর কোনো সাহায্য লাগলে জানাবেন।"},
	{Key: "reminder.before_3d", Name: "মেয়াদ শেষের ৩ দিন আগে", Category: "reminder", Body: "প্রিয় {{customer}}, আপনার {{product}} এর মেয়াদ {{expiry}} তারিখে শেষ হবে। রিনিউ: {{link}}"},
	{Key: "reminder.on_expiry", Name: "মেয়াদ শেষের দিন", Category: "reminder", Body: "প্রিয় {{customer}}, আজ আপনার {{product}} এর মেয়াদ শেষ। রিনিউ: {{link}}"},
	{Key: "reminder.before_1d", Name: "মেয়াদ শেষের ১ দিন আগে", Category: "reminder", Body: "প্রিয় {{customer}}, কাল আপনার {{product}} বন্ধ হয়ে যাবে। {{amount}} পাঠিয়ে রিনিউ করুন: {{link}}"},
	{Key: "delivery.default", Name: "ডেলিভারি মেসেজ", Category: "delivery", Body: "প্রিয় {{customer}}, আপনার {{product}} প্রস্তুত ✅ মেয়াদ: {{expiry}}\n{{credentials}}"},
	{Key: "delivery.invite", Name: "ইনভাইট লিংক ডেলিভারি", Category: "delivery", Body: "প্রিয় {{customer}}, আপনার {{product}} ইনভাইট পাঠানো হয়েছে। ইমেইল চেক করুন। মেয়াদ: {{expiry}}"},
	{Key: "delivery.replacement", Name: "রিপ্লেসমেন্ট ডেলিভারি", Category: "delivery", Body: "প্রিয় {{customer}}, আপনার {{product}} এর নতুন অ্যাকাউন্ট:\n{{credentials}}\nঅসুবিধার জন্য দুঃখিত।"},
	{Key: "invoice.default", Name: "ইনভয়েস", Category: "invoice", Body: "প্রিয় {{customer}}, আপনার বিল {{amount}}। বিস্তারিত: {{link}}"},
	{Key: "invoice.due", Name: "বাকি বিল", Category: "invoice", Body: "প্রিয় {{customer}}, আপনার {{amount}} বাকি আছে। পরিশোধ করুন: {{link}}"},
	{Key: "invoice.paid", Name: "পেমেন্ট পেয়েছি", Category: "invoice", Body: "ধন্যবাদ {{customer}}, আপনার {{amount}} পেমেন্ট পেয়েছি ✅"},
	{Key: "reminder.after_2d", Name: "মেয়াদ শেষের ২ দিন পর", Category: "winback", Body: "প্রিয় {{customer}}, আপনার {{product}} এর মেয়াদ শেষ হয়েছে। আবার চালু করতে রিপ্লাই দিন।"},
	{Key: "winback.discount", Name: "ফিরে আসুন অফার", Category: "winback", Body: "প্রিয় {{customer}}, আবার {{product}} নিলে ১০% ছাড়! অফার দেখুন: {{link}}"},
	{Key: "winback.feedback", Name: "ফিডব্যাক", Category: "winback", Body: "প্রিয় {{customer}}, কেন রিনিউ করেননি জানালে খুশি হব 🙏"},
	{Key: "other.greeting", Name: "স্বাগতম", Category: "other", Body: "আসসালামু আলাইকুম {{customer}}! Demo Digital Store এ স্বাগতম।"},
	{Key: "other.away", Name: "অফলাইন", Category: "other", Body: "আমরা এখন অফলাইনে আছি, সকাল ১০টায় রিপ্লাই দেব।"},
	{Key: "other.address", Name: "যোগাযোগ", Category: "other", Body: "WhatsApp: 01700000000 · Telegram: @demo_store_bot"},
}

const netflixDeliveryTemplate = "প্রিয় {{customer}}, আপনার {{product}} ({{plan}}) প্রস্তুত ✅\n{{credentials}}\nমেয়াদ: {{expiry}}\nঅন্য কারো সাথে শেয়ার করবেন না।"

func (c *Container) seedEngagement(ctx context.Context, tid, uid uuid.UUID) (EngagementResult, error) {
	var res EngagementResult
	var exists bool
	if err := db.WithTenantTx(ctx, c.Pool, tid, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM social_accounts WHERE tenant_id = $1 AND platform = 'facebook' AND external_id = $2)`,
			tid, DemoPageID).Scan(&exists)
	}); err != nil {
		return res, err
	}
	if exists {
		res.Skipped = true
		return res, nil
	}
	ctx = pauth.WithPrincipal(ctx, pauth.Principal{UserID: uid, TenantID: tid, Role: pauth.RoleOwner})
	err := db.WithPlatformTx(ctx, c.Pool, func(ctx context.Context, _ pgx.Tx) error {
		return c.seedEngagementTx(ctx, tid, uid, &res)
	})
	return res, err
}

func (c *Container) seedEngagementTx(ctx context.Context, tid, uid uuid.UUID, res *EngagementResult) error {
	now := time.Now()
	// ---- social accounts (fake tokens, encrypted in the vault like real ones)
	page, err := c.seedAccount(ctx, tid, uid, social.Account{Platform: social.PlatformFacebook, ExternalID: DemoPageID, Name: "Demo Digital Store",
		Username: strp("demodigitalstore"), Scopes: social.MetaScopes, WebhookVerified: true}, demoPageToken)
	if err != nil {
		return fmt.Errorf("seed page: %w", err)
	}
	bot, err := c.seedAccount(ctx, tid, uid, social.Account{Platform: social.PlatformTelegram, ExternalID: DemoBotID, Name: "Demo Store Bot",
		Username: strp("demo_store_bot"), Scopes: []string{}, WebhookVerified: true}, demoBotToken)
	if err != nil {
		return fmt.Errorf("seed bot: %w", err)
	}
	res.SocialAccounts = 2

	custs := make([]customer.Customer, len(seedCustomers))
	for i, sc := range seedCustomers {
		if custs[i], err = c.Customers.FindByPhone(ctx, tid, sc.phone); err != nil {
			return fmt.Errorf("seed customer %s: %w", sc.name, err)
		}
	}

	// ---- conversations: inbound through the real ingest path, outbound as sent history
	for ci, sc := range seedConvs {
		cust := custs[sc.cust]
		acct, platform, thread := page, social.PlatformFacebook, fmt.Sprintf("2468%011d", 1000+ci)
		if sc.telegram {
			acct, platform, thread = bot, social.PlatformTelegram, fmt.Sprintf("%d", 520000000+ci)
		}
		if _, err := c.Customers.AddIdentity(ctx, tid, cust.ID, platform, thread, strp(cust.Name), &acct.ID); err != nil {
			return fmt.Errorf("seed identity: %w", err)
		}
		var convID uuid.UUID
		for mi, m := range sc.msgs {
			at := now.Add(-time.Duration(m.minsAgo) * time.Minute)
			if m.in {
				r, err := c.Inbox.Ingest(ctx, social.InboundMessage{Platform: platform, AccountExternal: acct.ExternalID, ThreadExternal: thread,
					MessageExternal: fmt.Sprintf("seed-%d-%d", ci, mi), SenderName: cust.Name, Kind: "text", Body: m.text, SentAt: at})
				if err != nil {
					return fmt.Errorf("seed ingest: %w", err)
				}
				convID = r.ConversationID
			} else if err := c.seedOutbound(ctx, tid, uid, convID, m.text, at, fmt.Sprintf("seed-out-%d-%d", ci, mi)); err != nil {
				return err
			}
			res.Messages++
		}
		if err := c.seedConvState(ctx, tid, uid, convID, sc); err != nil {
			return err
		}
		res.Conversations++
	}

	// ---- templates (3 per category)
	for _, t := range seedTemplates {
		if _, err := c.Inbox.TemplateByKey(ctx, tid, t.Key); err == nil {
			continue
		}
		t.IsActive = true
		if _, err := c.Inbox.UpsertTemplate(ctx, tid, nil, t); err != nil {
			return fmt.Errorf("seed template %s: %w", t.Key, err)
		}
		res.Templates++
	}

	// ---- catalog: Netflix auto-delivery template + a fresh master account with labelled slots
	prods, err := c.Catalog.List(ctx, tid, catalog.Filter{}, 50)
	if err != nil {
		return err
	}
	byName := map[string]catalog.Product{}
	for _, p := range prods {
		byName[p.Name] = p
	}
	netflix, canva, spotify, youtube := byName["Netflix Premium"], byName["Canva Pro"], byName["Spotify Premium"], byName["YouTube Premium"]
	tpl := netflixDeliveryTemplate
	if _, err := c.Catalog.Update(ctx, tid, netflix.ID, catalog.UpdateInput{DeliveryTemplate: &tpl}); err != nil {
		return err
	}
	exp := now.AddDate(0, 0, 30)
	user, pw := "nf.dhaka03@gmail.com", "Seed-NF-03-pass"
	m3, err := c.Inventory.CreateMasterAccount(ctx, tid, inventory.CreateMasterInput{ProductID: netflix.ID, Label: "NF-03", Username: &user,
		Password: &pw, TotalSlots: 4, ExpiresAt: &exp, PurchaseCost: 110000})
	if err != nil {
		return fmt.Errorf("seed NF-03: %w", err)
	}
	slots, err := c.Inventory.SlotsByMasters(ctx, tid, []uuid.UUID{m3.ID})
	if err != nil {
		return err
	}
	for _, sl := range slots[m3.ID] {
		label, pin := fmt.Sprintf("Profile %d", sl.SlotNo), fmt.Sprintf("%d%d%d%d", sl.SlotNo, sl.SlotNo+1, sl.SlotNo+2, sl.SlotNo+3)
		if _, err := c.Inventory.UpdateSlot(ctx, tid, sl.ID, inventory.UpdateSlotInput{Label: &label, Pin: &pin}); err != nil {
			return err
		}
	}

	// ---- tickets: 2 replacements (1 resolved), a complaint and a question
	active := subscription.StatusActive
	subs, _, _, err := c.Subscriptions.List(ctx, tid, subscription.Filter{ProductID: &netflix.ID, Status: &active}, pagination.Page{First: 10})
	if err != nil {
		return err
	}
	var withSlot []subscription.Subscription
	for _, s := range subs {
		if s.SlotID != nil {
			withSlot = append(withSlot, s)
		}
	}
	if len(withSlot) < 2 {
		return fmt.Errorf("seed tickets: need 2 live Netflix subscriptions with slots, have %d", len(withSlot))
	}
	reason := "password_changed"
	t1, err := c.Tickets.Create(ctx, tid, uid, ticket.CreateInput{CustomerID: withSlot[0].CustomerID, SubscriptionID: &withSlot[0].ID,
		Kind: ticket.KindReplacement, Priority: "high", Title: "Netflix প্রোফাইলে password চাইছে", Reason: &reason})
	if err != nil {
		return fmt.Errorf("seed ticket 1: %w", err)
	}
	if _, err := c.Tickets.AddComment(ctx, tid, uid, t1.ID, "Supplier কে জানানো হয়েছে, NF-01 চেক করা হচ্ছে।", true); err != nil {
		return err
	}
	banned := "banned"
	t2, err := c.Tickets.Create(ctx, tid, uid, ticket.CreateInput{CustomerID: withSlot[1].CustomerID, SubscriptionID: &withSlot[1].ID,
		Kind: ticket.KindReplacement, Priority: "urgent", Title: "Netflix অ্যাকাউন্ট ব্যান হয়েছে", Reason: &banned})
	if err != nil {
		return fmt.Errorf("seed ticket 2: %w", err)
	}
	if _, err := c.Tickets.ResolveReplacement(ctx, tid, uid, t2.ID, nil, "banned", 2); err != nil {
		return fmt.Errorf("seed resolve replacement: %w", err)
	}
	spSubs, _, _, err := c.Subscriptions.List(ctx, tid, subscription.Filter{ProductID: &spotify.ID}, pagination.Page{First: 1})
	if err != nil || len(spSubs) == 0 {
		return fmt.Errorf("seed ticket 3: spotify subscription: %v", err)
	}
	t3, err := c.Tickets.Create(ctx, tid, uid, ticket.CreateInput{CustomerID: spSubs[0].CustomerID, SubscriptionID: &spSubs[0].ID,
		Kind: "complaint", Title: "Spotify মাঝে মাঝে লগআউট হয়ে যাচ্ছে"})
	if err != nil {
		return err
	}
	inProgress := ticket.StatusInProgress
	if _, err := c.Tickets.Update(ctx, tid, uid, t3.ID, ticket.UpdateInput{Status: &inProgress, AssignedTo: &uid}); err != nil {
		return err
	}
	t4, err := c.Tickets.Create(ctx, tid, uid, ticket.CreateInput{CustomerID: custs[9].ID, Kind: "question",
		Title: "YouTube Premium family তে কয়জন যোগ করা যায়?"})
	if err != nil {
		return err
	}
	waiting := ticket.StatusWaitingCustomer
	if _, err := c.Tickets.Update(ctx, tid, uid, t4.ID, ticket.UpdateInput{Status: &waiting}); err != nil {
		return err
	}
	res.Tickets = 4

	// ---- published storefront "demo"
	pub := true
	if _, err := c.Storefront.Upsert(ctx, tid, storefront.Input{Slug: strp(DemoStoreSlug), DisplayName: strp("Demo Digital Store"),
		Tagline: strp("Netflix · Spotify · Canva · ChatGPT — সবচেয়ে কম দামে, ইনস্ট্যান্ট ডেলিভারি"), Whatsapp: strp("01700000000"),
		MessengerURL: strp("https://m.me/demodigitalstore"), Telegram: strp("@demo_store_bot"), IsPublished: &pub,
		Theme:               map[string]any{"primaryColor": "#0f766e"},
		PaymentInstructions: strp("bKash/Nagad এ Send Money করে TrxID দিন। Send money via bKash/Nagad and share the TrxID."),
		PaymentNumbers: []any{
			map[string]any{"method": "bkash", "number": "01700000000", "type": "personal"},
			map[string]any{"method": "nagad", "number": "01800000000", "type": "personal"},
		}}); err != nil {
		return fmt.Errorf("seed storefront: %w", err)
	}

	// ---- 5 orders in mixed states
	plan := func(p catalog.Product) catalog.Plan {
		ps, _ := c.Catalog.PlansByProducts(ctx, tid, []uuid.UUID{p.ID})
		return ps[p.ID][0]
	}
	canvaPlan := plan(canva)
	// An SMS for order 5 arrives first, so placing the order auto-verifies its TrxID.
	sms := fmt.Sprintf("You have received Tk %d.00 from 01911000105. Fee Tk 0.00. Balance Tk 15,000.00. TrxID SEEDORD05X at %s",
		canvaPlan.Price/100, now.In(billing.Dhaka).Format("02/01/2006 15:04"))
	if _, err := c.Billing.IngestSMS(ctx, tid, "bKash", sms, now); err != nil {
		return err
	}
	type ord struct {
		p                    catalog.Product
		name, phone, trx, do string
	}
	orders := []ord{
		{spotify, "Rafi Islam", "01911000101", "", ""},
		{netflix, "Sharmin Akter", "01711000102", "SEEDORD02X", ""},
		{canva, "Rahim Uddin", "01711000001", "SEEDORD03X", "confirm"},
		{youtube, "Tahmid Hasan", "01811000104", "", "cancel"},
		{canva, "Nabila Chowdhury", "01911000105", "SEEDORD05X", ""},
	}
	bkash := "bkash"
	for _, o := range orders {
		pl := plan(o.p)
		in := storefront.PlaceOrderInput{Slug: DemoStoreSlug, ProductID: o.p.ID, PlanID: pl.ID, Quantity: 1, CustomerName: o.name, CustomerPhone: o.phone}
		if o.trx != "" {
			trx := o.trx
			in.TrxID, in.PaymentMethod = &trx, &bkash
		}
		po, err := c.Storefront.PlaceOrder(ctx, seedOrderKey, in)
		if err != nil {
			return fmt.Errorf("seed order %s: %w", o.name, err)
		}
		res.Orders++
		if o.do == "" {
			continue
		}
		var id uuid.UUID
		if err := db.WithTenantTx(ctx, c.Pool, tid, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT id FROM orders WHERE tenant_id = $1 AND number = $2`, tid, po.Number).Scan(&id)
		}); err != nil {
			return err
		}
		switch o.do {
		case "confirm":
			_, err = c.Storefront.ConfirmOrder(ctx, tid, uid, id, nil, strp("Canva invite পাঠানো হয়েছে ইমেইলে"))
		case "cancel":
			_, err = c.Storefront.CancelOrder(ctx, tid, uid, id, strp("customer changed mind"))
		}
		if err != nil {
			return fmt.Errorf("seed order %s %s: %w", o.name, o.do, err)
		}
	}

	// ---- reminders for upcoming expiries (default rules from tenant bootstrap)
	if res.Reminders, err = c.Reminders.Plan(ctx, tid, now); err != nil {
		return fmt.Errorf("seed reminders: %w", err)
	}

	// ---- one scheduled post
	at := now.Add(48 * time.Hour).Truncate(time.Hour)
	if _, err := c.Content.Create(ctx, tid, uid, content.PostInput{ScheduledAt: &at, SocialAccountIDs: []uuid.UUID{page.ID, bot.ID},
		Body: "🎉 উইকেন্ড অফার! Netflix Premium ১ মাস মাত্র ৩২০ টাকা, Spotify Family ১৪০ টাকা। অর্ডার করুন: https://app.example/s/demo"}); err != nil {
		return fmt.Errorf("seed post: %w", err)
	}
	res.Posts = 1

	// ---- SMS forwarder token (plaintext only now; afterwards only the hint)
	tok, _, created, err := c.Tenants.EnsureSMSWebhookToken(ctx, tid)
	if err != nil {
		return err
	}
	if created {
		res.SMSToken = tok
	}
	return nil
}

// seedAccount stores the token in the vault and upserts the account row.
func (c *Container) seedAccount(ctx context.Context, tid, uid uuid.UUID, a social.Account, token string) (social.Account, error) {
	sid, err := c.Vault.Store(ctx, tid, vault.KindSocialToken, []byte(token))
	if err != nil {
		return a, err
	}
	a.TokenSecretID, a.ConnectedBy = &sid, &uid
	var out social.Account
	err = db.WithTenantTx(ctx, c.Pool, tid, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = social.Repository{}.Upsert(ctx, tx, tid, a)
		return err
	})
	return out, err
}

func (c *Container) seedOutbound(ctx context.Context, tid, uid, convID uuid.UUID, body string, at time.Time, ext string) error {
	return db.WithTenantTx(ctx, c.Pool, tid, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO messages (tenant_id, conversation_id, direction, external_id, kind, body, sent_by_user_id, status, sent_at)
			VALUES ($1,$2,'outbound',$3,'text',$4,$5,'sent',$6)`, tid, convID, ext, body, uid, at)
		return err
	})
}

// seedConvState recomputes preview/unread from the messages and applies status/labels/assignee.
func (c *Container) seedConvState(ctx context.Context, tid, uid, convID uuid.UUID, sc seedConv) error {
	status := sc.status
	if status == "" {
		status = inbox.StatusOpen
	}
	labels := sc.labels
	if labels == nil {
		labels = []string{}
	}
	var assigned *uuid.UUID
	if sc.assign {
		assigned = &uid
	}
	return db.WithTenantTx(ctx, c.Pool, tid, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE conversations c SET last_message_at = m.sent_at, last_message_preview = left(m.body, 120),
			unread_count = (SELECT count(*) FROM messages x WHERE x.conversation_id = c.id AND x.direction = 'inbound'
			  AND x.sent_at > COALESCE((SELECT max(sent_at) FROM messages y WHERE y.conversation_id = c.id AND y.direction = 'outbound'), '-infinity')),
			status = $3, labels = $4, assigned_to = $5
			FROM (SELECT sent_at, body FROM messages WHERE tenant_id = $1 AND conversation_id = $2 ORDER BY sent_at DESC LIMIT 1) m
			WHERE c.tenant_id = $1 AND c.id = $2`, tid, convID, status, labels, assigned)
		return err
	})
}
