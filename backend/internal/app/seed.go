package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/modules/auth"
	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/platform/apperr"
	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/money"
	"provider_saas/internal/platform/pagination"
)

// Demo credentials created by `api seed`.
const (
	DemoEmail    = "demo@provider.test"
	DemoPassword = "Demo1234!"
)

// SeedResult summarises what Seed created.
type SeedResult struct {
	TenantID      uuid.UUID
	UserID        uuid.UUID
	Products      int
	MasterAccts   int
	Customers     int
	Subscriptions int
	Skipped       bool
	Engagement    EngagementResult
}

type seedPlan struct {
	name    string
	days    int
	priceTk int64
	costTk  int64
}

type seedProduct struct {
	name, category, delivery string
	warranty                 int
	plans                    []seedPlan
}

var seedProducts = []seedProduct{
	{"Netflix Premium", "streaming", catalog.DeliverySharedSlot, 30, []seedPlan{{"1 Month · 1 Screen", 30, 350, 200}, {"3 Months · 1 Screen", 90, 1000, 600}}},
	{"Spotify Premium", "music", catalog.DeliverySharedSlot, 30, []seedPlan{{"1 Month Family Slot", 30, 150, 80}, {"6 Months Family Slot", 180, 800, 450}}},
	{"Canva Pro", "design", catalog.DeliveryInviteLink, 7, []seedPlan{{"1 Month", 30, 100, 30}, {"1 Year", 365, 900, 300}}},
	{"ChatGPT Plus", "ai", catalog.DeliverySharedSlot, 15, []seedPlan{{"1 Month Shared", 30, 650, 450}}},
	{"YouTube Premium", "streaming", catalog.DeliveryInviteLink, 30, []seedPlan{{"1 Month Family", 30, 120, 60}, {"1 Year Family", 365, 1200, 600}}},
}

var seedCustomers = []struct{ name, phone, source, tag string }{
	{"Rahim Uddin", "01711000001", "facebook", "vip"}, {"Karim Hossain", "01811000002", "whatsapp", ""},
	{"Nusrat Jahan", "01911000003", "facebook", "student"}, {"Tanvir Ahmed", "01611000004", "telegram", ""},
	{"Sumaiya Akter", "01511000005", "instagram", "student"}, {"Mehedi Hasan", "01311000006", "facebook", ""},
	{"Farhana Rahman", "01711000007", "whatsapp", "vip"}, {"Arif Chowdhury", "01811000008", "manual", "reseller"},
	{"Jannatul Ferdous", "01911000009", "facebook", ""}, {"Sabbir Rahman", "01611000010", "telegram", ""},
	{"Tasnim Ara", "01511000011", "instagram", "student"}, {"Imran Kabir", "01711000012", "facebook", ""},
	{"Shila Begum", "01811000013", "whatsapp", ""}, {"Rakib Hasan", "01911000014", "facebook", "late-payer"},
	{"Mim Akter", "01611000015", "storefront", ""}, {"Fahim Shahriar", "01511000016", "telegram", "vip"},
	{"Sadia Islam", "01311000017", "facebook", ""}, {"Nayeem Khan", "01711000018", "whatsapp", ""},
	{"Lamia Haque", "01811000019", "instagram", ""}, {"Shakib Al Amin", "01911000020", "facebook", "late-payer"},
	{"Ruma Sultana", "01611000021", "manual", ""}, {"Hasibul Islam", "01511000022", "facebook", ""},
	{"Afsana Mimi", "01311000023", "whatsapp", "student"}, {"Jahid Hasan", "01711000024", "telegram", ""},
	{"Priya Das", "01811000025", "facebook", ""},
}

// pay modes for seeded subscriptions
const (
	payFull    = "full"    // markPaid with full amount (verified)
	payPartial = "partial" // verified partial payment -> due balance
	payPending = "pending" // bKash TrxID recorded, awaiting verification
	payNone    = "none"    // invoice only (baki)
)

type seedSub struct {
	cust      int
	product   int
	plan      int
	startDays int // start offset from now in days (negative = past)
	pay       string
	delivered bool
}

