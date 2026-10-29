//go:build integration

// Integration tests run against a real Postgres (TEST_DATABASE_URL, default psaas_dev):
//
//	go test -tags integration ./...
package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/app"
	"provider_saas/internal/modules/auth"
	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/platform/apperr"
	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/config"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/jobs"
	"provider_saas/internal/platform/pagination"
)

var testC *app.Container

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://amdadulhq@localhost:5432/psaas_dev?sslmode=disable"
	}
	cfg, err := config.FromEnv(func(k string) string {
		switch k {
		case "DATABASE_URL":
			return url
		case "JWT_SECRET":
			return "integration-test-secret-integration-test"
		case "VAULT_KEY":
			return strings.Repeat("ab", 32)
		case "APP_ENV":
			return "test"
		case "RATE_LIMIT_RPS":
			return "0"
		}
		return ""
	})
	if err != nil {
		fmt.Println("config:", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx := context.Background()
	if err := db.Migrate(ctx, url, log); err != nil {
		fmt.Println("migrate:", err)
		os.Exit(1)
	}
	testC, err = app.New(ctx, cfg, log)
	if err != nil {
		fmt.Println("container:", err)
		os.Exit(1)
	}
	code := m.Run()
	testC.Close()
	os.Exit(code)
}

type tenantFx struct {
	tid, uid uuid.UUID
	ctx      context.Context
	session  auth.Session
}

func newTenant(t *testing.T, business string) tenantFx {
	t.Helper()
	email := fmt.Sprintf("it-%s@provider.test", uuid.NewString()[:12])
	s, err := testC.Auth.Register(context.Background(), auth.RegisterInput{Name: "IT Owner", Email: &email, Password: "Passw0rd!", BusinessName: business}, auth.ClientMeta{})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	p := pauth.Principal{UserID: s.User.ID, TenantID: s.Membership.Tenant.ID, Role: pauth.RoleOwner}
	return tenantFx{tid: p.TenantID, uid: p.UserID, ctx: pauth.WithPrincipal(context.Background(), p), session: s}
}

func mustCustomer(t *testing.T, f tenantFx, name string) customer.Customer {
	t.Helper()
	c, err := testC.Customers.Create(f.ctx, f.tid, f.uid, customer.CreateInput{Name: name, Phone: strp("01712345678")})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// sharedProduct creates a SHARED_SLOT product with one 30-day plan and one master account of n slots.
func sharedProduct(t *testing.T, f tenantFx, slots int) (catalog.Product, catalog.Plan, inventory.MasterAccount) {
	t.Helper()
	p, err := testC.Catalog.Create(f.ctx, f.tid, catalog.CreateInput{Name: "Netflix", DeliveryType: catalog.DeliverySharedSlot,
		Plans: []catalog.PlanInput{{Name: "1 Month", DurationDays: 30, Price: 35000, Cost: 20000, IsActive: true}}})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := testC.Catalog.PlansByProducts(f.ctx, f.tid, []uuid.UUID{p.ID})
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().AddDate(0, 2, 0)
	u, pw := "master@gmail.com", "s3cret-pass"
	m, err := testC.Inventory.CreateMasterAccount(f.ctx, f.tid, inventory.CreateMasterInput{ProductID: p.ID, Label: "NF-T",
		Username: &u, Password: &pw, TotalSlots: slots, ExpiresAt: &exp})
	if err != nil {
		t.Fatal(err)
	}
	return p, plans[p.ID][0], m
}

func strp(s string) *string { return &s }

func TestTenantIsolation(t *testing.T) {
	a, b := newTenant(t, "Tenant A Store"), newTenant(t, "Tenant B Store")
	cb := mustCustomer(t, b, "B's secret customer")

	// Service layer: tenant A cannot read B's customer by id...
	if _, err := testC.Customers.Get(a.ctx, a.tid, cb.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("tenant A read tenant B customer: err=%v", err)
	}
	// ...nor see it in lists / batch loads.
	items, _, total, err := testC.Customers.List(a.ctx, a.tid, customer.Filter{}, pagination.Page{First: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range items {
		if c.ID == cb.ID {
			t.Fatal("tenant B customer leaked into tenant A list")
		}
	}
	if total != 0 {
		t.Fatalf("tenant A total = %d, want 0", total)
	}
	if m, _ := testC.Customers.GetByIDs(a.ctx, a.tid, []uuid.UUID{cb.ID}); len(m) != 0 {
		t.Fatal("GetByIDs leaked across tenants")
	}
	// Update/delete across tenants fail too.
	if _, err := testC.Customers.Update(a.ctx, a.tid, cb.ID, customer.UpdateInput{Name: strp("hacked")}); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("cross-tenant update err=%v", err)
	}

	// RLS defense in depth: as a non-superuser role with app.tenant_id = A, B's row is invisible
	// even without a tenant_id filter in the query.
	_, _ = testC.Pool.Exec(context.Background(), `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'psaas_rls_probe') THEN CREATE ROLE psaas_rls_probe NOLOGIN; END IF; END $$`)
	if _, err := testC.Pool.Exec(context.Background(), `GRANT SELECT ON customers TO psaas_rls_probe`); err != nil {
		t.Fatal(err)
	}
	var seen int
	err = db.WithTenantTx(context.Background(), testC.Pool, a.tid, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL ROLE psaas_rls_probe`); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT count(*) FROM customers WHERE id = $1`, cb.ID).Scan(&seen)
	})
	if err != nil || seen != 0 {
		t.Fatalf("RLS: tenant A saw %d of B's rows (err=%v)", seen, err)
	}
	// Sanity: the same probe under tenant B sees it.
	_ = db.WithTenantTx(context.Background(), testC.Pool, b.tid, func(ctx context.Context, tx pgx.Tx) error {
		_, _ = tx.Exec(ctx, `SET LOCAL ROLE psaas_rls_probe`)
		return tx.QueryRow(ctx, `SELECT count(*) FROM customers WHERE id = $1`, cb.ID).Scan(&seen)
	})
	if seen != 1 {
		t.Fatalf("RLS: tenant B saw %d of its own rows", seen)
	}
}

func TestRegisterBootstrapsTenant(t *testing.T) {
	f := newTenant(t, "Rahim's Netflix BD")
	if f.session.Membership.Tenant.Slug != "rahims-netflix-bd" && !strings.HasPrefix(f.session.Membership.Tenant.Slug, "rahims-netflix-bd-") {
		t.Fatalf("slug = %s", f.session.Membership.Tenant.Slug)
	}
	var stores, rules, tpls int
	err := testC.Pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM storefronts WHERE tenant_id = $1),
		(SELECT count(*) FROM reminder_rules WHERE tenant_id = $1),
		(SELECT count(*) FROM message_templates WHERE tenant_id = $1)`, f.tid).Scan(&stores, &rules, &tpls)
	if err != nil || stores != 1 || rules != 3 || tpls < 3 {
		t.Fatalf("bootstrap: storefronts=%d rules=%d templates=%d err=%v", stores, rules, tpls, err)
	}
	// Second registration with same business name gets a unique slug.
	g := newTenant(t, "Rahim's Netflix BD")
	if g.session.Membership.Tenant.Slug == f.session.Membership.Tenant.Slug {
		t.Fatal("duplicate slug")
	}
}

func TestAuthFlows(t *testing.T) {
	f := newTenant(t, "Auth Flow Shop")
	email := *f.session.User.Email
	if _, err := testC.Auth.Login(context.Background(), auth.LoginInput{Identifier: email, Password: "wrong-pass1"}, auth.ClientMeta{}); !errors.Is(err, apperr.ErrUnauthenticated) {
		t.Fatalf("bad password: %v", err)
	}
	s, err := testC.Auth.Login(context.Background(), auth.LoginInput{Identifier: strings.ToUpper(email), Password: "Passw0rd!"}, auth.ClientMeta{IP: "127.0.0.1"})
	if err != nil || s.Membership == nil || s.Membership.Tenant.ID != f.tid {
		t.Fatalf("login: %v %+v", err, s.Membership)
	}
	p, err := testC.Tokens.Parse(s.AccessToken)
	if err != nil || p.TenantID != f.tid || p.Role != "owner" {
		t.Fatalf("claims: %+v %v", p, err)
	}
	r1, err := testC.Auth.Refresh(context.Background(), s.RefreshToken, auth.ClientMeta{})
	if err != nil || r1.Membership == nil || r1.Membership.Tenant.ID != f.tid {
		t.Fatalf("refresh: %v", err)
	}
	// Replaying the rotated token is rejected and revokes the whole family.
	if _, err := testC.Auth.Refresh(context.Background(), s.RefreshToken, auth.ClientMeta{}); err == nil {
		t.Fatal("refresh token reuse accepted")
	}
	if _, err := testC.Auth.Refresh(context.Background(), r1.RefreshToken, auth.ClientMeta{}); err == nil {
		t.Fatal("token family not revoked after reuse")
	}
	// Switch tenant: not a member of another tenant -> forbidden.
	other := newTenant(t, "Other Shop")
	if _, err := testC.Auth.SwitchTenant(context.Background(), f.uid, other.tid, auth.ClientMeta{}); !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("switch to foreign tenant: %v", err)
	}
	// Duplicate registration -> conflict.
	_, err = testC.Auth.Register(context.Background(), auth.RegisterInput{Name: "x", Email: &email, Password: "Passw0rd!", BusinessName: "x"}, auth.ClientMeta{})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("duplicate register: %v", err)
	}
}

func TestCreateSubscriptionSlotsAndBilling(t *testing.T) {
	f := newTenant(t, "Slot Shop")
	p, plan, m := sharedProduct(t, f, 2)
	c := mustCustomer(t, f, "Karim")

	trx := "9AB12CD34E"
	s1, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID,
		CreateInvoice: true, MarkPaid: &billing.PaymentInput{Method: "bkash", Amount: 20000, TrxID: &trx}})
	if err != nil {
		t.Fatal(err)
	}
	if s1.SlotID == nil || s1.Status != subscription.StatusActive || !s1.ExpiresAt.Equal(s1.StartsAt.AddDate(0, 0, 30)) {
		t.Fatalf("sub = %+v", s1)
	}
	// Partial payment: invoice 350 - paid 200 -> due 150.
	cu, _ := testC.Customers.Get(f.ctx, f.tid, c.ID)
	if cu.DueBalance != 15000 || cu.LifetimeValue != 20000 {
		t.Fatalf("due=%d ltv=%d", cu.DueBalance, cu.LifetimeValue)
	}
	invs, _, _, _ := testC.Billing.Invoices(f.ctx, f.tid, billing.InvoiceFilter{CustomerID: &c.ID}, pagination.Page{First: 5})
	if len(invs) != 1 || invs[0].Status != billing.InvoicePartial || invs[0].PaidAmount != 20000 || !strings.HasPrefix(invs[0].Number, "INV-") {
		t.Fatalf("invoice = %+v", invs)
	}
	// Same TrxID again -> conflict.
	_, err = testC.Billing.RecordPayment(f.ctx, f.tid, f.uid, &invs[0].ID, nil, billing.PaymentInput{Method: "bkash", Amount: 15000, TrxID: &trx}, billing.RecordOptions{})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("dup trx: %v", err)
	}
	// Pending bKash payment, then the SMS arrives and auto-verifies it.
	trx2 := "7QWE8RTY9U"
	pay, err := testC.Billing.RecordPayment(f.ctx, f.tid, f.uid, &invs[0].ID, nil, billing.PaymentInput{Method: "bkash", Amount: 15000, TrxID: &trx2}, billing.RecordOptions{})
	if err != nil || pay.Status != billing.PaymentPending {
		t.Fatalf("pending payment: %+v %v", pay, err)
	}
	item, err := testC.Billing.IngestSMS(f.ctx, f.tid, "You have received Tk 150.00 from 01712345678. Fee Tk 0.00. Balance Tk 1,250.00. TrxID 7QWE8RTY9U at 29/10/2026 10:15", time.Now())
	if err != nil || item.Status != "matched" {
		t.Fatalf("sms: %+v %v", item, err)
	}
	inv, _ := testC.Billing.Invoice(f.ctx, f.tid, invs[0].ID)
	cu, _ = testC.Customers.Get(f.ctx, f.tid, c.ID)
	if inv.Status != billing.InvoicePaid || cu.DueBalance != 0 || cu.LifetimeValue != 35000 {
		t.Fatalf("after sms: invoice=%s due=%d ltv=%d", inv.Status, cu.DueBalance, cu.LifetimeValue)
	}
	led, _ := testC.Billing.Ledger(f.ctx, f.tid, c.ID)
	var sum int64
	for _, e := range led {
		if e.EntryType == billing.Debit {
			sum += e.Amount
		} else {
			sum -= e.Amount
		}
	}
	if sum != cu.DueBalance {
		t.Fatalf("ledger sum %d != cached due %d", sum, cu.DueBalance)
	}

	// Second slot, then no free slot -> CONFLICT.
	if _, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID}); err != nil {
		t.Fatal(err)
	}
	_, err = testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID, CreateInvoice: true})
	if !errors.Is(err, apperr.ErrConflict) || !strings.Contains(err.Error(), "no free slot") {
		t.Fatalf("want no free slot conflict, got %v", err)
	}
	// The failed sale rolled back: no stray invoice.
	invs, _, total, _ := testC.Billing.Invoices(f.ctx, f.tid, billing.InvoiceFilter{CustomerID: &c.ID}, pagination.Page{First: 5})
	if total != 1 {
		t.Fatalf("invoices after failed sale = %d", total)
	}
	// Plan of another product is rejected.
	p2, _ := testC.Catalog.Create(f.ctx, f.tid, catalog.CreateInput{Name: "Canva", DeliveryType: catalog.DeliveryInviteLink})
	if _, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p2.ID, PlanID: &plan.ID}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("foreign plan: %v", err)
	}

	// Cancel frees the slot; renew of the first moves its slot to the new row.
	if _, err := testC.Subscriptions.Cancel(f.ctx, f.tid, s1.ID, strp("test")); err != nil {
		t.Fatal(err)
	}
	free, _ := testC.Inventory.FreeCountsByMaster(f.ctx, f.tid, []uuid.UUID{m.ID})
	if free[m.ID] != 1 {
		t.Fatalf("free after cancel = %d", free[m.ID])
	}
	if _, err := testC.Subscriptions.Renew(f.ctx, f.tid, f.uid, s1.ID, subscription.RenewInput{}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("renew cancelled: %v", err)
	}
}

func TestRenewAndExpireCheck(t *testing.T) {
	f := newTenant(t, "Renew Shop")
	p, plan, m := sharedProduct(t, f, 3)
	c := mustCustomer(t, f, "Nusrat")
	start := time.Now().AddDate(0, 0, -28)
	old, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID, StartsAt: &start})
	if err != nil || old.Status != subscription.StatusExpiring {
		t.Fatalf("expiring sub: %+v %v", old, err)
	}
	renewed, err := testC.Subscriptions.Renew(f.ctx, f.tid, f.uid, old.ID, subscription.RenewInput{CreateInvoice: true})
	if err != nil {
		t.Fatal(err)
	}
	if renewed.RenewedFromID == nil || *renewed.RenewedFromID != old.ID || renewed.SlotID == nil || *renewed.SlotID != *old.SlotID ||
		!renewed.StartsAt.Equal(old.ExpiresAt) || renewed.Status != subscription.StatusPending {
		t.Fatalf("renewed = %+v", renewed)
	}
	if _, err := testC.Subscriptions.Renew(f.ctx, f.tid, f.uid, old.ID, subscription.RenewInput{}); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("double renew: %v", err)
	}
	// An unrelated sub that is expired by the time expire_check runs frees its slot.
	other := mustCustomer(t, f, "Tanvir")
	s2, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: other.ID, ProductID: p.ID, PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().AddDate(0, 0, 31)
	res, err := testC.Subscriptions.ExpireCheck(f.ctx, f.tid, future)
	if err != nil {
		t.Fatal(err)
	}
	if res.Expired != 2 || res.SlotsFreed != 1 || res.Activated != 1 {
		t.Fatalf("expire check = %+v", res)
	}
	got, _ := testC.Subscriptions.GetByIDs(f.ctx, f.tid, []uuid.UUID{old.ID, renewed.ID, s2.ID})
	if got[old.ID].Status != subscription.StatusExpired || got[s2.ID].Status != subscription.StatusExpired || got[renewed.ID].Status != subscription.StatusExpiring { // renewal ends ~1 day after "future"
		t.Fatalf("statuses: old=%s s2=%s renewed=%s", got[old.ID].Status, got[s2.ID].Status, got[renewed.ID].Status)
	}
	free, _ := testC.Inventory.FreeCountsByMaster(f.ctx, f.tid, []uuid.UUID{m.ID})
	if free[m.ID] != 2 { // 3 slots - renewed holds 1
		t.Fatalf("free = %d", free[m.ID])
	}
}

func TestConcurrentSalesDoNotOversell(t *testing.T) {
	f := newTenant(t, "Rush Shop")
	p, plan, _ := sharedProduct(t, f, 3)
	c := mustCustomer(t, f, "Crowd")
	var wg sync.WaitGroup
	var mu sync.Mutex
	var ok, conflict int
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID, CreateInvoice: true})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, apperr.ErrConflict):
				conflict++
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 3 || conflict != 5 {
		t.Fatalf("ok=%d conflict=%d", ok, conflict)
	}
	var dup int
	_ = testC.Pool.QueryRow(context.Background(), `SELECT count(*) FROM (SELECT slot_id FROM subscriptions WHERE tenant_id = $1
		AND slot_id IS NOT NULL GROUP BY slot_id HAVING count(*) > 1) d`, f.tid).Scan(&dup)
	if dup != 0 {
		t.Fatalf("%d slots oversold", dup)
	}
}

func TestVaultRevealIsAudited(t *testing.T) {
	f := newTenant(t, "Vault Shop")
	_, _, m := sharedProduct(t, f, 1)
	cred, err := testC.Inventory.RevealCredential(f.ctx, f.tid, m.ID)
	if err != nil || cred.Username != "master@gmail.com" || cred.Password != "s3cret-pass" {
		t.Fatalf("reveal: %+v %v", cred, err)
	}
	if m.LoginHint == nil || *m.LoginHint != "ma***@gmail.com" {
		t.Fatalf("hint = %v", m.LoginHint)
	}
	var n int
	_ = testC.Pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE tenant_id = $1 AND action = 'vault.reveal' AND user_id = $2`, f.tid, f.uid).Scan(&n)
	if n != 1 {
		t.Fatalf("audit rows = %d", n)
	}
	var plain int
	_ = testC.Pool.QueryRow(context.Background(), `SELECT count(*) FROM vault_secrets WHERE tenant_id = $1 AND position('s3cret' in convert_from(ciphertext, 'SQL_ASCII')) > 0`, f.tid).Scan(&plain)
	if plain != 0 {
		t.Fatal("plaintext found in vault ciphertext")
	}
	// Another tenant cannot reveal it.
	g := newTenant(t, "Thief Shop")
	if _, err := testC.Inventory.RevealCredential(g.ctx, g.tid, m.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("cross-tenant reveal: %v", err)
	}
}

