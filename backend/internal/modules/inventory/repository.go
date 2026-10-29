package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
)

type Repository struct{}

// ---- suppliers

const supplierCols = `id, name, contact, notes, rating, created_at`

func scanSupplier(r pgx.Row) (Supplier, error) {
	var s Supplier
	var rating *int16
	err := r.Scan(&s.ID, &s.Name, &s.Contact, &s.Notes, &rating, &s.CreatedAt)
	if rating != nil {
		v := int(*rating)
		s.Rating = &v
	}
	return s, err
}

func collectSuppliers(rows pgx.Rows, err error) ([]Supplier, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Supplier, error) { return scanSupplier(r) })
}

func (Repository) Suppliers(ctx context.Context, q db.Querier, tenantID uuid.UUID) ([]Supplier, error) {
	return collectSuppliers(q.Query(ctx, `SELECT `+supplierCols+` FROM suppliers WHERE tenant_id = $1 ORDER BY name`, tenantID))
}

func (Repository) SuppliersByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Supplier, error) {
	return collectSuppliers(q.Query(ctx, `SELECT `+supplierCols+` FROM suppliers WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, ids))
}

func (Repository) InsertSupplier(ctx context.Context, q db.Querier, tenantID uuid.UUID, in SupplierInput) (Supplier, error) {
	s, err := scanSupplier(q.QueryRow(ctx, `INSERT INTO suppliers (tenant_id, name, contact, notes, rating) VALUES ($1,$2,$3,$4,$5) RETURNING `+supplierCols,
		tenantID, in.Name, in.Contact, in.Notes, in.Rating))
	return s, db.MapErr(err, "supplier")
}

func (Repository) UpdateSupplier(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, in SupplierInput) (Supplier, error) {
	s, err := scanSupplier(q.QueryRow(ctx, `UPDATE suppliers SET name=$3, contact=$4, notes=$5, rating=$6 WHERE tenant_id = $1 AND id = $2 RETURNING `+supplierCols,
		tenantID, id, in.Name, in.Contact, in.Notes, in.Rating))
	return s, db.MapErr(err, "supplier")
}

func (Repository) DeleteSupplier(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `DELETE FROM suppliers WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return tag.RowsAffected() > 0, err
}

func (Repository) AccountCountsBySupplier(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	return countMap(ctx, q, `SELECT supplier_id, count(*) FROM master_accounts WHERE tenant_id = $1 AND supplier_id = ANY($2) GROUP BY supplier_id`, tenantID, ids)
}

func countMap(ctx context.Context, q db.Querier, sql string, args ...any) (map[uuid.UUID]int, error) {
	rows, err := q.Query(ctx, sql, args...)
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

// ---- master accounts

const masterCols = `id, product_id, supplier_id, label, login_hint, credential_secret_id, total_slots, purchase_cost, currency,
	purchased_at::timestamptz, expires_at, status, notes, created_at`

func scanMaster(r pgx.Row) (MasterAccount, error) {
	var m MasterAccount
	err := r.Scan(&m.ID, &m.ProductID, &m.SupplierID, &m.Label, &m.LoginHint, &m.CredentialSecretID, &m.TotalSlots, &m.PurchaseCost,
		&m.Currency, &m.PurchasedAt, &m.ExpiresAt, &m.Status, &m.Notes, &m.CreatedAt)
	return m, err
}

func collectMasters(rows pgx.Rows, err error) ([]MasterAccount, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (MasterAccount, error) { return scanMaster(r) })
}

func (Repository) ListMasters(ctx context.Context, q db.Querier, tenantID uuid.UUID, f MasterFilter, page pagination.Page) ([]MasterAccount, error) {
	args := []any{tenantID}
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	conds := []string{"tenant_id = $1"}
	if f.ProductID != nil {
		conds = append(conds, "product_id = "+add(*f.ProductID))
	}
	if f.SupplierID != nil {
		conds = append(conds, "supplier_id = "+add(*f.SupplierID))
	}
	if f.Status != nil {
		conds = append(conds, "status = "+add(*f.Status))
	}
	if f.ExpiringWithinDays != nil {
		conds = append(conds, "status IN ('active','expiring') AND expires_at <= now() + make_interval(days => "+add(*f.ExpiringWithinDays)+")")
	}
	if page.After != nil {
		conds = append(conds, fmt.Sprintf("(created_at, id) < (%s, %s)", add(page.After.CreatedAt), add(page.After.ID)))
	}
	lim := add(page.First)
	return collectMasters(q.Query(ctx, `SELECT `+masterCols+` FROM master_accounts WHERE `+strings.Join(conds, " AND ")+
		` ORDER BY created_at DESC, id DESC LIMIT `+lim, args...))
}

func (Repository) GetMaster(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (MasterAccount, error) {
	sql := `SELECT ` + masterCols + ` FROM master_accounts WHERE tenant_id = $1 AND id = $2`
	if lock {
		sql += ` FOR UPDATE`
	}
	m, err := scanMaster(q.QueryRow(ctx, sql, tenantID, id))
	return m, db.MapErr(err, "master account")
}

func (Repository) MastersByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]MasterAccount, error) {
	return collectMasters(q.Query(ctx, `SELECT `+masterCols+` FROM master_accounts WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, ids))
}

func (Repository) InsertMaster(ctx context.Context, q db.Querier, tenantID uuid.UUID, in CreateMasterInput, secretID *uuid.UUID, currency, status string) (MasterAccount, error) {
	m, err := scanMaster(q.QueryRow(ctx, `INSERT INTO master_accounts (tenant_id, product_id, supplier_id, label, login_hint, credential_secret_id,
		total_slots, purchase_cost, currency, purchased_at, expires_at, status, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::timestamptz::date,$11,$12,$13) RETURNING `+masterCols,
		tenantID, in.ProductID, in.SupplierID, in.Label, in.LoginHint, secretID, in.TotalSlots, in.PurchaseCost, currency,
		in.PurchasedAt, in.ExpiresAt, status, in.Notes))
	return m, db.MapErr(err, "master account")
}

func (Repository) UpdateMaster(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, in UpdateMasterInput) (MasterAccount, error) {
	m, err := scanMaster(q.QueryRow(ctx, `UPDATE master_accounts SET
		supplier_id = COALESCE($3, supplier_id), label = COALESCE($4, label), login_hint = COALESCE($5, login_hint),
		total_slots = COALESCE($6, total_slots), purchase_cost = COALESCE($7, purchase_cost),
		purchased_at = COALESCE($8::timestamptz::date, purchased_at), expires_at = COALESCE($9, expires_at),
		status = COALESCE($10, status), notes = COALESCE($11, notes)
		WHERE tenant_id = $1 AND id = $2 RETURNING `+masterCols,
		tenantID, id, in.SupplierID, in.Label, in.LoginHint, in.TotalSlots, in.PurchaseCost, in.PurchasedAt, in.ExpiresAt, in.Status, in.Notes))
	return m, db.MapErr(err, "master account")
}

func (Repository) SetMasterSecret(ctx context.Context, q db.Querier, tenantID, id, secretID uuid.UUID, hint *string) error {
	_, err := q.Exec(ctx, `UPDATE master_accounts SET credential_secret_id = $3, login_hint = COALESCE($4, login_hint) WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, secretID, hint)
	return err
}

// RefreshMasterStatuses flips active/expiring/expired by expiry date. Returns rows changed.
func (Repository) RefreshMasterStatuses(ctx context.Context, q db.Querier, tenantID uuid.UUID, now time.Time, window time.Duration) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE master_accounts SET status = CASE
			WHEN expires_at <= $2 THEN 'expired'
			WHEN expires_at <= $2 + $3::interval THEN 'expiring'
			ELSE 'active' END
		WHERE tenant_id = $1 AND status IN ('active','expiring','expired') AND expires_at IS NOT NULL
		  AND status <> CASE WHEN expires_at <= $2 THEN 'expired' WHEN expires_at <= $2 + $3::interval THEN 'expiring' ELSE 'active' END`,
		tenantID, now, fmt.Sprintf("%d seconds", int(window.Seconds())))
	return tag.RowsAffected(), err
}

// ---- slots

const slotCols = `s.id, s.master_account_id, s.slot_no, s.label, s.pin, s.status, s.subscription_id`

func scanSlot(r pgx.Row) (Slot, error) {
	var s Slot
	err := r.Scan(&s.ID, &s.MasterAccountID, &s.SlotNo, &s.Label, &s.Pin, &s.Status, &s.SubscriptionID)
	return s, err
}

func collectSlots(rows pgx.Rows, err error) ([]Slot, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Slot, error) { return scanSlot(r) })
}

// InsertSlots creates slot rows numbered from+1..to.
func (Repository) InsertSlots(ctx context.Context, q db.Querier, tenantID, masterID uuid.UUID, from, to int) error {
	_, err := q.Exec(ctx, `INSERT INTO slots (tenant_id, master_account_id, slot_no, label)
		SELECT $1, $2, n, 'Profile ' || n FROM generate_series($3::int + 1, $4::int) AS n`, tenantID, masterID, from, to)
	return err
}

// DeleteFreeSlotsAbove removes free slots with slot_no > keep; returns how many non-free slots block the shrink.
func (Repository) DeleteFreeSlotsAbove(ctx context.Context, q db.Querier, tenantID, masterID uuid.UUID, keep int) (blocked int, err error) {
	if err = q.QueryRow(ctx, `SELECT count(*) FROM slots WHERE tenant_id = $1 AND master_account_id = $2 AND slot_no > $3 AND status = 'assigned'`,
		tenantID, masterID, keep).Scan(&blocked); err != nil || blocked > 0 {
		return blocked, err
	}
	_, err = q.Exec(ctx, `DELETE FROM slots WHERE tenant_id = $1 AND master_account_id = $2 AND slot_no > $3`, tenantID, masterID, keep)
	return 0, err
}

func (Repository) MaxSlotNo(ctx context.Context, q db.Querier, tenantID, masterID uuid.UUID) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT COALESCE(max(slot_no), 0) FROM slots WHERE tenant_id = $1 AND master_account_id = $2`, tenantID, masterID).Scan(&n)
	return n, err
}

