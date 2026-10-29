package reminder

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
	"provider_saas/internal/platform/sqlb"
)

// Repository owns reminder_rules and reminders. The planner/dispatcher also READ
// subscriptions, invoices, master accounts, customers and products directly (read-only
// materialisation queries, like analytics) instead of N service round trips per tenant.
type Repository struct{}

const ruleCols = `id, name, trigger, offset_days, send_hour, channel, template_id, product_id, is_active, created_at`

func scanRule(r pgx.Row) (Rule, error) {
	var x Rule
	err := r.Scan(&x.ID, &x.Name, &x.Trigger, &x.OffsetDays, &x.SendHour, &x.Channel, &x.TemplateID, &x.ProductID, &x.IsActive, &x.CreatedAt)
	return x, err
}

func (Repository) Rules(ctx context.Context, q db.Querier, tenantID uuid.UUID, activeOnly bool) ([]Rule, error) {
	rows, err := q.Query(ctx, `SELECT `+ruleCols+` FROM reminder_rules WHERE tenant_id = $1 AND (NOT $2 OR is_active)
		ORDER BY trigger, offset_days, created_at`, tenantID, activeOnly)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Rule, error) { return scanRule(r) })
}

func (Repository) RulesByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Rule, error) {
	rows, err := q.Query(ctx, `SELECT `+ruleCols+` FROM reminder_rules WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Rule, error) { return scanRule(r) })
}

func (Repository) InsertRule(ctx context.Context, q db.Querier, tenantID uuid.UUID, in RuleInput) (Rule, error) {
	r, err := scanRule(q.QueryRow(ctx, `INSERT INTO reminder_rules (tenant_id, name, trigger, offset_days, send_hour, channel, template_id, product_id, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+ruleCols, tenantID, in.Name, in.Trigger, in.OffsetDays, in.SendHour, in.Channel,
		in.TemplateID, in.ProductID, in.IsActive))
	return r, db.MapErr(err, "reminder rule")
}

func (Repository) UpdateRule(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, in RuleInput) (Rule, error) {
	r, err := scanRule(q.QueryRow(ctx, `UPDATE reminder_rules SET name = $3, trigger = $4, offset_days = $5, send_hour = $6, channel = $7,
		template_id = $8, product_id = $9, is_active = $10 WHERE tenant_id = $1 AND id = $2 RETURNING `+ruleCols,
		tenantID, id, in.Name, in.Trigger, in.OffsetDays, in.SendHour, in.Channel, in.TemplateID, in.ProductID, in.IsActive))
	return r, db.MapErr(err, "reminder rule")
}

func (Repository) DeleteRule(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (bool, error) {
	// Future reminders of the rule are cancelled; history keeps rule_id = NULL.
	if _, err := q.Exec(ctx, `UPDATE reminders SET status = 'cancelled' WHERE tenant_id = $1 AND rule_id = $2 AND status = 'scheduled'`, tenantID, id); err != nil {
		return false, err
	}
	tag, err := q.Exec(ctx, `DELETE FROM reminder_rules WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return tag.RowsAffected() > 0, err
}

// CancelScheduledForRule drops not-yet-sent reminders of a rule (after the rule changed).
func (Repository) CancelScheduledForRule(ctx context.Context, q db.Querier, tenantID, ruleID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE reminders SET status = 'cancelled' WHERE tenant_id = $1 AND rule_id = $2 AND status = 'scheduled'`, tenantID, ruleID)
	return err
}

// ---- planner reads

// Anchor is one entity a rule may produce a reminder for.
type Anchor struct {
	ID         uuid.UUID
	CustomerID *uuid.UUID
	At         time.Time
}

func collectAnchors(rows pgx.Rows, err error) ([]Anchor, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Anchor, error) {
		var a Anchor
		err := r.Scan(&a.ID, &a.CustomerID, &a.At)
		return a, err
	})
}

// SubscriptionAnchors: live (expired=false) or expired-and-not-renewed subscriptions expiring in [from, to].
func (Repository) SubscriptionAnchors(ctx context.Context, q db.Querier, tenantID uuid.UUID, productID *uuid.UUID, expired bool, from, to time.Time) ([]Anchor, error) {
	status := `s.status IN ('pending','active','expiring')`
	if expired {
		status = `s.status = 'expired'`
	}
	return collectAnchors(q.Query(ctx, `SELECT s.id, s.customer_id, s.expires_at FROM subscriptions s
		WHERE s.tenant_id = $1 AND `+status+` AND s.expires_at BETWEEN $2 AND $3 AND ($4::uuid IS NULL OR s.product_id = $4)
		  AND NOT EXISTS (SELECT 1 FROM subscriptions n WHERE n.tenant_id = s.tenant_id AND n.renewed_from_id = s.id
		                  AND n.status IN ('pending','active','expiring'))`, tenantID, from, to, productID))
}