func TestJobsQueue(t *testing.T) {
	ctx := context.Background()
	q := jobs.NewQueue(testC.Pool)
	kind := "test.job." + uuid.NewString()[:8]
	id1, err := q.Enqueue(ctx, kind, map[string]int{"n": 1}, time.Now(), kind+":k")
	if err != nil || id1 == 0 {
		t.Fatalf("enqueue: %d %v", id1, err)
	}
	if id2, _ := q.Enqueue(ctx, kind, nil, time.Now(), kind+":k"); id2 != 0 {
		t.Fatal("dedupe failed")
	}
	w := jobs.NewWorker(testC.Pool, slog.Default())
	var calls int
	w.Register(kind, func(ctx context.Context, j jobs.Job) error {
		calls++
		if calls == 1 {
			return errors.New("transient")
		}
		return nil
	})
	// Only run our job kind: other kinds may be queued in the shared dev DB, so drain ours.
	for i := 0; i < 50 && calls == 0; i++ {
		if _, err := w.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var attempts int
	var runAt time.Time
	var lastErr *string
	_ = testC.Pool.QueryRow(ctx, `SELECT attempts, run_at, last_error FROM jobs WHERE id = $1`, id1).Scan(&attempts, &runAt, &lastErr)
	if calls != 1 || attempts != 1 || lastErr == nil || !runAt.After(time.Now()) {
		t.Fatalf("after failure: calls=%d attempts=%d err=%v runAt=%v", calls, attempts, lastErr, runAt)
	}
	_, _ = testC.Pool.Exec(ctx, `UPDATE jobs SET run_at = now() WHERE id = $1`, id1)
	for i := 0; i < 50 && calls == 1; i++ {
		_, _ = w.RunOnce(ctx)
	}
	var done *time.Time
	_ = testC.Pool.QueryRow(ctx, `SELECT done_at FROM jobs WHERE id = $1`, id1).Scan(&done)
	if calls != 2 || done == nil {
		t.Fatalf("retry: calls=%d done=%v", calls, done)
	}
}

func TestAnalyticsAndSeed(t *testing.T) {
	res, err := testC.Seed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx := pauth.WithPrincipal(context.Background(), pauth.Principal{UserID: res.UserID, TenantID: res.TenantID, Role: "owner"})
	d, err := testC.Analytics.Dashboard(ctx, res.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalCustomers < 25 || d.ActiveSubscriptions == 0 || d.Currency != "BDT" || d.MRR <= 0 {
		t.Fatalf("dashboard = %+v", d)
	}
	for _, f := range []func() error{
		func() error { _, err := testC.Analytics.RevenueSeries(ctx, res.TenantID, 30); return err },
		func() error { _, err := testC.Analytics.ProductStats(ctx, res.TenantID, 30); return err },
		func() error { _, err := testC.Analytics.ChannelStats(ctx, res.TenantID, 30); return err },
	} {
		if err := f(); err != nil {
			t.Fatal(err)
		}
	}
	risk, err := testC.Analytics.AtRiskCustomers(ctx, res.TenantID, 50)
	if err != nil || len(risk) == 0 {
		t.Fatalf("at risk: %d %v", len(risk), err)
	}
	series, _ := testC.Analytics.RevenueSeries(ctx, res.TenantID, 7)
	if len(series) != 7 {
		t.Fatalf("series len = %d", len(series))
	}
}

// ---- GraphQL over HTTP

func gqlDo(t *testing.T, h http.Handler, token, query string, vars map[string]any) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"query": query, "variables": vars})
	req := httptest.NewRequest("POST", "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return out
}