func (Repository) SlotsByMasters(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Slot, error) {
	return collectSlots(q.Query(ctx, `SELECT `+slotCols+` FROM slots s WHERE s.tenant_id = $1 AND s.master_account_id = ANY($2) ORDER BY s.slot_no`, tenantID, ids))
}

func (Repository) SlotsByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Slot, error) {
	return collectSlots(q.Query(ctx, `SELECT `+slotCols+` FROM slots s WHERE s.tenant_id = $1 AND s.id = ANY($2)`, tenantID, ids))
}

func (Repository) GetSlot(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Slot, uuid.UUID, string, error) {
	sql := `SELECT ` + slotCols + `, m.product_id, m.status FROM slots s JOIN master_accounts m ON m.id = s.master_account_id AND m.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1 AND s.id = $2`
	if lock {
		sql += ` FOR UPDATE OF s`
	}
	var s Slot
	var productID uuid.UUID
	var masterStatus string
	err := q.QueryRow(ctx, sql, tenantID, id).Scan(&s.ID, &s.MasterAccountID, &s.SlotNo, &s.Label, &s.Pin, &s.Status, &s.SubscriptionID, &productID, &masterStatus)
	return s, productID, masterStatus, db.MapErr(err, "slot")
}

// FreeSlots lists free slots of usable master accounts for a product.
func (Repository) FreeSlots(ctx context.Context, q db.Querier, tenantID, productID uuid.UUID) ([]Slot, error) {
	return collectSlots(q.Query(ctx, `SELECT `+slotCols+` FROM slots s JOIN master_accounts m ON m.id = s.master_account_id AND m.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1 AND m.product_id = $2 AND s.status = 'free' AND m.status IN ('active','expiring')
		ORDER BY m.created_at, s.slot_no`, tenantID, productID))
}

