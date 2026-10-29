package analytics

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/db"
)

// Service is the analytics module API.
type Service interface {
	Dashboard(ctx context.Context, tenantID uuid.UUID) (DashboardStats, error)
	RevenueSeries(ctx context.Context, tenantID uuid.UUID, days int) ([]SeriesPoint, error)
	ProductStats(ctx context.Context, tenantID uuid.UUID, days int) ([]ProductStat, error)
	ChannelStats(ctx context.Context, tenantID uuid.UUID, days int) ([]ChannelStat, error)
	AtRiskCustomers(ctx context.Context, tenantID uuid.UUID, limit int) ([]AtRiskCustomer, error)
}

type service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) Service { return &service{pool: pool} }

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

// Month boundaries use the tenant's timezone.
const dashboardSQL = `
WITH t AS (SELECT id, currency, timezone FROM tenants WHERE id = $1),
m AS (SELECT (date_trunc('month', now() AT TIME ZONE t.timezone) AT TIME ZONE t.timezone) AS start FROM t),
subs AS (
  SELECT
    count(*) FILTER (WHERE status IN ('active','expiring')) AS active,
    count(*) FILTER (WHERE status IN ('active','expiring') AND expires_at <= now() + interval '7 days') AS expiring7,
    COALESCE(sum(round(price * 30.0 / GREATEST(1, extract(epoch FROM expires_at - starts_at) / 86400)))
      FILTER (WHERE status IN ('active','expiring')), 0)::bigint AS mrr,
    COALESCE(sum(cost) FILTER (WHERE created_at >= (SELECT start FROM m) AND status <> 'cancelled'), 0)::bigint AS cost_month
  FROM subscriptions WHERE tenant_id = $1),
expired AS (
  SELECT count(*) AS n FROM subscriptions s
  WHERE s.tenant_id = $1 AND s.status = 'expired' AND s.expires_at >= now() - interval '30 days'
    AND NOT EXISTS (SELECT 1 FROM subscriptions r WHERE r.tenant_id = s.tenant_id AND r.renewed_from_id = s.id)
    AND NOT EXISTS (SELECT 1 FROM subscriptions o WHERE o.tenant_id = s.tenant_id AND o.customer_id = s.customer_id
                    AND o.product_id = s.product_id AND o.status IN ('pending','active','expiring'))),
cust AS (
  SELECT count(*) AS total,
         count(*) FILTER (WHERE created_at >= (SELECT start FROM m)) AS new_month,
         COALESCE(sum(due_balance) FILTER (WHERE due_balance > 0), 0)::bigint AS due
  FROM customers WHERE tenant_id = $1 AND deleted_at IS NULL),
pay AS (
  SELECT COALESCE(sum(amount) FILTER (WHERE status = 'verified' AND received_at >= (SELECT start FROM m)), 0)::bigint AS revenue,
         count(*) FILTER (WHERE status = 'pending') AS pending
  FROM payments WHERE tenant_id = $1),
tick AS (SELECT count(*) AS n FROM tickets WHERE tenant_id = $1 AND status IN ('open','in_progress','waiting_customer')),
conv AS (SELECT count(*) AS n FROM conversations WHERE tenant_id = $1 AND status = 'open'),
slot AS (SELECT count(*) AS n FROM slots s JOIN master_accounts ma ON ma.id = s.master_account_id AND ma.tenant_id = s.tenant_id
         WHERE s.tenant_id = $1 AND s.status = 'free' AND ma.status IN ('active','expiring')),
mast AS (SELECT count(*) AS n FROM master_accounts WHERE tenant_id = $1 AND status IN ('active','expiring')
         AND expires_at <= now() + interval '7 days')
SELECT subs.active, subs.expiring7, expired.n, cust.total, cust.new_month, subs.mrr, pay.revenue, subs.cost_month,
       cust.due, pay.pending, tick.n, conv.n, slot.n, mast.n, t.currency
FROM t, subs, expired, cust, pay, tick, conv, slot, mast`

func (s *service) Dashboard(ctx context.Context, tenantID uuid.UUID) (d DashboardStats, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, dashboardSQL, tenantID).Scan(&d.ActiveSubscriptions, &d.ExpiringIn7Days, &d.ExpiredUnrenewed30Days,
			&d.TotalCustomers, &d.NewCustomersThisMonth, &d.MRR, &d.RevenueThisMonth, &d.CostThisMonth, &d.TotalDue,
			&d.PendingPayments, &d.OpenTickets, &d.OpenConversations, &d.FreeSlots, &d.MasterAccountsExpiringIn7Days, &d.Currency)
	})
	d.ProfitThisMonth = d.RevenueThisMonth - d.CostThisMonth
	return d, err
}