func errCode(out map[string]any) string {
	errs, _ := out["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	ext, _ := errs[0].(map[string]any)["extensions"].(map[string]any)
	code, _ := ext["code"].(string)
	return code
}

func TestGraphQLEndToEnd(t *testing.T) {
	h := testC.HTTPHandler()
	email := fmt.Sprintf("gql-%s@provider.test", uuid.NewString()[:8])
	out := gqlDo(t, h, "", `mutation($i: RegisterInput!){ register(input:$i){ accessToken refreshToken user{ id email memberships{ role tenant{ slug } } } } }`,
		map[string]any{"i": map[string]any{"name": "GQL Owner", "email": email, "password": "Passw0rd!", "businessName": "GQL Shop"}})
	if out["errors"] != nil {
		t.Fatalf("register: %v", out["errors"])
	}
	reg := out["data"].(map[string]any)["register"].(map[string]any)
	token := reg["accessToken"].(string)
	ms := reg["user"].(map[string]any)["memberships"].([]any)
	if len(ms) != 1 || ms[0].(map[string]any)["role"] != "OWNER" {
		t.Fatalf("memberships = %v", ms)
	}

	if code := errCode(gqlDo(t, h, "", `{ customers { totalCount } }`, nil)); code != "UNAUTHENTICATED" {
		t.Fatalf("anonymous code = %q", code)
	}
	out = gqlDo(t, h, token, `mutation{ createCustomer(input:{name:"Rahim", phone:"+8801711000001", tags:["VIP"], preferredChannel: WHATSAPP}){ id phone tags preferredChannel status } }`, nil)
	if out["errors"] != nil {
		t.Fatalf("createCustomer: %v", out["errors"])
	}
	cust := out["data"].(map[string]any)["createCustomer"].(map[string]any)
	if cust["phone"] != "01711000001" || cust["preferredChannel"] != "WHATSAPP" || cust["status"] != "ACTIVE" {
		t.Fatalf("customer = %v", cust)
	}
	out = gqlDo(t, h, token, `{ customers(page:{first:10}){ totalCount edges{ cursor node{ name activeSubscriptionCount identities{ id } subscriptions{ id } } } pageInfo{ hasNextPage endCursor } } dashboardStats{ totalCustomers currency } }`, nil)
	if out["errors"] != nil {
		t.Fatalf("customers: %v", out["errors"])
	}
	conn := out["data"].(map[string]any)["customers"].(map[string]any)
	if conn["totalCount"].(float64) != 1 {
		t.Fatalf("conn = %v", conn)
	}
	// Validation errors carry BAD_USER_INPUT; unknown ids return null.
	if code := errCode(gqlDo(t, h, token, `mutation{ createCustomer(input:{name:"  "}){ id } }`, nil)); code != "BAD_USER_INPUT" {
		t.Fatalf("validation code = %q", code)
	}
	out = gqlDo(t, h, token, `query($id: UUID!){ customer(id:$id){ id } }`, map[string]any{"id": uuid.NewString()})
	if out["errors"] != nil || out["data"].(map[string]any)["customer"] != nil {
		t.Fatalf("unknown customer: %v", out)
	}
	// A STAFF token is forbidden from owner/admin mutations.
	staff, _, _ := testC.Tokens.Issue(pauth.Principal{UserID: uuid.New(), TenantID: uuid.New(), Role: "staff"})
	if code := errCode(gqlDo(t, h, staff, `mutation{ deleteCustomer(id:"`+uuid.NewString()+`") }`, nil)); code != "FORBIDDEN" {
		t.Fatalf("staff code = %q", code)
	}
}