// InvoiceAnchors: unpaid invoices due in [from, to].
func (Repository) InvoiceAnchors(ctx context.Context, q db.Querier, tenantID uuid.UUID, from, to time.Time) ([]Anchor, error) {
	return collectAnchors(q.Query(ctx, `SELECT i.id, i.customer_id, i.due_at FROM invoices i
		WHERE i.tenant_id = $1 AND i.status IN ('sent','partial','overdue') AND i.total > i.paid_amount
		  AND i.due_at BETWEEN $2 AND $3`, tenantID, from, to))
}

// MasterAnchors: active master accounts expiring in [from, to].
func (Repository) MasterAnchors(ctx context.Context, q db.Querier, tenantID uuid.UUID, productID *uuid.UUID, from, to time.Time) ([]Anchor, error) {
	return collectAnchors(q.Query(ctx, `SELECT m.id, NULL::uuid, m.expires_at FROM master_accounts m
		WHERE m.tenant_id = $1 AND m.status IN ('active','expiring') AND m.expires_at BETWEEN $2 AND $3
		  AND ($4::uuid IS NULL OR m.product_id = $4)`, tenantID, from, to, productID))
}

// Upsert materialises one reminder; returns false when it already exists (any status).
func (Repository) Upsert(ctx context.Context, q db.Querier, tenantID uuid.UUID, r Reminder) (bool, error) {
	if r.SubscriptionID != nil {
		tag, err := q.Exec(ctx, `INSERT INTO reminders (tenant_id, rule_id, customer_id, subscription_id, channel, scheduled_at)
			VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (rule_id, subscription_id, scheduled_at) WHERE subscription_id IS NOT NULL DO NOTHING`,
			tenantID, r.RuleID, r.CustomerID, r.SubscriptionID, r.Channel, r.ScheduledAt)
		return tag.RowsAffected() > 0, err
	}
	tag, err := q.Exec(ctx, `INSERT INTO reminders (tenant_id, rule_id, customer_id, invoice_id, master_account_id, channel, scheduled_at)
		SELECT $1,$2,$3,$4,$5,$6,$7 WHERE NOT EXISTS (SELECT 1 FROM reminders WHERE tenant_id = $1 AND rule_id = $2
		  AND invoice_id IS NOT DISTINCT FROM $4 AND master_account_id IS NOT DISTINCT FROM $5 AND scheduled_at = $7)`,
		tenantID, r.RuleID, r.CustomerID, r.InvoiceID, r.MasterAccountID, r.Channel, r.ScheduledAt)
	return tag.RowsAffected() > 0, err
}

// ---- reminders

const cols = `r.id, r.rule_id, r.customer_id, r.subscription_id, r.invoice_id, r.master_account_id, r.channel, r.scheduled_at, r.sent_at,
	r.status, r.rendered_body, r.error, r.created_at`

func scan(row pgx.Row) (Reminder, error) {
	var r Reminder
	err := row.Scan(&r.ID, &r.RuleID, &r.CustomerID, &r.SubscriptionID, &r.InvoiceID, &r.MasterAccountID, &r.Channel, &r.ScheduledAt,
		&r.SentAt, &r.Status, &r.RenderedBody, &r.Error, &r.CreatedAt)
	return r, err
}

func collect(rows pgx.Rows, err error) ([]Reminder, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Reminder, error) { return scan(r) })
}