// Live shared-slot subscriptions must fit the 21 seeded slots (NF 10, SP 6, GPT 5).
var seedSubs = []seedSub{
	// expired (created first; expire_check frees their slots)
	{1, 0, 0, -40, payFull, true}, {5, 1, 0, -35, payFull, true}, {9, 3, 0, -33, payNone, true},
	{13, 2, 0, -45, payFull, true}, {19, 4, 0, -32, payPartial, true},
	// expiring within 2-3 days
	{0, 0, 0, -28, payFull, true}, {2, 1, 0, -28, payFull, true}, {3, 3, 0, -28, payPending, true},
	{4, 2, 0, -28, payFull, true}, {6, 4, 0, -28, payNone, true},
	// active
	{7, 0, 1, -10, payFull, true}, {8, 0, 0, -5, payFull, true}, {10, 0, 0, -3, payPartial, true},
	{11, 0, 0, -1, payPending, false}, {12, 0, 1, -40, payFull, true}, {14, 0, 0, -12, payFull, true},
	{15, 0, 0, -20, payNone, true}, {16, 1, 1, -30, payFull, true}, {17, 1, 0, -7, payFull, true},
	{18, 1, 0, -2, payPartial, true}, {20, 1, 0, -15, payFull, true}, {21, 3, 0, -9, payFull, true},
	{22, 3, 0, -4, payFull, true}, {23, 3, 0, -6, payNone, true}, {24, 2, 1, -100, payFull, true},
	{0, 2, 0, -10, payFull, true}, {1, 4, 1, -60, payFull, true}, {2, 4, 0, -14, payFull, true},
	{3, 4, 0, -8, payPartial, true}, {5, 0, 0, 0, payPending, false},
}

// Seed creates the demo tenant in ONE transaction (all-or-nothing). It is idempotent:
// if the demo user exists it does nothing.
func (c *Container) Seed(ctx context.Context) (SeedResult, error) {
	var res SeedResult
	sess, err := c.Auth.Login(ctx, auth.LoginInput{Identifier: DemoEmail, Password: DemoPassword}, auth.ClientMeta{})
	if err == nil && sess.Membership != nil {
		res.Skipped, res.TenantID, res.UserID = true, sess.Membership.Tenant.ID, sess.User.ID
	} else if err != nil && !errors.Is(err, apperr.ErrUnauthenticated) {
		return res, err
	} else {
		err = db.WithTx(ctx, c.Pool, func(ctx context.Context, _ pgx.Tx) error {
			var err error
			res, err = c.seed(ctx)
			return err
		})
		if err != nil {
			return res, err
		}
	}
	// Inbox / storefront / tickets / reminders / content demo data: its own all-or-nothing
	// transaction, applied once (also to a demo tenant seeded before these modules existed).
	res.Engagement, err = c.seedEngagement(ctx, res.TenantID, res.UserID)
	return res, err
}

