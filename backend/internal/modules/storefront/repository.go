package storefront

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/db"
)

type Repository struct{}

const sfCols = `id, tenant_id, slug::text, display_name, tagline, logo_url, cover_url, theme, whatsapp, messenger_url, telegram,
	payment_instructions, payment_numbers, is_published`

func scanSF(r pgx.Row) (Storefront, error) {
	var s Storefront
	var pn []byte
	err := r.Scan(&s.ID, &s.TenantID, &s.Slug, &s.DisplayName, &s.Tagline, &s.LogoURL, &s.CoverURL, &s.Theme, &s.Whatsapp,
		&s.MessengerURL, &s.Telegram, &s.PaymentInstructions, &pn, &s.IsPublished)
	s.PaymentNumbers = json.RawMessage(pn)
	if s.Theme == nil {
		s.Theme = map[string]any{}
	}
	return s, err
}

func (Repository) ByTenant(ctx context.Context, q db.Querier, tenantID uuid.UUID) (Storefront, error) {
	return scanSF(q.QueryRow(ctx, `SELECT `+sfCols+` FROM storefronts WHERE tenant_id = $1`, tenantID))
}

// BySlug is platform-level (public pages): no tenant scope.
func (Repository) BySlug(ctx context.Context, q db.Querier, slug string) (Storefront, error) {
	s, err := scanSF(q.QueryRow(ctx, `SELECT `+sfCols+` FROM storefronts WHERE slug = $1`, slug))
	return s, db.MapErr(err, "storefront")
}

func (Repository) Insert(ctx context.Context, q db.Querier, tenantID uuid.UUID, slug, name string) (Storefront, error) {
	s, err := scanSF(q.QueryRow(ctx, `INSERT INTO storefronts (tenant_id, slug, display_name) VALUES ($1,$2,$3) RETURNING `+sfCols,
		tenantID, slug, name))
	return s, err
}

func (Repository) Update(ctx context.Context, q db.Querier, tenantID uuid.UUID, in Input, paymentNumbers []byte) (Storefront, error) {
	var theme []byte
	if in.Theme != nil {
		theme, _ = json.Marshal(in.Theme)
	}
	return scanSF(q.QueryRow(ctx, `UPDATE storefronts SET slug = COALESCE($2, slug), display_name = COALESCE($3, display_name),
		tagline = COALESCE($4, tagline), logo_url = COALESCE($5, logo_url), cover_url = COALESCE($6, cover_url),
		theme = COALESCE($7::jsonb, theme), whatsapp = COALESCE($8, whatsapp), messenger_url = COALESCE($9, messenger_url),
		telegram = COALESCE($10, telegram), payment_instructions = COALESCE($11, payment_instructions),
		payment_numbers = COALESCE($12::jsonb, payment_numbers), is_published = COALESCE($13, is_published)
		WHERE tenant_id = $1 RETURNING `+sfCols, tenantID, in.Slug, in.DisplayName, in.Tagline, in.LogoURL, in.CoverURL, theme,
		in.Whatsapp, in.MessengerURL, in.Telegram, in.PaymentInstructions, paymentNumbers, in.IsPublished))
}

// ---- orders

// ownerScope restricts a RESELLER caller to orders they created (ADR-007).
func ownerScope(ctx context.Context, args *[]any) string {
	if uid, ok := pauth.ResellerScope(ctx); ok {
		*args = append(*args, uid)
		return fmt.Sprintf(" AND o.created_by = $%d", len(*args))
	}
	return ""
}

const orderCols = `o.id, o.tenant_id, o.number, o.customer_id, o.conversation_id, o.product_id, o.plan_id, o.quantity, o.amount, o.currency,
	o.source, o.status, o.payment_id, o.subscription_id, o.customer_name, o.customer_phone, o.customer_note, o.created_by, o.created_at`

func scanOrder(r pgx.Row) (Order, error) {
	var o Order
	err := r.Scan(&o.ID, &o.TenantID, &o.Number, &o.CustomerID, &o.ConversationID, &o.ProductID, &o.PlanID, &o.Quantity, &o.Amount,
		&o.Currency, &o.Source, &o.Status, &o.PaymentID, &o.SubscriptionID, &o.CustomerName, &o.CustomerPhone, &o.CustomerNote,
		&o.CreatedBy, &o.CreatedAt)
	return o, err
}

func collectOrders(rows pgx.Rows, err error) ([]Order, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Order, error) { return scanOrder(r) })
}

