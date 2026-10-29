package subscription

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
)

type Repository struct{}

const cols = `s.id, s.customer_id, s.product_id, s.plan_id, s.slot_id, s.starts_at, s.expires_at, s.price, s.cost, s.currency,
	s.status, s.auto_renew, s.delivered_at, s.delivery_note, s.renewed_from_id, s.sold_by, s.notes, s.created_at`

func scan(r pgx.Row) (Subscription, error) {
	var s Subscription
	err := r.Scan(&s.ID, &s.CustomerID, &s.ProductID, &s.PlanID, &s.SlotID, &s.StartsAt, &s.ExpiresAt, &s.Price, &s.Cost, &s.Currency,
		&s.Status, &s.AutoRenew, &s.DeliveredAt, &s.DeliveryNote, &s.RenewedFromID, &s.SoldBy, &s.Notes, &s.CreatedAt)
	return s, err
}

func collect(rows pgx.Rows, err error) ([]Subscription, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Subscription, error) { return scan(r) })
}

func (Repository) Insert(ctx context.Context, q db.Querier, tenantID uuid.UUID, s Subscription) (Subscription, error) {
	out, err := scan(q.QueryRow(ctx, `INSERT INTO subscriptions AS s (tenant_id, customer_id, product_id, plan_id, slot_id, starts_at, expires_at,
		price, cost, currency, status, auto_renew, renewed_from_id, sold_by, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING `+cols,
		tenantID, s.CustomerID, s.ProductID, s.PlanID, s.SlotID, s.StartsAt, s.ExpiresAt, s.Price, s.Cost, s.Currency, s.Status,
		s.AutoRenew, s.RenewedFromID, s.SoldBy, s.Notes))
	return out, db.MapErr(err, "subscription")
}

func (Repository) Get(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Subscription, error) {
	sql := `SELECT ` + cols + ` FROM subscriptions s WHERE s.tenant_id = $1 AND s.id = $2`
	if lock {
		sql += ` FOR UPDATE`
	}
	out, err := scan(q.QueryRow(ctx, sql, tenantID, id))
	return out, db.MapErr(err, "subscription")
}

func (Repository) GetByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Subscription, error) {
	return collect(q.Query(ctx, `SELECT `+cols+` FROM subscriptions s WHERE s.tenant_id = $1 AND s.id = ANY($2)`, tenantID, ids))
}

func (Repository) ByCustomers(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Subscription, error) {
	return collect(q.Query(ctx, `SELECT `+cols+` FROM subscriptions s WHERE s.tenant_id = $1 AND s.customer_id = ANY($2)
		ORDER BY s.expires_at DESC`, tenantID, ids))
}

// Update persists the mutable fields.
func (Repository) Update(ctx context.Context, q db.Querier, tenantID uuid.UUID, s Subscription) (Subscription, error) {
	out, err := scan(q.QueryRow(ctx, `UPDATE subscriptions s SET slot_id = $3, expires_at = $4, status = $5, delivered_at = $6,
		delivery_note = $7, notes = $8, auto_renew = $9 WHERE s.tenant_id = $1 AND s.id = $2 RETURNING `+cols,
		tenantID, s.ID, s.SlotID, s.ExpiresAt, s.Status, s.DeliveredAt, s.DeliveryNote, s.Notes, s.AutoRenew))
	return out, db.MapErr(err, "subscription")
}

func where(f Filter, args *[]any) string {
	add := func(v any) string { *args = append(*args, v); return fmt.Sprintf("$%d", len(*args)) }
	conds := []string{"s.tenant_id = $1"}
	if f.Search != nil && strings.TrimSpace(*f.Search) != "" {
		p := add("%" + strings.TrimSpace(*f.Search) + "%")
		conds = append(conds, fmt.Sprintf(`(EXISTS (SELECT 1 FROM customers c WHERE c.tenant_id = s.tenant_id AND c.id = s.customer_id
			AND (c.name ILIKE %[1]s OR c.phone ILIKE %[1]s)) OR EXISTS (SELECT 1 FROM products p WHERE p.tenant_id = s.tenant_id
			AND p.id = s.product_id AND p.name ILIKE %[1]s) OR s.notes ILIKE %[1]s)`, p))
	}
	if f.Status != nil {
		conds = append(conds, "s.status = "+add(*f.Status))
	}
	if f.ProductID != nil {
		conds = append(conds, "s.product_id = "+add(*f.ProductID))
	}
	if f.CustomerID != nil {
		conds = append(conds, "s.customer_id = "+add(*f.CustomerID))
	}
	if f.ExpiringWithinDays != nil {
		conds = append(conds, "s.status IN ('active','expiring') AND s.expires_at <= now() + make_interval(days => "+add(*f.ExpiringWithinDays)+")")
	}
	if f.ExpiredWithinDays != nil {
		conds = append(conds, "s.status = 'expired' AND s.expires_at >= now() - make_interval(days => "+add(*f.ExpiredWithinDays)+")")
	}
	return strings.Join(conds, " AND ")
}