// PickFreeSlot locks the next free slot of a usable (active, or expiring but not yet
// expired) master account of the product, preferring 'active' accounts and filling one
// account before the next. SKIP LOCKED lets concurrent sales pick different slots
// instead of blocking or overselling.
func (Repository) PickFreeSlot(ctx context.Context, q db.Querier, tenantID, productID uuid.UUID) (Slot, error) {
	s, err := scanSlot(q.QueryRow(ctx, `SELECT `+slotCols+` FROM slots s JOIN master_accounts m ON m.id = s.master_account_id AND m.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1 AND m.product_id = $2 AND s.status = 'free' AND m.status IN ('active','expiring')
		  AND (m.expires_at IS NULL OR m.expires_at > now())
		ORDER BY (m.status = 'active') DESC, m.created_at, s.slot_no
		LIMIT 1 FOR UPDATE OF s SKIP LOCKED`, tenantID, productID))
	return s, err
}

func (Repository) SetSlotAssignment(ctx context.Context, q db.Querier, tenantID, slotID uuid.UUID, subID *uuid.UUID, status string) error {
	_, err := q.Exec(ctx, `UPDATE slots SET subscription_id = $3, status = $4 WHERE tenant_id = $1 AND id = $2`, tenantID, slotID, subID, status)
	return err
}

func (Repository) UpdateSlot(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, in UpdateSlotInput) (Slot, error) {
	s, err := scanSlot(q.QueryRow(ctx, `UPDATE slots s SET label = COALESCE($3, s.label), pin = COALESCE($4, s.pin), status = COALESCE($5, s.status)
		WHERE s.tenant_id = $1 AND s.id = $2 RETURNING `+slotCols, tenantID, id, in.Label, in.Pin, in.Status))
	return s, db.MapErr(err, "slot")
}

func (Repository) FreeCountsByMaster(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	return countMap(ctx, q, `SELECT master_account_id, count(*) FROM slots WHERE tenant_id = $1 AND master_account_id = ANY($2) AND status = 'free' GROUP BY master_account_id`, tenantID, ids)
}

func (Repository) FreeCountsByProduct(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	return countMap(ctx, q, `SELECT m.product_id, count(*) FROM slots s JOIN master_accounts m ON m.id = s.master_account_id AND m.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1 AND m.product_id = ANY($2) AND s.status = 'free' AND m.status IN ('active','expiring') GROUP BY m.product_id`, tenantID, ids)
}

func (Repository) TenantCurrency(ctx context.Context, q db.Querier, tenantID uuid.UUID) (string, error) {
	var c string
	err := q.QueryRow(ctx, `SELECT currency FROM tenants WHERE id = $1`, tenantID).Scan(&c)
	return c, err
}