// InsertOrder numbers the order per tenant (db.NextNumber; QA BUG-11, SECURITY L6).
func (Repository) InsertOrder(ctx context.Context, q db.Querier, tenantID uuid.UUID, o Order) (Order, error) {
	n, err := db.NextNumber(ctx, q, tenantID, "order")
	if err != nil {
		return Order{}, err
	}
	out, err := scanOrder(q.QueryRow(ctx, `INSERT INTO orders AS o (tenant_id, number, customer_id, conversation_id, product_id, plan_id,
		quantity, amount, currency, source, status, payment_id, customer_name, customer_phone, customer_note, created_by)
		VALUES ($1, $16, $2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING `+orderCols,
		tenantID, o.CustomerID, o.ConversationID, o.ProductID, o.PlanID, o.Quantity, o.Amount, o.Currency, o.Source, o.Status,
		o.PaymentID, o.CustomerName, o.CustomerPhone, o.CustomerNote, o.CreatedBy, n))
	return out, db.MapErr(err, "order")
}

func (Repository) GetOrder(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Order, error) {
	args := []any{tenantID, id}
	sql := `SELECT ` + orderCols + ` FROM orders o WHERE o.tenant_id = $1 AND o.id = $2` + ownerScope(ctx, &args)
	if lock {
		sql += ` FOR UPDATE`
	}
	o, err := scanOrder(q.QueryRow(ctx, sql, args...))
	return o, db.MapErr(err, "order")
}

func (Repository) OrderByNumber(ctx context.Context, q db.Querier, tenantID uuid.UUID, number int) (Order, error) {
	o, err := scanOrder(q.QueryRow(ctx, `SELECT `+orderCols+` FROM orders o WHERE o.tenant_id = $1 AND o.number = $2`, tenantID, number))
	return o, db.MapErr(err, "order")
}

func (Repository) UpdateOrder(ctx context.Context, q db.Querier, tenantID uuid.UUID, o Order) (Order, error) {
	out, err := scanOrder(q.QueryRow(ctx, `UPDATE orders o SET status = $3, payment_id = $4, subscription_id = $5, customer_id = $6,
		customer_note = $7 WHERE o.tenant_id = $1 AND o.id = $2 RETURNING `+orderCols,
		tenantID, o.ID, o.Status, o.PaymentID, o.SubscriptionID, o.CustomerID, o.CustomerNote))
	return out, db.MapErr(err, "order")
}

// MarkDeliveredBySubscription flips paid orders fulfilled by a subscription to delivered.
func (Repository) MarkDeliveredBySubscription(ctx context.Context, q db.Querier, tenantID, subscriptionID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE orders SET status = 'delivered' WHERE tenant_id = $1 AND subscription_id = $2 AND status = 'paid'`,
		tenantID, subscriptionID)
	return err
}

func (Repository) ListOrders(ctx context.Context, q db.Querier, tenantID uuid.UUID, f OrderFilter, after *uuid.UUID, limit int) ([]Order, error) {
	args := []any{tenantID}
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	conds := []string{"o.tenant_id = $1"}
	if f.Status != nil {
		conds = append(conds, "o.status = "+add(*f.Status))
	}
	if f.Source != nil {
		conds = append(conds, "o.source = "+add(*f.Source))
	}
	if f.Search != nil && strings.TrimSpace(*f.Search) != "" {
		term := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(*f.Search), "#"))
		p := add(db.Contains(term))
		conds = append(conds, fmt.Sprintf(`(o.number::text = %s OR o.customer_name ILIKE %s OR o.customer_phone ILIKE %s
			OR EXISTS (SELECT 1 FROM customers c WHERE c.tenant_id = o.tenant_id AND c.id = o.customer_id AND c.name ILIKE %s))`,
			add(term), p, p, p))
	}
	if after != nil {
		conds = append(conds, "(o.created_at, o.id) < (SELECT created_at, id FROM orders WHERE tenant_id = $1 AND id = "+add(*after)+")")
	}
	if uid, ok := pauth.ResellerScope(ctx); ok { // ADR-007: resellers see only orders they created
		conds = append(conds, "o.created_by = "+add(uid))
	}
	return collectOrders(q.Query(ctx, `SELECT `+orderCols+` FROM orders o WHERE `+strings.Join(conds, " AND ")+
		` ORDER BY o.created_at DESC, o.id DESC LIMIT `+add(limit), args...))
}

// ---- portal tokens

func (Repository) InsertPortalToken(ctx context.Context, q db.Querier, tenantID, customerID uuid.UUID, hash string, expires time.Time) error {
	_, err := q.Exec(ctx, `INSERT INTO portal_tokens (tenant_id, customer_id, token_hash, expires_at) VALUES ($1,$2,$3,$4)`,
		tenantID, customerID, hash, expires)
	return err
}

// PortalToken resolves a token hash (platform level) to tenant + customer when not expired,
// stamping used_at on first use.
func (Repository) PortalToken(ctx context.Context, q db.Querier, hash string, now time.Time) (tenantID, customerID uuid.UUID, err error) {
	err = q.QueryRow(ctx, `UPDATE portal_tokens SET used_at = COALESCE(used_at, $2) WHERE token_hash = $1 AND expires_at > $2
		RETURNING tenant_id, customer_id`, hash, now).Scan(&tenantID, &customerID)
	return tenantID, customerID, db.MapErr(err, "portal link")
}
