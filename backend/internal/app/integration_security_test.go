//go:build integration

// Sprint 1 security / QA fix regressions (SPRINT-01-SECURITY H1-H6, M1-M9, L3, L5; QA BUG-01..04, 11, 12).
// Every test here runs with the API connected as psaas_app (RLS enforced, see TestMain).
package app_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/app"
	"provider_saas/internal/modules/auth"
	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/social"
	"provider_saas/internal/modules/storefront"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/modules/ticket"
	"provider_saas/internal/platform/apperr"
	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/db"
)

// ---- helpers

func inviteToken(t *testing.T, m tenant.Member) string {
	t.Helper()
	i := strings.LastIndex(m.InviteURL, "/invite/")
	if i < 0 {
		t.Fatalf("invite url = %q", m.InviteURL)
	}
	return m.InviteURL[i+len("/invite/"):]
}

// member invites a brand-new user with role and accepts; returns the session.
func member(t *testing.T, owner tenantFx, role string) auth.Session {
	t.Helper()
	email := fmt.Sprintf("%s-%s@provider.test", role, uuid.NewString()[:8])
	m, err := testC.Tenants.InviteMember(owner.ctx, owner.tid, owner.uid, tenant.InviteInput{Email: &email, Name: "IT " + role, Role: role})
	if err != nil {
		t.Fatal(err)
	}
	s, err := testC.Auth.AcceptInvite(context.Background(), inviteToken(t, m), "Passw0rd!", nil, auth.ClientMeta{})
	if err != nil {
		t.Fatalf("accept %s: %v", role, err)
	}
	if s.Membership == nil || s.Membership.Role != role || s.Membership.Tenant.ID != owner.tid {
		t.Fatalf("session membership = %+v", s.Membership)
	}
	return s
}

func fxOf(s auth.Session) tenantFx {
	p := pauth.Principal{UserID: s.User.ID, TenantID: s.Membership.Tenant.ID, Role: s.Membership.Role}
	return tenantFx{tid: p.TenantID, uid: p.UserID, ctx: pauth.WithPrincipal(context.Background(), p), session: s}
}

