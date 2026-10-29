package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
)

type Repository struct{}

const productCols = `id, tenant_id, name, category, description, image_url, delivery_type, warranty_days, delivery_template, is_active, sort_order, created_at`

func scanProduct(r pgx.Row) (Product, error) {
	var p Product
	err := r.Scan(&p.ID, &p.TenantID, &p.Name, &p.Category, &p.Description, &p.ImageURL, &p.DeliveryType, &p.WarrantyDays,
		&p.DeliveryTemplate, &p.IsActive, &p.SortOrder, &p.CreatedAt)
	return p, err
}

func collectProducts(rows pgx.Rows, err error) ([]Product, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Product, error) { return scanProduct(r) })
}

const planCols = `id, product_id, name, duration_days, price, cost, currency, is_active, sort_order`

func scanPlan(r pgx.Row) (Plan, error) {
	var p Plan
	err := r.Scan(&p.ID, &p.ProductID, &p.Name, &p.DurationDays, &p.Price, &p.Cost, &p.Currency, &p.IsActive, &p.SortOrder)
	return p, err
}

func collectPlans(rows pgx.Rows, err error) ([]Plan, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Plan, error) { return scanPlan(r) })
}

func (Repository) List(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter, limit int) ([]Product, error) {
	args := []any{tenantID}
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	conds := []string{"tenant_id = $1", "deleted_at IS NULL"}
	if f.Search != nil && strings.TrimSpace(*f.Search) != "" {
		p := add(db.Contains(*f.Search))
		conds = append(conds, fmt.Sprintf("(name ILIKE %[1]s OR description ILIKE %[1]s)", p))
	}
	if f.Category != nil {
		conds = append(conds, "category = "+add(strings.ToLower(*f.Category)))
	}
	if f.IsActive != nil {
		conds = append(conds, "is_active = "+add(*f.IsActive))
	}
	lim := add(limit)
	return collectProducts(q.Query(ctx, `SELECT `+productCols+` FROM products WHERE `+strings.Join(conds, " AND ")+
		` ORDER BY sort_order, name LIMIT `+lim, args...))
}

func (Repository) Get(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (Product, error) {
	p, err := scanProduct(q.QueryRow(ctx, `SELECT `+productCols+` FROM products WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id))
	return p, db.MapErr(err, "product")
}

func (Repository) GetByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Product, error) {
	return collectProducts(q.Query(ctx, `SELECT `+productCols+` FROM products WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, ids))
}

func (Repository) Insert(ctx context.Context, q db.Querier, tenantID uuid.UUID, in CreateInput) (Product, error) {
	p, err := scanProduct(q.QueryRow(ctx, `INSERT INTO products (tenant_id, name, category, description, image_url, delivery_type, warranty_days, delivery_template, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, COALESCE((SELECT max(sort_order)+1 FROM products WHERE tenant_id = $1), 0)) RETURNING `+productCols,
		tenantID, in.Name, in.Category, in.Description, in.ImageURL, in.DeliveryType, in.WarrantyDays, in.DeliveryTemplate))
	return p, db.MapErr(err, "product")
}

func (Repository) Update(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, in UpdateInput) (Product, error) {
	p, err := scanProduct(q.QueryRow(ctx, `UPDATE products SET
		name = COALESCE($3, name), category = COALESCE($4, category), description = COALESCE($5, description),
		image_url = COALESCE($6, image_url), delivery_type = COALESCE($7, delivery_type), warranty_days = COALESCE($8, warranty_days),
		delivery_template = COALESCE($9, delivery_template), is_active = COALESCE($10, is_active), sort_order = COALESCE($11, sort_order)
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL RETURNING `+productCols,
		tenantID, id, in.Name, in.Category, in.Description, in.ImageURL, in.DeliveryType, in.WarrantyDays, in.DeliveryTemplate, in.IsActive, in.SortOrder))
	return p, db.MapErr(err, "product")
}

func (Repository) SoftDelete(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE products SET deleted_at = now(), is_active = false WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	return tag.RowsAffected() > 0, err
}

func (Repository) InsertPlan(ctx context.Context, q db.Querier, tenantID, productID uuid.UUID, currency string, in PlanInput) (Plan, error) {
	p, err := scanPlan(q.QueryRow(ctx, `INSERT INTO product_plans (tenant_id, product_id, name, duration_days, price, cost, currency, is_active, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+planCols, tenantID, productID, in.Name, in.DurationDays, in.Price, in.Cost, currency, in.IsActive, in.SortOrder))
	return p, db.MapErr(err, "plan")
}

func (Repository) UpdatePlan(ctx context.Context, q db.Querier, tenantID, productID, id uuid.UUID, in PlanInput) (Plan, error) {
	p, err := scanPlan(q.QueryRow(ctx, `UPDATE product_plans SET name=$4, duration_days=$5, price=$6, cost=$7, is_active=$8, sort_order=$9
		WHERE tenant_id = $1 AND product_id = $2 AND id = $3 RETURNING `+planCols, tenantID, productID, id, in.Name, in.DurationDays, in.Price, in.Cost, in.IsActive, in.SortOrder))
	return p, db.MapErr(err, "plan")
}

func (Repository) DeletePlan(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `DELETE FROM product_plans WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return tag.RowsAffected() > 0, err
}

func (Repository) GetPlan(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (Plan, error) {
	p, err := scanPlan(q.QueryRow(ctx, `SELECT `+planCols+` FROM product_plans WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	return p, db.MapErr(err, "plan")
}

func (Repository) PlansByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Plan, error) {
	return collectPlans(q.Query(ctx, `SELECT `+planCols+` FROM product_plans WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, ids))
}

func (Repository) PlansByProducts(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Plan, error) {
	return collectPlans(q.Query(ctx, `SELECT `+planCols+` FROM product_plans WHERE tenant_id = $1 AND product_id = ANY($2)
		ORDER BY sort_order, duration_days, price`, tenantID, ids))
}

func (Repository) TenantCurrency(ctx context.Context, q db.Querier, tenantID uuid.UUID) (string, error) {
	var c string
	err := q.QueryRow(ctx, `SELECT currency FROM tenants WHERE id = $1`, tenantID).Scan(&c)
	return c, err
}