func (Repository) List(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter, page pagination.Page) ([]Subscription, error) {
	args := []any{tenantID}
	w := where(f, &args)
	if page.After != nil {
		args = append(args, page.After.CreatedAt, page.After.ID)
		w += fmt.Sprintf(" AND (s.created_at, s.id) < ($%d, $%d)", len(args)-1, len(args))
	}
	args = append(args, page.First+1)
	return collect(q.Query(ctx, `SELECT `+cols+` FROM subscriptions s WHERE `+w+
		fmt.Sprintf(` ORDER BY s.created_at DESC, s.id DESC LIMIT $%d`, len(args)), args...))
}

func (Repository) Count(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter) (int, error) {
	args := []any{tenantID}
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM subscriptions s WHERE `+where(f, &args), args...).Scan(&n)
	return n, err
}

func (Repository) Expiring(ctx context.Context, q db.Querier, tenantID uuid.UUID, days int) ([]Subscription, error) {
	return collect(q.Query(ctx, `SELECT `+cols+` FROM subscriptions s WHERE s.tenant_id = $1 AND s.status IN ('active','expiring')
		AND s.expires_at <= now() + make_interval(days => $2) ORDER BY s.expires_at`, tenantID, days))
}

func countBy(ctx context.Context, q db.Querier, col string, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := q.Query(ctx, `SELECT `+col+`, count(*) FROM subscriptions WHERE tenant_id = $1 AND `+col+` = ANY($2)
		AND status IN ('active','expiring') GROUP BY `+col, tenantID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func (Repository) ActiveCountByCustomers(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	return countBy(ctx, q, "customer_id", tenantID, ids)
}

func (Repository) ActiveCountByProducts(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	return countBy(ctx, q, "product_id", tenantID, ids)
}

type expiredRow struct {
	ID     uuid.UUID
	SlotID *uuid.UUID
}

// ExpireDue flips live subscriptions past expiry to expired and returns them (for slot release).
func (Repository) ExpireDue(ctx context.Context, q db.Querier, tenantID uuid.UUID, now time.Time) ([]expiredRow, error) {
	rows, err := q.Query(ctx, `UPDATE subscriptions SET status = 'expired' WHERE tenant_id = $1
		AND status IN ('pending','active','expiring') AND expires_at <= $2 RETURNING id, slot_id`, tenantID, now)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (expiredRow, error) {
		var e expiredRow
		err := r.Scan(&e.ID, &e.SlotID)
		return e, err
	})
}

// MarkExpiring flips active subscriptions within the window to expiring.
func (Repository) MarkExpiring(ctx context.Context, q db.Querier, tenantID uuid.UUID, now time.Time, window time.Duration) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE subscriptions SET status = 'expiring' WHERE tenant_id = $1 AND status IN ('active','pending')
		AND starts_at <= $2 AND expires_at > $2 AND expires_at <= $2 + $3::interval`, tenantID, now, fmt.Sprintf("%d seconds", int(window.Seconds())))
	return tag.RowsAffected(), err
}

// Activate flips pending subscriptions whose start has come to active.
func (Repository) Activate(ctx context.Context, q db.Querier, tenantID uuid.UUID, now time.Time) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE subscriptions SET status = 'active' WHERE tenant_id = $1 AND status = 'pending'
		AND starts_at <= $2 AND expires_at > $2`, tenantID, now)
	return tag.RowsAffected(), err
}

// HasLiveSuccessor reports whether a renewal of id is pending/active (keeps the slot).
func (Repository) LiveSuccessor(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (*uuid.UUID, error) {
	var sid uuid.UUID
	err := q.QueryRow(ctx, `SELECT id FROM subscriptions WHERE tenant_id = $1 AND renewed_from_id = $2
		AND status IN ('pending','active','expiring') LIMIT 1`, tenantID, id).Scan(&sid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sid, nil
}

func (Repository) SetSlot(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, slotID *uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE subscriptions SET slot_id = $3 WHERE tenant_id = $1 AND id = $2`, tenantID, id, slotID)
	return err
}

func (Repository) TenantsWithLiveSubscriptions(ctx context.Context, q db.Querier) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx, `SELECT DISTINCT tenant_id FROM subscriptions WHERE status IN ('pending','active','expiring')`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}