func ownerCount(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := testOwner.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func invoiceFor(t *testing.T, f tenantFx, customerID uuid.UUID, total int64) billing.Invoice {
	t.Helper()
	inv, err := testC.Billing.CreateInvoice(f.ctx, f.tid, f.uid, billing.CreateInvoiceInput{CustomerID: customerID,
		Items: []billing.ItemInput{{Description: "Netflix", Quantity: 1, UnitPrice: total}}})
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

// ---- H1: row-level security is fail-closed for the API role

func TestRLSFailClosedForAppRole(t *testing.T) {
	ctx := context.Background()
	a, b := newTenant(t, "RLS A"), newTenant(t, "RLS B")
	mustCustomer(t, a, "A customer")
	cb := mustCustomer(t, b, "B customer")

	var user string
	var super, bypass bool
	if err := testC.Pool.QueryRow(ctx, `SELECT current_user, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`).
		Scan(&user, &super, &bypass); err != nil || super || bypass {
		t.Fatalf("API role %s super=%v bypass=%v err=%v", user, super, bypass, err)
	}
	// No app.tenant_id at all (plain pool query, a forgotten WithTenantTx): no rows.
	for _, table := range []string{"customers", "vault_secrets", "tenant_members", "messages", "payments", "social_accounts", "tenant_counters"} {
		var n int
		if err := testC.Pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s without tenant: %d rows (err=%v)", table, n, err)
		}
	}
	// Platform transaction: tenant tables stay invisible, platform tables are readable.
	_ = db.WithPlatformTx(ctx, testC.Pool, func(ctx context.Context, tx pgx.Tx) error {
		var n, tenants int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM customers`).Scan(&n)
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM tenants WHERE id = $1`, a.tid).Scan(&tenants)
		if n != 0 || tenants != 1 {
			t.Errorf("platform tx: customers=%d tenants=%d", n, tenants)
		}
		return nil
	})
	// Tenant A: only A's rows, even without a tenant_id filter; writing B's tenant_id fails.
	err := db.WithTenantTx(ctx, testC.Pool, a.tid, func(ctx context.Context, tx pgx.Tx) error {
		var all, foreign int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM customers`).Scan(&all)
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM customers WHERE id = $1`, cb.ID).Scan(&foreign)
		if all != 1 || foreign != 0 {
			t.Errorf("tenant A sees %d customers, %d of B's", all, foreign)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.WithTenantTx(ctx, testC.Pool, a.tid, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO customers (tenant_id, name) VALUES ($1, 'planted')`, b.tid)
		return err
	})
	if err == nil {
		t.Fatal("tenant A could insert a row for tenant B")
	}
	// FORCE ROW LEVEL SECURITY on every tenant table; jobs is the only exempt table with tenant_id.
	if n := ownerCount(t, `SELECT count(*) FROM pg_class c JOIN information_schema.columns col ON col.table_name = c.relname
		AND col.column_name = 'tenant_id' AND col.table_schema = 'public'
		WHERE c.relkind = 'r' AND c.relname <> 'jobs' AND NOT (c.relrowsecurity AND c.relforcerowsecurity)`); n != 0 {
		t.Fatalf("%d tenant tables without FORCE ROW LEVEL SECURITY", n)
	}
}

// ---- H2: role matrix + reseller scoping (GraphQL)

func TestRoleMatrixViaGraphQL(t *testing.T) {
	h := testC.HTTPHandler()
	owner := newTenant(t, "Roles Shop")
	viewer, reseller, staff := member(t, owner, "viewer"), member(t, owner, "reseller"), member(t, owner, "staff")
	c := mustCustomer(t, owner, "Owner's customer")
	inv := invoiceFor(t, owner, c.ID, 50000)
	trx := "ROLE" + strings.ToUpper(uniq(""))
	pay, err := testC.Billing.RecordPayment(owner.ctx, owner.tid, owner.uid, &inv.ID, nil, billing.PaymentInput{Method: "bkash", Amount: 20000, TrxID: &trx}, billing.RecordOptions{})
	if err != nil || pay.Status != billing.PaymentPending {
		t.Fatalf("pending payment: %v", err)
	}
	verify := `mutation($id: UUID!){ verifyPayment(id:$id){ id status } }`
	vars := map[string]any{"id": pay.ID.String()}

	// VIEWER: read-only.
	if code := errCode(gqlDo(t, h, viewer.AccessToken, verify, vars)); code != "FORBIDDEN" {
		t.Fatalf("viewer verifyPayment code = %q", code)
	}
	if code := errCode(gqlDo(t, h, viewer.AccessToken, `mutation{ createCustomer(input:{name:"v"}){ id } }`, nil)); code != "FORBIDDEN" {
		t.Fatalf("viewer createCustomer code = %q", code)
	}
	if out := gqlDo(t, h, viewer.AccessToken, `{ customers { totalCount } invoices { totalCount } }`, nil); out["errors"] != nil {
		t.Fatalf("viewer reads: %v", out["errors"])
	}
	// RESELLER: own customers only, no payments, no tenant-wide data.
	if code := errCode(gqlDo(t, h, reseller.AccessToken, verify, vars)); code != "FORBIDDEN" {
		t.Fatalf("reseller verifyPayment code = %q", code)
	}
	if code := errCode(gqlDo(t, h, reseller.AccessToken, `{ invoices { totalCount } }`, nil)); code != "FORBIDDEN" {
		t.Fatalf("reseller invoices code = %q", code)
	}
	out := gqlDo(t, h, reseller.AccessToken, `mutation{ createCustomer(input:{name:"Reseller buyer"}){ id } }`, nil)
	if out["errors"] != nil {
		t.Fatalf("reseller createCustomer: %v", out["errors"])
	}
	out = gqlDo(t, h, reseller.AccessToken, `query($id: UUID!){ customers { totalCount edges { node { name } } } customer(id:$id){ id } }`,
		map[string]any{"id": c.ID.String()})
	data := out["data"].(map[string]any)
	if conn := data["customers"].(map[string]any); conn["totalCount"].(float64) != 1 || data["customer"] != nil {
		t.Fatalf("reseller sees %v / owner's customer %v", conn, data["customer"])
	}
	p, plan, _ := sharedProduct(t, owner, 2)
	rf := fxOf(reseller)
	if _, err := testC.Subscriptions.Create(rf.ctx, rf.tid, rf.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID}); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("reseller sold to a customer it does not own: %v", err)
	}
	markPaid := `mutation($c: UUID!, $p: UUID!){ createSubscription(input:{customerId:$c, productId:$p, markPaid:{method:CASH, amount:100}}){ id } }`
	if code := errCode(gqlDo(t, h, reseller.AccessToken, markPaid, map[string]any{"c": c.ID.String(), "p": p.ID.String()})); code != "FORBIDDEN" {
		t.Fatalf("reseller markPaid code = %q", code)
	}
	// STAFF: may verify while settings.staff_can_verify_payments is not false...
	off := map[string]any{"staff_can_verify_payments": false}
	if out := gqlDo(t, h, staff.AccessToken, verify, vars); out["errors"] != nil {
		t.Fatalf("staff verify: %v", out["errors"])
	}
	// ...cannot reverse a verified payment...
	if code := errCode(gqlDo(t, h, staff.AccessToken, `mutation($id: UUID!){ rejectPayment(id:$id){ id } }`, vars)); code != "FORBIDDEN" {
		t.Fatalf("staff reverse code = %q", code)
	}
	// ...and loses verification when the owner switches it off.
	if _, err := testC.Tenants.Update(owner.ctx, owner.tid, owner.uid, tenant.UpdateInput{Settings: off}); err != nil {
		t.Fatal(err)
	}
	trx2 := "ROLE" + strings.ToUpper(uniq(""))
	pay2, _ := testC.Billing.RecordPayment(owner.ctx, owner.tid, owner.uid, &inv.ID, nil, billing.PaymentInput{Method: "bkash", Amount: 1000, TrxID: &trx2}, billing.RecordOptions{})
	if code := errCode(gqlDo(t, h, staff.AccessToken, verify, map[string]any{"id": pay2.ID.String()})); code != "FORBIDDEN" {
		t.Fatalf("staff verify with setting off code = %q", code)
	}
	// Slot PINs are masked for everyone but owner/admin (L8).
	_, _, m := sharedProduct(t, owner, 1)
	q := `query($id: UUID!){ masterAccount(id:$id){ slots { id pin } } }`
	slots := gqlDo(t, h, owner.session.AccessToken, q, map[string]any{"id": m.ID.String()})["data"].(map[string]any)["masterAccount"].(map[string]any)["slots"].([]any)
	slotID := slots[0].(map[string]any)["id"].(string)
	if out := gqlDo(t, h, owner.session.AccessToken, `mutation($id: UUID!){ updateSlot(id:$id, pin:"4321"){ pin } }`, map[string]any{"id": slotID}); out["errors"] != nil {
		t.Fatalf("updateSlot: %v", out["errors"])
	}
	staffSlots := gqlDo(t, h, staff.AccessToken, q, map[string]any{"id": m.ID.String()})["data"].(map[string]any)["masterAccount"].(map[string]any)["slots"].([]any)
	if pin := staffSlots[0].(map[string]any)["pin"]; pin != "••••" {
		t.Fatalf("staff sees pin %v", pin)
	}
	// Members never expose another user's platform-admin flag; invited rows show status.
	out = gqlDo(t, h, owner.session.AccessToken, `{ tenant { members { status role user { name isPlatformAdmin } } } }`, nil)
	if out["errors"] != nil || len(out["data"].(map[string]any)["tenant"].(map[string]any)["members"].([]any)) != 4 {
		t.Fatalf("members: %v", out)
	}
}

// ---- H4: SMS auto-verification needs a trusted sender

func TestSMSSenderAllowlist(t *testing.T) {
	f := newTenant(t, "SMS Shop")
	c := mustCustomer(t, f, "SMS Buyer")
	inv := invoiceFor(t, f, c.ID, 90000)
	sms := func(trx string, tk int) string {
		return fmt.Sprintf("You have received Tk %d.00 from 01712345678. Fee Tk 0.00. Balance Tk 9,250.00. TrxID %s at 29/10/2026 10:15", tk, trx)
	}
	pending := func(trx string, amount int64) billing.Payment {
		p, err := testC.Billing.RecordPayment(f.ctx, f.tid, f.uid, &inv.ID, nil, billing.PaymentInput{Method: "bkash", Amount: amount, TrxID: &trx}, billing.RecordOptions{})
		if err != nil || p.Status != billing.PaymentPending {
			t.Fatalf("pending: %+v %v", p, err)
		}
		return p
	}
	// Forged SMS from a personal number: stored for review, payment stays pending.
	trx1 := "SP" + strings.ToUpper(uniq(""))
	p1 := pending(trx1, 10000)
	item, err := testC.Billing.IngestSMS(f.ctx, f.tid, "01712345678", sms(trx1, 100), time.Now())
	if err != nil || item.Status != "needs_review" || item.Sender == nil || *item.Sender != "01712345678" {
		t.Fatalf("forged sms: %+v %v", item, err)
	}
	if p, _ := testC.Billing.Payment(f.ctx, f.tid, p1.ID); p.Status != billing.PaymentPending {
		t.Fatalf("forged SMS verified the payment: %s", p.Status)
	}
	// A later payment with that TrxID must not auto-match the needs_review SMS either.
	trx2 := "SP" + strings.ToUpper(uniq(""))
	if _, err := testC.Billing.IngestSMS(f.ctx, f.tid, "+8801812345678", sms(trx2, 100), time.Now()); err != nil {
		t.Fatal(err)
	}
	if p := pending(trx2, 10000); p.Status != billing.PaymentPending {
		t.Fatal("needs_review SMS auto-matched a new payment")
	}
	// The official short code / sender id verifies.
	for _, from := range []string{"bKash", "16247"} {
		trx := "SP" + strings.ToUpper(uniq(""))
		p := pending(trx, 10000)
		item, err := testC.Billing.IngestSMS(f.ctx, f.tid, from, sms(trx, 100), time.Now())
		if err != nil || item.Status != "matched" {
			t.Fatalf("sms from %s: %+v %v", from, item, err)
		}
		if got, _ := testC.Billing.Payment(f.ctx, f.tid, p.ID); got.Status != billing.PaymentVerified || got.VerificationSource == nil || *got.VerificationSource != billing.SourceSMSMatch {
			t.Fatalf("payment after %s sms = %+v", from, got)
		}
	}
	// Tenant override (settings.payment_sms_senders) replaces the default list.
	if _, err := testC.Tenants.Update(f.ctx, f.tid, f.uid, tenant.UpdateInput{Settings: map[string]any{
		"payment_sms_senders": map[string]any{"bkash": []any{"MyGateway"}}}}); err != nil {
		t.Fatal(err)
	}
	trx3 := "SP" + strings.ToUpper(uniq(""))
	pending(trx3, 10000)
	if item, _ := testC.Billing.IngestSMS(f.ctx, f.tid, "bKash", sms(trx3, 100), time.Now()); item.Status != "needs_review" {
		t.Fatalf("override ignored: %s", item.Status)
	}
}

// ---- H5: a page / number / bot cannot be connected by two businesses

func TestSocialAccountCannotBeHijacked(t *testing.T) {
	victim, attacker := newTenant(t, "Victim Shop"), newTenant(t, "Attacker Shop")
	botToken := fmt.Sprintf("%d:AAItHijack_%s", 600000000+time.Now().UnixNano()%100000000, uniq("x"))
	bot, err := testC.Social.ConnectTelegram(victim.ctx, victim.tid, victim.uid, botToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Social.ConnectTelegram(attacker.ctx, attacker.tid, attacker.uid, botToken); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("second business connected the same bot: %v", err)
	}
	if a, err := testC.Social.ResolveInbound(context.Background(), social.PlatformTelegram, bot.ExternalID); err != nil || a.TenantID != victim.tid {
		t.Fatalf("routing = %+v %v", a, err)
	}
	// WhatsApp: the number must belong to the token's WABA, and only one live connection.
	fake := testC.SocialClient.(*social.Fake)
	pn := uniq("pn")
	fake.AddWABANumber("waba-victim", pn)
	tok := "EAAG-it-whatsapp-access-token-123456"
	if _, err := testC.Social.ConnectWhatsApp(attacker.ctx, attacker.tid, attacker.uid, social.ConnectWhatsAppInput{PhoneNumberID: pn,
		BusinessAccountID: "waba-attacker", AccessToken: tok, DisplayName: "x"}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("number of another WABA accepted: %v", err)
	}
	if _, err := testC.Social.ConnectWhatsApp(victim.ctx, victim.tid, victim.uid, social.ConnectWhatsAppInput{PhoneNumberID: pn,
		BusinessAccountID: "waba-victim", AccessToken: tok, DisplayName: "Victim WA"}); err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Social.ConnectWhatsApp(attacker.ctx, attacker.tid, attacker.uid, social.ConnectWhatsAppInput{PhoneNumberID: pn,
		BusinessAccountID: "waba-victim", AccessToken: tok, DisplayName: "x"}); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("live number connected twice: %v", err)
	}
	// Once the victim disconnects, the bot can be connected elsewhere (rows are kept).
	if err := testC.Social.Disconnect(victim.ctx, victim.tid, bot.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Social.ConnectTelegram(attacker.ctx, attacker.tid, attacker.uid, botToken); err != nil {
		t.Fatalf("connect after disconnect: %v", err)
	}
}

// ---- M2: OAuth state is single-use and bound to the user who started it

func TestOAuthStateSingleUseAndBound(t *testing.T) {
	owner := newTenant(t, "OAuth Shop")
	admin := fxOf(member(t, owner, "admin"))
	u, err := testC.Social.StartMetaConnect(owner.ctx, owner.tid, owner.uid, "facebook")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Social.CompleteMetaConnect(admin.ctx, admin.tid, admin.uid, "code-"+uniq(""), u.State); !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("other user completed the flow: %v", err)
	}
	accts, err := testC.Social.CompleteMetaConnect(owner.ctx, owner.tid, owner.uid, "code-"+uniq(""), u.State)
	if err != nil || len(accts) != 2 {
		t.Fatalf("complete: %d %v", len(accts), err)
	}
	if _, err := testC.Social.CompleteMetaConnect(owner.ctx, owner.tid, owner.uid, "code-"+uniq(""), u.State); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("state replay: %v", err)
	}
	// The unauthenticated callback only forwards to the SPA, it never connects.
	h := testC.HTTPHandler()
	u2, _ := testC.Social.StartMetaConnect(owner.ctx, owner.tid, owner.uid, "facebook")
	req := httptestRequest("GET", "/webhooks/meta/oauth?code=abc&state="+u2.State)
	rec := serve(h, req)
	if rec.Code != 302 || !strings.Contains(rec.Header().Get("Location"), "/social?code=abc&state=") {
		t.Fatalf("callback = %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if loc := serve(h, httptestRequest("GET", "/webhooks/meta/oauth?code=abc&state=forged")).Header().Get("Location"); !strings.HasSuffix(loc, "/social?error=invalid_state") {
		t.Fatalf("forged state redirect = %s", loc)
	}
}

// ---- H6 / M1 / M9: login lockout, audit and alias amplification

func TestLoginLockoutAndAudit(t *testing.T) {
	f := newTenant(t, "Lockout Shop")
	email := *f.session.User.Email
	meta := auth.ClientMeta{IP: "198.51.100.23"}
	for i := 1; i <= 5; i++ {
		if _, err := testC.Auth.Login(context.Background(), auth.LoginInput{Identifier: email, Password: "wrong-pass1"}, meta); !errors.Is(err, apperr.ErrUnauthenticated) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if _, err := testC.Auth.Login(context.Background(), auth.LoginInput{Identifier: email, Password: "Passw0rd!"}, meta); !errors.Is(err, apperr.ErrRateLimited) {
		t.Fatalf("locked account accepted the right password: %v", err)
	}
	if n := ownerCount(t, `SELECT count(*) FROM audit_logs WHERE tenant_id IS NULL AND action = 'auth.login_failed' AND user_id = $1`, f.uid); n != 5 {
		t.Fatalf("failed-login audit rows = %d", n)
	}
	if n := ownerCount(t, `SELECT count(*) FROM audit_logs WHERE tenant_id IS NULL AND action = 'auth.login_locked' AND user_id = $1`, f.uid); n != 1 {
		t.Fatalf("lockout audit rows = %d", n)
	}
	// The API role can write but never read platform audit rows.
	var visible int
	_ = testC.Pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE user_id = $1`, f.uid).Scan(&visible)
	if visible != 0 {
		t.Fatalf("platform audit rows readable by the API role: %d", visible)
	}
	// Unknown identifiers lock the same way (no account enumeration through lockout).
	ghost := "ghost-" + uniq("") + "@provider.test"
	for i := 0; i < 5; i++ {
		_, _ = testC.Auth.Login(context.Background(), auth.LoginInput{Identifier: ghost, Password: "x1234567"}, auth.ClientMeta{})
	}
	if _, err := testC.Auth.Login(context.Background(), auth.LoginInput{Identifier: ghost, Password: "x1234567"}, auth.ClientMeta{}); !errors.Is(err, apperr.ErrRateLimited) {
		t.Fatalf("unknown identifier not locked: %v", err)
	}
	// One HTTP request cannot carry several login attempts.
	out := gqlDo(t, testC.HTTPHandler(), "", `mutation { a: login(input:{identifier:"x@y.z", password:"a"}){ accessToken } b: login(input:{identifier:"x@y.z", password:"b"}){ accessToken } }`, nil)
	if code := errCode(out); code != "OPERATION_LIMIT" || out["data"] != nil {
		t.Fatalf("aliased logins: %v", out)
	}
}

// ---- BUG-02 / BUG-03 / M4: invitations

func TestInviteFlow(t *testing.T) {
	ctx := context.Background()
	owner := newTenant(t, "Invite Shop")
	email := "new-" + uniq("") + "@provider.test"
	m, err := testC.Tenants.InviteMember(owner.ctx, owner.tid, owner.uid, tenant.InviteInput{Email: &email, Name: "Nusrat", Role: "staff"})
	if err != nil || m.Status != tenant.MemberInvited || !strings.HasPrefix(m.InviteURL, testC.Cfg.PublicAppURL+"/invite/") || m.InviteExpiresAt == nil {
		t.Fatalf("invite = %+v %v", m, err)
	}
	token := inviteToken(t, m)
	if n := ownerCount(t, `SELECT count(*) FROM tenant_members WHERE invite_token_hash = $1`, token); n != 0 {
		t.Fatal("plaintext invite token stored")
	}
	// Not usable before acceptance.
	if _, err := testC.Auth.Login(ctx, auth.LoginInput{Identifier: email, Password: "Passw0rd!"}, auth.ClientMeta{}); !errors.Is(err, apperr.ErrUnauthenticated) {
		t.Fatalf("login before accept: %v", err)
	}
	prev, err := testC.Auth.Invite(ctx, token)
	if err != nil || prev.TenantName != "Invite Shop" || prev.Role != "staff" || prev.UserExists || prev.EmailHint == nil || strings.Contains(*prev.EmailHint, email) {
		t.Fatalf("preview = %+v %v", prev, err)
	}
	if _, err := testC.Auth.AcceptInvite(ctx, token, "short", nil, auth.ClientMeta{}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("weak password: %v", err)
	}
	name := "Nusrat Jahan"
	s, err := testC.Auth.AcceptInvite(ctx, token, "Passw0rd!", &name, auth.ClientMeta{})
	if err != nil || s.Membership == nil || s.Membership.Tenant.ID != owner.tid || s.Membership.Role != "staff" || s.User.Name != name {
		t.Fatalf("accept: %+v %v", s.Membership, err)
	}
	if _, err := testC.Auth.AcceptInvite(ctx, token, "Passw0rd!", nil, auth.ClientMeta{}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("token reuse: %v", err)
	}
	if s2, err := testC.Auth.Login(ctx, auth.LoginInput{Identifier: email, Password: "Passw0rd!"}, auth.ClientMeta{}); err != nil || s2.Membership.Tenant.ID != owner.tid {
		t.Fatalf("login after accept: %v", err)
	}

	// Existing user: not attached, profile not leaked, must accept with their own password.
	other := newTenant(t, "Other Owner Shop")
	otherEmail := *other.session.User.Email
	m2, err := testC.Tenants.InviteMember(owner.ctx, owner.tid, owner.uid, tenant.InviteInput{Email: &otherEmail, Name: "Typed Name", Role: "viewer"})
	if err != nil {
		t.Fatal(err)
	}
	if m2.User.Name != "Typed Name" || m2.User.Email == nil || *m2.User.Email == otherEmail {
		t.Fatalf("invite response leaks the existing profile: %+v", m2.User)
	}
	if ms, _ := testC.Auth.Memberships(ctx, other.uid); len(ms) != 1 {
		t.Fatalf("pending invite listed as membership: %d", len(ms))
	}
	if _, err := testC.Auth.SwitchTenant(ctx, other.uid, owner.tid, auth.ClientMeta{}); !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("switch into an unaccepted invite: %v", err)
	}
	// Resend rotates the token: the old link stops working.
	m3, err := testC.Tenants.ResendInvite(owner.ctx, owner.tid, owner.uid, m2.ID)
	if err != nil || m3.InviteURL == m2.InviteURL {
		t.Fatalf("resend: %v", err)
	}
	if _, err := testC.Auth.Invite(ctx, inviteToken(t, m2)); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("old token after resend: %v", err)
	}
	if _, err := testC.Auth.AcceptInvite(ctx, inviteToken(t, m3), "wrong-pass1", nil, auth.ClientMeta{}); !errors.Is(err, apperr.ErrUnauthenticated) {
		t.Fatalf("existing user, wrong password: %v", err)
	}
	if _, err := testC.Auth.AcceptInvite(ctx, inviteToken(t, m3), "Passw0rd!", nil, auth.ClientMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Auth.SwitchTenant(ctx, other.uid, owner.tid, auth.ClientMeta{}); err != nil {
		t.Fatalf("switch after accept: %v", err)
	}
	// Expired links are rejected.
	email3 := "late-" + uniq("") + "@provider.test"
	m4, _ := testC.Tenants.InviteMember(owner.ctx, owner.tid, owner.uid, tenant.InviteInput{Email: &email3, Name: "Late", Role: "staff"})
	_, _ = testOwner.Exec(ctx, `UPDATE tenant_members SET invite_expires_at = now() - interval '1 minute' WHERE id = $1`, m4.ID)
	if _, err := testC.Auth.AcceptInvite(ctx, inviteToken(t, m4), "Passw0rd!", nil, auth.ClientMeta{}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expired invite: %v", err)
	}
	members, _ := testC.Tenants.Members(owner.ctx, owner.tid)
	if len(members) != 4 || members[3].Status != tenant.MemberInvited {
		t.Fatalf("members = %+v", members)
	}
}

// ---- BUG-04: overpayment

func TestOverpaymentRejectedAndReverseVerified(t *testing.T) {
	f := newTenant(t, "Overpay Shop")
	c := mustCustomer(t, f, "Overpayer")
	inv := invoiceFor(t, f, c.ID, 135048)
	rec := func(method string, amount int64) (billing.Payment, error) {
		trx := "OP" + strings.ToUpper(uniq(""))
		var tp *string
		if billing.IsMobileMoney(method) {
			tp = &trx
		}
		return testC.Billing.RecordPayment(f.ctx, f.tid, f.uid, &inv.ID, nil, billing.PaymentInput{Method: method, Amount: amount, TrxID: tp}, billing.RecordOptions{})
	}
	p1, _ := rec("bkash", 50000)
	p2, err := rec("nagad", 135048)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rec("cash", 500); err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Billing.VerifyPayment(f.ctx, f.tid, f.uid, p1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Billing.VerifyPayment(f.ctx, f.tid, f.uid, p2.ID); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("overpaying verify: %v", err)
	}
	if _, err := rec("cash", 200000); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("overpaying record: %v", err)
	}
	got, _ := testC.Billing.Invoice(f.ctx, f.tid, inv.ID)
	if got.PaidAmount != 50500 {
		t.Fatalf("paid = %d", got.PaidAmount)
	}
	// Reversing a verified payment restores invoice, ledger and due.
	if _, err := testC.Billing.RejectPayment(f.ctx, f.tid, f.uid, p1.ID, strp("spoofed sms")); err != nil {
		t.Fatal(err)
	}
	got, _ = testC.Billing.Invoice(f.ctx, f.tid, inv.ID)
	cu, _ := testC.Customers.Get(f.ctx, f.tid, c.ID)
	if got.PaidAmount != 500 || cu.DueBalance != 135048-500 || cu.LifetimeValue != 500 {
		t.Fatalf("after reversal: paid=%d due=%d ltv=%d", got.PaidAmount, cu.DueBalance, cu.LifetimeValue)
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
		t.Fatalf("ledger %d != due %d", sum, cu.DueBalance)
	}
	// The tenant switch allows explicit overpayments (advance).
	if _, err := testC.Tenants.Update(f.ctx, f.tid, f.uid, tenant.UpdateInput{Settings: map[string]any{"allow_overpayment": true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Billing.VerifyPayment(f.ctx, f.tid, f.uid, p2.ID); err != nil {
		t.Fatalf("allowed overpayment: %v", err)
	}
	for _, a := range []string{"payment.verify", "payment.reject"} {
		if n := count(t, f.tid, `SELECT count(*) FROM audit_logs WHERE tenant_id = $1 AND action = $2`, a); n < 1 {
			t.Fatalf("no %s audit row", a)
		}
	}
}

// ---- BUG-11 / L6: per-tenant numbering

func TestPerTenantNumbersAreContiguousUnderConcurrency(t *testing.T) {
	f := newTenant(t, "Numbers Shop")
	c := mustCustomer(t, f, "Numbers Buyer")
	first := invoiceFor(t, f, c.ID, 1000)
	want := "INV-" + time.Now().In(billing.Dhaka).Format("200601") + "-0001"
	if first.Number != want {
		t.Fatalf("first invoice = %s, want %s", first.Number, want)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inv, err := testC.Billing.CreateInvoice(f.ctx, f.tid, f.uid, billing.CreateInvoiceInput{CustomerID: c.ID,
				Items: []billing.ItemInput{{Description: "x", Quantity: 1, UnitPrice: 100}}})
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			seen[inv.Number] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	for i := 2; i <= 11; i++ {
		if n := fmt.Sprintf("%s%04d", want[:len(want)-4], i); !seen[n] {
			t.Fatalf("missing %s in %v", n, seen)
		}
	}
	tk, err := testC.Tickets.Create(f.ctx, f.tid, f.uid, ticket.CreateInput{CustomerID: c.ID, Kind: "question", Priority: "normal", Title: "Q"})
	if err != nil || tk.Number != 1 {
		t.Fatalf("first ticket = %d %v", tk.Number, err)
	}
	// Another tenant starts at 1 as well.
	g := newTenant(t, "Numbers Shop 2")
	if inv := invoiceFor(t, g, mustCustomer(t, g, "x").ID, 100); inv.Number != want {
		t.Fatalf("second tenant first invoice = %s", inv.Number)
	}
}

// ---- BUG-12: out of stock

func TestPlaceOrderRejectsOutOfStock(t *testing.T) {
	f := newTenant(t, "Stock Shop")
	p, plan, _ := sharedProduct(t, f, 1)
	sf := publishedStore(t, f)
	c := mustCustomer(t, f, "Slot taker")
	if _, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID}); err != nil {
		t.Fatal(err)
	}
	_, err := testC.Storefront.PlaceOrder(context.Background(), "it-"+uniq(""), storefront.PlaceOrderInput{Slug: sf.Slug, ProductID: p.ID,
		PlanID: plan.ID, CustomerName: "Late Buyer", CustomerPhone: "01711000999"})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("out-of-stock order: %v", err)
	}
}

// ---- L3: SMS token rotation

func TestRotateSMSWebhookToken(t *testing.T) {
	f := newTenant(t, "Rotate Shop")
	t1, err := testC.Tenants.RotateSMSWebhookToken(f.ctx, f.tid, f.uid)
	if err != nil {
		t.Fatal(err)
	}
	if tid, err := testC.Tenants.VerifySMSWebhookToken(context.Background(), t1); err != nil || tid != f.tid {
		t.Fatalf("t1: %v", err)
	}
	t2, err := testC.Tenants.RotateSMSWebhookToken(f.ctx, f.tid, f.uid)
	if err != nil || t2 == t1 {
		t.Fatal(err)
	}
	if _, err := testC.Tenants.VerifySMSWebhookToken(context.Background(), t1); err == nil {
		t.Fatal("old token still valid after rotation")
	}
	if _, err := testC.Tenants.VerifySMSWebhookToken(context.Background(), t2); err != nil {
		t.Fatal(err)
	}
	if n := count(t, f.tid, `SELECT count(*) FROM audit_logs WHERE tenant_id = $1 AND action = 'sms_token.rotate'`); n != 2 {
		t.Fatalf("rotate audit rows = %d", n)
	}
	tn, _ := testC.Tenants.Get(f.ctx, f.tid)
	if tn.Settings["sms_webhook_token_hint"] != tenant.SMSTokenHint(t2) {
		t.Fatalf("hint = %v", tn.Settings["sms_webhook_token_hint"])
	}
}

// ---- BUG-01: vault key self-check

func TestVaultSelfCheckDetectsWrongKey(t *testing.T) {
	if _, err := testC.Seed(context.Background()); err != nil { // pins the fingerprint on the demo tenant
		t.Fatal(err)
	}
	if err := testC.Vault.SelfCheck(context.Background()); err != nil {
		t.Fatalf("self-check with the seed key: %v", err)
	}
	cfg := testC.Cfg
	cfg.VaultKey = []byte(strings.Repeat("z", 32))
	c2, err := app.NewWithPool(cfg, testC.Log, testC.Pool)
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.Vault.SelfCheck(context.Background()); err == nil {
		t.Fatal("self-check accepted a different VAULT_KEY")
	}
}

// ---- M9: money and fulfilment actions are audited

func TestFulfilmentActionsAreAudited(t *testing.T) {
	f := newTenant(t, "Audit Shop")
	p, plan, _ := sharedProduct(t, f, 2)
	c := mustCustomer(t, f, "Audited")
	sub, err := testC.Subscriptions.Create(f.ctx, f.tid, f.uid, subscription.CreateInput{CustomerID: c.ID, ProductID: p.ID, PlanID: &plan.ID, CreateInvoice: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Subscriptions.MarkDelivered(f.ctx, f.tid, sub.ID, strp("manual")); err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Subscriptions.Extend(f.ctx, f.tid, sub.ID, 3, strp("goodwill")); err != nil {
		t.Fatal(err)
	}
	if _, err := testC.Billing.AddLedgerAdjustment(f.ctx, f.tid, f.uid, c.ID, billing.Debit, 500, strp("old due")); err != nil {
		t.Fatal(err)
	}
	inv := invoiceFor(t, f, c.ID, 700)
	if _, err := testC.Billing.VoidInvoice(f.ctx, f.tid, f.uid, inv.ID, strp("mistake")); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"subscription.delivered", "subscription.extend", "ledger.adjust", "invoice.void"} {
		if n := count(t, f.tid, `SELECT count(*) FROM audit_logs WHERE tenant_id = $1 AND action = $2 AND user_id IS NOT NULL`, a); n != 1 {
			t.Fatalf("%s audit rows = %d", a, n)
		}
	}
}

func httptestRequest(method, path string) *http.Request {
	return httptest.NewRequest(method, path, nil)
}

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}