func (Repository) Insert(ctx context.Context, q db.Querier, tenantID uuid.UUID, r Reminder) (Reminder, error) {
	out, err := scan(q.QueryRow(ctx, `INSERT INTO reminders AS r (tenant_id, rule_id, customer_id, subscription_id, invoice_id, master_account_id,
		channel, scheduled_at, sent_at, status, rendered_body, error) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+cols,
		tenantID, r.RuleID, r.CustomerID, r.SubscriptionID, r.InvoiceID, r.MasterAccountID, r.Channel, r.ScheduledAt, r.SentAt, r.Status,
		r.RenderedBody, r.Error))
	return out, db.MapErr(err, "reminder")
}

func (Repository) Get(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (Reminder, error) {
	r, err := scan(q.QueryRow(ctx, `SELECT `+cols+` FROM reminders r WHERE r.tenant_id = $1 AND r.id = $2`, tenantID, id))
	return r, db.MapErr(err, "reminder")
}

var keyset = sqlb.Keyset{Sort: "r.scheduled_at", ID: "r.id", From: "reminders r"}

func (Repository) List(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter, page pagination.Page) ([]Reminder, error) {
	w := sqlb.New("r.tenant_id", tenantID)
	sqlb.Eq(w, "r.status", f.Status)
	sqlb.Eq(w, "r.customer_id", f.CustomerID)
	return collect(q.Query(ctx, `SELECT `+cols+` FROM reminders r`+w.Page(page, keyset), w.Args()...))
}

// Due claims scheduled reminders whose time has come (SKIP LOCKED: safe with many workers).
func (Repository) Due(ctx context.Context, q db.Querier, tenantID uuid.UUID, now time.Time, limit int) ([]Reminder, error) {
	return collect(q.Query(ctx, `SELECT `+cols+` FROM reminders r WHERE r.tenant_id = $1 AND r.status = 'scheduled' AND r.scheduled_at <= $2
		ORDER BY r.scheduled_at LIMIT $3 FOR UPDATE SKIP LOCKED`, tenantID, now, limit))
}

// ActiveTenants lists active tenants (platform table).
func (Repository) ActiveTenants(ctx context.Context, q db.Querier) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, `SELECT id FROM tenants WHERE status = 'active' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// HasDue reports whether the tenant has scheduled reminders due by now.
func (Repository) HasDue(ctx context.Context, q db.Querier, tenantID uuid.UUID, now time.Time) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM reminders WHERE tenant_id = $1 AND status = 'scheduled' AND scheduled_at <= $2)`,
		tenantID, now).Scan(&ok)
	return ok, err
}

func (Repository) SetResult(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, status string, body, errMsg *string, sentAt *time.Time) error {
	_, err := q.Exec(ctx, `UPDATE reminders SET status = $3, rendered_body = COALESCE($4, rendered_body), error = $5, sent_at = $6
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, status, body, errMsg, sentAt)
	return err
}

func (Repository) Cancel(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE reminders SET status = 'cancelled' WHERE tenant_id = $1 AND id = $2 AND status = 'scheduled'`, tenantID, id)
	return tag.RowsAffected() > 0, err
}

// Context is what a reminder body is rendered from.
type Context struct {
	CustomerName    string
	ProductName     string
	ExpiresAt       *time.Time
	Amount          int64
	Currency        string
	SubscriptionOK  bool // subscription still eligible (live, or expired and not renewed)
	SubscriptionSts string
	InvoiceOpen     bool
	MasterLabel     string
}

func (Repository) LoadContext(ctx context.Context, q db.Querier, tenantID uuid.UUID, r Reminder) (Context, error) {
	var c Context
	if r.CustomerID != nil {
		if err := q.QueryRow(ctx, `SELECT name FROM customers WHERE tenant_id = $1 AND id = $2`, tenantID, *r.CustomerID).Scan(&c.CustomerName); err != nil {
			return c, db.MapErr(err, "customer")
		}
	}
	switch {
	case r.SubscriptionID != nil:
		var exp time.Time
		err := q.QueryRow(ctx, `SELECT p.name, s.expires_at, COALESCE(pl.price, s.price), COALESCE(pl.currency, s.currency), s.status,
			NOT EXISTS (SELECT 1 FROM subscriptions n WHERE n.tenant_id = s.tenant_id AND n.renewed_from_id = s.id AND n.status IN ('pending','active','expiring'))
			FROM subscriptions s JOIN products p ON p.id = s.product_id AND p.tenant_id = s.tenant_id
			LEFT JOIN product_plans pl ON pl.id = s.plan_id AND pl.tenant_id = s.tenant_id
			WHERE s.tenant_id = $1 AND s.id = $2`, tenantID, *r.SubscriptionID).
			Scan(&c.ProductName, &exp, &c.Amount, &c.Currency, &c.SubscriptionSts, &c.SubscriptionOK)
		if err != nil {
			return c, db.MapErr(err, "subscription")
		}
		c.ExpiresAt = &exp
		c.SubscriptionOK = c.SubscriptionOK && c.SubscriptionSts != "cancelled" && c.SubscriptionSts != "replaced"
	case r.InvoiceID != nil:
		err := q.QueryRow(ctx, `SELECT i.number, i.due_at, i.total - i.paid_amount, i.currency, i.status IN ('sent','partial','overdue') AND i.total > i.paid_amount
			FROM invoices i WHERE i.tenant_id = $1 AND i.id = $2`, tenantID, *r.InvoiceID).
			Scan(&c.ProductName, &c.ExpiresAt, &c.Amount, &c.Currency, &c.InvoiceOpen)
		if err != nil {
			return c, db.MapErr(err, "invoice")
		}
	case r.MasterAccountID != nil:
		err := q.QueryRow(ctx, `SELECT m.label || ' (' || p.name || ')', m.expires_at FROM master_accounts m
			JOIN products p ON p.id = m.product_id AND p.tenant_id = m.tenant_id WHERE m.tenant_id = $1 AND m.id = $2`,
			tenantID, *r.MasterAccountID).Scan(&c.ProductName, &c.ExpiresAt)
		if err != nil {
			return c, db.MapErr(err, "master account")
		}
		c.MasterLabel = c.ProductName
	}
	return c, nil
}