func (c *Container) seed(ctx context.Context) (SeedResult, error) {
	var res SeedResult
	email, phone := DemoEmail, "01700000000"
	sess, err := c.Auth.Register(ctx, auth.RegisterInput{Name: "Demo Owner", Email: &email, Phone: &phone,
		Password: DemoPassword, BusinessName: "Demo Digital Store"}, auth.ClientMeta{})
	if err != nil {
		return res, fmt.Errorf("register demo owner: %w", err)
	}
	tid, uid := sess.Membership.Tenant.ID, sess.User.ID
	res.TenantID, res.UserID = tid, uid
	ctx = pauth.WithPrincipal(ctx, pauth.Principal{UserID: uid, TenantID: tid, Role: pauth.RoleOwner})
	now := time.Now()

	// catalog
	type prod struct {
		p     catalog.Product
		plans []catalog.Plan
	}
	prods := make([]prod, len(seedProducts))
	for i, sp := range seedProducts {
		cat := sp.category
		in := catalog.CreateInput{Name: sp.name, Category: &cat, DeliveryType: sp.delivery, WarrantyDays: sp.warranty}
		for j, pl := range sp.plans {
			in.Plans = append(in.Plans, catalog.PlanInput{Name: pl.name, DurationDays: pl.days, Price: money.FromMajor(pl.priceTk),
				Cost: money.FromMajor(pl.costTk), IsActive: true, SortOrder: j})
		}
		p, err := c.Catalog.Create(ctx, tid, in)
		if err != nil {
			return res, fmt.Errorf("product %s: %w", sp.name, err)
		}
		plans, err := c.Catalog.PlansByProducts(ctx, tid, []uuid.UUID{p.ID})
		if err != nil {
			return res, err
		}
		prods[i] = prod{p, plans[p.ID]}
		res.Products++
	}

	// suppliers + master accounts
	sp1, err := c.Inventory.CreateSupplier(ctx, tid, inventory.SupplierInput{Name: "Dhaka Digital Wholesale", Contact: strp("01755000111"), Rating: intp(5)})
	if err != nil {
		return res, err
	}
	sp2, err := c.Inventory.CreateSupplier(ctx, tid, inventory.SupplierInput{Name: "Global Accounts BD", Contact: strp("t.me/globalaccbd"), Rating: intp(4)})
	if err != nil {
		return res, err
	}
	masters := []struct {
		product  int
		supplier uuid.UUID
		label    string
		user     string
		slots    int
		costTk   int64
		expDays  int
	}{
		{0, sp1.ID, "NF-01", "nf.dhaka01@gmail.com", 5, 1100, 25},
		{0, sp1.ID, "NF-02", "nf.dhaka02@gmail.com", 5, 1100, 6},
		{1, sp2.ID, "SP-01", "spotify.family.bd@gmail.com", 6, 500, 60},
		{3, sp2.ID, "GPT-01", "gpt.team.bd@outlook.com", 5, 2400, 20},
	}
	for _, m := range masters {
		pw := "Seed-" + m.label + "-pass"
		exp := now.AddDate(0, 0, m.expDays)
		purchased := now.AddDate(0, 0, m.expDays-30)
		if _, err := c.Inventory.CreateMasterAccount(ctx, tid, inventory.CreateMasterInput{ProductID: prods[m.product].p.ID, SupplierID: &m.supplier,
			Label: m.label, Username: &m.user, Password: &pw, TotalSlots: m.slots, PurchaseCost: money.FromMajor(m.costTk),
			PurchasedAt: &purchased, ExpiresAt: &exp}); err != nil {
			return res, fmt.Errorf("master %s: %w", m.label, err)
		}
		res.MasterAccts++
	}

	// customers
	custs := make([]customer.Customer, len(seedCustomers))
	for i, sc := range seedCustomers {
		in := customer.CreateInput{Name: sc.name, Phone: strp(sc.phone), Source: sc.source}
		if sc.source == "whatsapp" {
			in.Whatsapp, in.PreferredChannel = strp(sc.phone), strp("whatsapp")
		}
		if sc.source == "facebook" {
			in.PreferredChannel = strp("facebook")
		}
		if sc.tag != "" {
			in.Tags = []string{sc.tag}
		}
		cu, err := c.Customers.Create(ctx, tid, uid, in)
		if err != nil {
			return res, fmt.Errorf("customer %s: %w", sc.name, err)
		}
		custs[i] = cu
		res.Customers++
	}

	// subscriptions (+ invoices, payments)
	var expiredIDs []uuid.UUID
	for i, ss := range seedSubs {
		pr := prods[ss.product]
		plan := pr.plans[ss.plan]
		starts := now.AddDate(0, 0, ss.startDays).Add(-time.Duration(i) * time.Minute)
		in := subscription.CreateInput{CustomerID: custs[ss.cust].ID, ProductID: pr.p.ID, PlanID: &plan.ID, StartsAt: &starts, CreateInvoice: true}
		trx := fmt.Sprintf("SEED%06dX", 100+i)
		switch ss.pay {
		case payFull:
			in.MarkPaid = &billing.PaymentInput{Method: "bkash", Amount: plan.Price, TrxID: &trx, SenderNumber: strp(seedCustomers[ss.cust].phone)}
		case payPartial:
			in.MarkPaid = &billing.PaymentInput{Method: "cash", Amount: plan.Price / 2}
		}
		sub, err := c.Subscriptions.Create(ctx, tid, uid, in)
		if err != nil {
			return res, fmt.Errorf("subscription %d: %w", i, err)
		}
		res.Subscriptions++
		if ss.pay == payPending {
			// All rows of the seed tx share created_at (now() = tx start), so "newest invoice"
			// is ambiguous: find the invoice whose item bills this subscription.
			invID, err := c.invoiceOfSubscription(ctx, tid, sub)
			if err != nil {
				return res, fmt.Errorf("pending payment %d: invoice lookup: %w", i, err)
			}
			if _, err := c.Billing.RecordPayment(ctx, tid, uid, &invID, &sub.CustomerID, billing.PaymentInput{Method: "bkash", Amount: plan.Price,
				TrxID: &trx, SenderNumber: strp(seedCustomers[ss.cust].phone)}, billing.RecordOptions{}); err != nil {
				return res, fmt.Errorf("pending payment %d: %w", i, err)
			}
		}
		if ss.delivered && subscription.IsLive(sub.Status) {
			note := "Profile & PIN sent on " + seedCustomers[ss.cust].source
			if _, err := c.Subscriptions.MarkDelivered(ctx, tid, sub.ID, &note); err != nil {
				return res, err
			}
		}
		if sub.Status == subscription.StatusExpired {
			expiredIDs = append(expiredIDs, sub.ID)
		}
		if i == 4 { // all expired ones are created: free their slots before selling the live ones
			if _, err := c.Subscriptions.ExpireCheck(ctx, tid, now); err != nil {
				return res, err
			}
		}
	}

	// one renewal of an expired subscription (keeps history via renewed_from_id)
	if len(expiredIDs) > 0 {
		trx := "SEEDRENEW01"
		if _, err := c.Subscriptions.Renew(ctx, tid, uid, expiredIDs[0], subscription.RenewInput{CreateInvoice: true,
			MarkPaid: &billing.PaymentInput{Method: "nagad", Amount: money.FromMajor(350), TrxID: &trx}}); err != nil {
			return res, fmt.Errorf("renew: %w", err)
		}
		res.Subscriptions++
	}

	// forwarded SMS: one matches a pending payment (auto-verified), one is unmatched
	sms := []string{
		fmt.Sprintf("You have received Tk 650.00 from %s. Fee Tk 0.00. Balance Tk 12,450.00. TrxID SEED000107X at %s",
			seedCustomers[3].phone, now.In(billing.Dhaka).Format("02/01/2006 15:04")),
		"Money Received.\nAmount: Tk 500.00\nSender: 01899000000\nRef: N/A\nTxnID: 7NGD8XYZ\nBalance: Tk 2,000.00\n" + now.In(billing.Dhaka).Format("02/01/2006 15:04"),
	}
	for _, s := range sms {
		if _, err := c.Billing.IngestSMS(ctx, tid, s, now); err != nil {
			return res, fmt.Errorf("ingest sms: %w", err)
		}
	}
	// a manual baki adjustment
	if _, err := c.Billing.AddLedgerAdjustment(ctx, tid, uid, custs[13].ID, billing.Debit, money.FromMajor(200), strp("পুরনো বাকি (old due)")); err != nil {
		return res, err
	}

	// time-based statuses
	if _, err := c.Subscriptions.ExpireCheck(ctx, tid, now); err != nil {
		return res, err
	}
	if _, err := c.Inventory.RefreshExpiryStatuses(ctx, tid, now); err != nil {
		return res, err
	}
	if _, err := c.Billing.MarkOverdue(ctx, tid, now); err != nil {
		return res, err
	}
	return res, nil
}

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }

// invoiceOfSubscription returns the invoice that bills sub.
func (c *Container) invoiceOfSubscription(ctx context.Context, tid uuid.UUID, sub subscription.Subscription) (uuid.UUID, error) {
	invs, _, _, err := c.Billing.Invoices(ctx, tid, billing.InvoiceFilter{CustomerID: &sub.CustomerID}, pagination.Page{First: pagination.MaxFirst})
	if err != nil {
		return uuid.Nil, err
	}
	ids := make([]uuid.UUID, len(invs))
	for i, inv := range invs {
		ids[i] = inv.ID
	}
	items, err := c.Billing.ItemsByInvoices(ctx, tid, ids)
	if err != nil {
		return uuid.Nil, err
	}
	for invID, its := range items {
		for _, it := range its {
			if it.SubscriptionID != nil && *it.SubscriptionID == sub.ID {
				return invID, nil
			}
		}
	}
	return uuid.Nil, fmt.Errorf("no invoice bills subscription %s", sub.ID)
}