func (s *service) RevenueSeries(ctx context.Context, tenantID uuid.UUID, days int) (out []SeriesPoint, err error) {
	days = ClampDays(days)
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH t AS (SELECT timezone FROM tenants WHERE id = $1),
			d AS (SELECT generate_series((now() AT TIME ZONE t.timezone)::date - ($2::int - 1), (now() AT TIME ZONE t.timezone)::date, interval '1 day')::date AS day FROM t)
			SELECT (d.day::timestamp AT TIME ZONE t.timezone) AS day, COALESCE(sum(p.amount), 0)::bigint, count(p.id)
			FROM d CROSS JOIN t
			LEFT JOIN payments p ON p.tenant_id = $1 AND p.status = 'verified'
			     AND (p.received_at AT TIME ZONE t.timezone)::date = d.day
			GROUP BY d.day, t.timezone ORDER BY d.day`, tenantID, days)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (SeriesPoint, error) {
			var p SeriesPoint
			err := r.Scan(&p.Date, &p.Value, &p.Count)
			return p, err
		})
		return err
	})
	return out, err
}

func (s *service) ProductStats(ctx context.Context, tenantID uuid.UUID, days int) (out []ProductStat, err error) {
	days = ClampDays(days)
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT p.id,
			  count(s.id) FILTER (WHERE s.status IN ('active','expiring')),
			  COALESCE(sum(s.price) FILTER (WHERE s.created_at >= now() - make_interval(days => $2) AND s.status <> 'cancelled'), 0)::bigint,
			  COALESCE(sum(s.cost) FILTER (WHERE s.created_at >= now() - make_interval(days => $2) AND s.status <> 'cancelled'), 0)::bigint,
			  count(s.id) FILTER (WHERE s.expires_at BETWEEN now() - make_interval(days => $2) AND now() AND s.status <> 'cancelled'
			        AND EXISTS (SELECT 1 FROM subscriptions r WHERE r.tenant_id = s.tenant_id AND r.renewed_from_id = s.id)),
			  count(s.id) FILTER (WHERE s.expires_at BETWEEN now() - make_interval(days => $2) AND now() AND s.status <> 'cancelled')
			FROM products p LEFT JOIN subscriptions s ON s.tenant_id = p.tenant_id AND s.product_id = p.id
			WHERE p.tenant_id = $1 AND p.deleted_at IS NULL
			GROUP BY p.id, p.sort_order, p.name ORDER BY p.sort_order, p.name`, tenantID, days)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (ProductStat, error) {
			var p ProductStat
			var renewed, ended int
			err := r.Scan(&p.ProductID, &p.ActiveSubscriptions, &p.Revenue, &p.Cost, &renewed, &ended)
			p.Profit = p.Revenue - p.Cost
			p.RenewalRate = RenewalRate(renewed, ended)
			return p, err
		})
		return err
	})
	return out, err
}

func (s *service) ChannelStats(ctx context.Context, tenantID uuid.UUID, days int) (out []ChannelStat, err error) {
	days = ClampDays(days)
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT pl.platform,
			  (SELECT count(*) FROM conversations c WHERE c.tenant_id = $1 AND c.platform = pl.platform AND c.created_at >= now() - make_interval(days => $2)),
			  (SELECT count(*) FROM customers cu WHERE cu.tenant_id = $1 AND cu.deleted_at IS NULL AND cu.source = pl.platform AND cu.created_at >= now() - make_interval(days => $2)),
			  (SELECT count(*) FROM orders o JOIN conversations c ON c.id = o.conversation_id AND c.tenant_id = o.tenant_id
			     WHERE o.tenant_id = $1 AND c.platform = pl.platform AND o.created_at >= now() - make_interval(days => $2))
			FROM unnest(ARRAY['facebook','instagram','whatsapp','telegram']) AS pl(platform)`, tenantID, days)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (ChannelStat, error) {
			var c ChannelStat
			err := r.Scan(&c.Platform, &c.Conversations, &c.Customers, &c.Orders)
			return c, err
		})
		return err
	})
	return out, err
}

func (s *service) AtRiskCustomers(ctx context.Context, tenantID uuid.UUID, limit int) (out []AtRiskCustomer, err error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH risks AS (
			  SELECT s.customer_id, 1 AS prio, 'EXPIRING_SOON' AS reason, s.expires_at
			  FROM subscriptions s
			  WHERE s.tenant_id = $1 AND s.status IN ('active','expiring') AND s.expires_at <= now() + interval '3 days'
			    AND NOT EXISTS (SELECT 1 FROM subscriptions r WHERE r.tenant_id = s.tenant_id AND r.renewed_from_id = s.id)
			  UNION ALL
			  SELECT i.customer_id, 2, 'OVERDUE_INVOICE', i.due_at FROM invoices i
			  WHERE i.tenant_id = $1 AND i.status NOT IN ('paid','void') AND i.due_at < now() AND i.total > i.paid_amount
			  UNION ALL
			  SELECT c.id, 3, 'HAS_DUE', NULL::timestamptz FROM customers c WHERE c.tenant_id = $1 AND c.due_balance > 0 AND c.deleted_at IS NULL
			), best AS (
			  SELECT DISTINCT ON (r.customer_id) r.customer_id, r.prio, r.reason, r.expires_at
			  FROM risks r ORDER BY r.customer_id, r.prio, r.expires_at
			)
			SELECT b.customer_id, b.reason, b.expires_at, c.due_balance
			FROM best b JOIN customers c ON c.id = b.customer_id AND c.tenant_id = $1 AND c.deleted_at IS NULL
			ORDER BY b.prio, b.expires_at NULLS LAST, c.due_balance DESC
			LIMIT $2`, tenantID, limit)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (AtRiskCustomer, error) {
			var a AtRiskCustomer
			err := r.Scan(&a.CustomerID, &a.Reason, &a.ExpiresAt, &a.DueBalance)
			return a, err
		})
		return err
	})
	return out, err
}
