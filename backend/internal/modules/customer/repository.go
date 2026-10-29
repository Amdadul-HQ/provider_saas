package customer

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

// Repository: every query is filtered by tenant_id and runs inside db.WithTenantTx.
type Repository struct{}

const cols = `c.id, c.tenant_id, c.name, c.phone, c.email::text, c.whatsapp, c.telegram, c.address, c.tags, c.notes,
	c.source, c.preferred_channel, c.due_balance, c.lifetime_value, c.status, c.owner_user_id, c.last_contact_at, c.created_at, c.updated_at`

func scan(row pgx.Row) (Customer, error) {
	var c Customer
	err := row.Scan(&c.ID, &c.TenantID, &c.Name, &c.Phone, &c.Email, &c.Whatsapp, &c.Telegram, &c.Address, &c.Tags, &c.Notes,
		&c.Source, &c.PreferredChannel, &c.DueBalance, &c.LifetimeValue, &c.Status, &c.OwnerUserID, &c.LastContactAt, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func collect(rows pgx.Rows, err error) ([]Customer, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Customer, error) { return scan(r) })
}

// where builds the filter clause; args[0] must be tenant id.
func where(f Filter, args *[]any) string {
	add := func(v any) string { *args = append(*args, v); return fmt.Sprintf("$%d", len(*args)) }
	conds := []string{"c.tenant_id = $1", "c.deleted_at IS NULL"}
	if f.Search != nil && strings.TrimSpace(*f.Search) != "" {
		p := add("%" + strings.TrimSpace(*f.Search) + "%")
		conds = append(conds, fmt.Sprintf("(c.name ILIKE %[1]s OR c.phone ILIKE %[1]s OR c.email::text ILIKE %[1]s OR c.whatsapp ILIKE %[1]s OR c.telegram ILIKE %[1]s)", p))
	}
	if f.Status != nil {
		conds = append(conds, "c.status = "+add(*f.Status))
	}
	if f.Source != nil {
		conds = append(conds, "c.source = "+add(*f.Source))
	}
	if len(f.Tags) > 0 {
		conds = append(conds, "c.tags @> "+add(f.Tags)+"::text[]")
	}
	if f.HasDue != nil {
		if *f.HasDue {
			conds = append(conds, "c.due_balance > 0")
		} else {
			conds = append(conds, "c.due_balance <= 0")
		}
	}
	if f.ExpiringWithinDays != nil {
		p := add(*f.ExpiringWithinDays)
		conds = append(conds, fmt.Sprintf(`EXISTS (SELECT 1 FROM subscriptions s WHERE s.tenant_id = c.tenant_id AND s.customer_id = c.id
			AND s.status IN ('active','expiring') AND s.expires_at <= now() + make_interval(days => %s))`, p))
	}
	return strings.Join(conds, " AND ")
}

func (Repository) List(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter, page pagination.Page) ([]Customer, error) {
	args := []any{tenantID}
	w := where(f, &args)
	if page.After != nil {
		args = append(args, page.After.CreatedAt, page.After.ID)
		w += fmt.Sprintf(" AND (c.created_at, c.id) < ($%d, $%d)", len(args)-1, len(args))
	}
	args = append(args, page.First+1)
	return collect(q.Query(ctx, `SELECT `+cols+` FROM customers c WHERE `+w+
		fmt.Sprintf(` ORDER BY c.created_at DESC, c.id DESC LIMIT $%d`, len(args)), args...))
}

func (Repository) Count(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter) (int, error) {
	args := []any{tenantID}
	w := where(f, &args)
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM customers c WHERE `+w, args...).Scan(&n)
	return n, err
}

func (Repository) Get(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Customer, error) {
	sql := `SELECT ` + cols + ` FROM customers c WHERE c.tenant_id = $1 AND c.id = $2 AND c.deleted_at IS NULL`
	if lock {
		sql += ` FOR UPDATE`
	}
	c, err := scan(q.QueryRow(ctx, sql, tenantID, id))
	return c, db.MapErr(err, "customer")
}

func (Repository) GetByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Customer, error) {
	// Soft-deleted customers are still returned so historical subscriptions/invoices resolve.
	return collect(q.Query(ctx, `SELECT `+cols+` FROM customers c WHERE c.tenant_id = $1 AND c.id = ANY($2)`, tenantID, ids))
}

func (Repository) Insert(ctx context.Context, q db.Querier, tenantID uuid.UUID, owner *uuid.UUID, in CreateInput) (Customer, error) {
	c, err := scan(q.QueryRow(ctx, `INSERT INTO customers AS c (tenant_id, name, phone, email, whatsapp, telegram, address, tags, notes, source, preferred_channel, owner_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+cols,
		tenantID, in.Name, in.Phone, in.Email, in.Whatsapp, in.Telegram, in.Address, in.Tags, in.Notes, in.Source, in.PreferredChannel, owner))
	return c, db.MapErr(err, "customer")
}

func (Repository) Update(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, in UpdateInput) (Customer, error) {
	// NULLIF(x,'') lets clients clear an optional field by sending "".
	c, err := scan(q.QueryRow(ctx, `UPDATE customers AS c SET
		name = COALESCE($3, c.name),
		phone = CASE WHEN $4::text IS NULL THEN c.phone ELSE NULLIF($4, '') END,
		email = CASE WHEN $5::text IS NULL THEN c.email ELSE NULLIF($5, '')::citext END,
		whatsapp = CASE WHEN $6::text IS NULL THEN c.whatsapp ELSE NULLIF($6, '') END,
		telegram = CASE WHEN $7::text IS NULL THEN c.telegram ELSE NULLIF($7, '') END,
		address = CASE WHEN $8::text IS NULL THEN c.address ELSE NULLIF($8, '') END,
		tags = COALESCE($9::text[], c.tags),
		notes = CASE WHEN $10::text IS NULL THEN c.notes ELSE NULLIF($10, '') END,
		status = COALESCE($11, c.status),
		preferred_channel = COALESCE($12, c.preferred_channel)
		WHERE c.tenant_id = $1 AND c.id = $2 AND c.deleted_at IS NULL RETURNING `+cols,
		tenantID, id, in.Name, in.Phone, in.Email, in.Whatsapp, in.Telegram, in.Address, in.Tags, in.Notes, in.Status, in.PreferredChannel))
	return c, db.MapErr(err, "customer")
}

func (Repository) SoftDelete(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE customers SET deleted_at = now() WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`, tenantID, id)
	return tag.RowsAffected() > 0, err
}

func (Repository) AdjustBalance(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, dueDelta, ltvDelta int64) error {
	tag, err := q.Exec(ctx, `UPDATE customers SET due_balance = due_balance + $3, lifetime_value = lifetime_value + $4
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, dueDelta, ltvDelta)
	if err == nil && tag.RowsAffected() == 0 {
		return db.MapErr(pgx.ErrNoRows, "customer")
	}
	return err
}

func (Repository) Touch(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, at time.Time) error {
	_, err := q.Exec(ctx, `UPDATE customers SET last_contact_at = GREATEST(COALESCE(last_contact_at, $3), $3) WHERE tenant_id = $1 AND id = $2`, tenantID, id, at)
	return err
}

func (Repository) InsertNote(ctx context.Context, q db.Querier, tenantID, customerID uuid.UUID, author *uuid.UUID, body string) (Note, error) {
	var n Note
	err := q.QueryRow(ctx, `INSERT INTO customer_notes (tenant_id, customer_id, author_id, body) VALUES ($1,$2,$3,$4)
		RETURNING id, customer_id, author_id, body, created_at`, tenantID, customerID, author, body).
		Scan(&n.ID, &n.CustomerID, &n.AuthorID, &n.Body, &n.CreatedAt)
	return n, db.MapErr(err, "note")
}

func (Repository) Notes(ctx context.Context, q db.Querier, tenantID, customerID uuid.UUID) ([]Note, error) {
	rows, err := q.Query(ctx, `SELECT id, customer_id, author_id, body, created_at FROM customer_notes
		WHERE tenant_id = $1 AND customer_id = $2 ORDER BY created_at DESC`, tenantID, customerID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Note, error) {
		var n Note
		err := r.Scan(&n.ID, &n.CustomerID, &n.AuthorID, &n.Body, &n.CreatedAt)
		return n, err
	})
}

const identityCols = `id, customer_id, platform, external_id, handle, avatar_url, social_account_id, created_at`

func scanIdentity(r pgx.Row) (Identity, error) {
	var i Identity
	err := r.Scan(&i.ID, &i.CustomerID, &i.Platform, &i.ExternalID, &i.Handle, &i.AvatarURL, &i.SocialAccountID, &i.CreatedAt)
	return i, err
}

func (Repository) IdentitiesByCustomers(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Identity, error) {
	rows, err := q.Query(ctx, `SELECT `+identityCols+` FROM customer_identities WHERE tenant_id = $1 AND customer_id = ANY($2) ORDER BY created_at`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Identity, error) { return scanIdentity(r) })
}

func (Repository) GetIdentity(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (Identity, error) {
	i, err := scanIdentity(q.QueryRow(ctx, `SELECT `+identityCols+` FROM customer_identities WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id))
	return i, db.MapErr(err, "identity")
}

func (Repository) InsertIdentity(ctx context.Context, q db.Querier, tenantID, customerID uuid.UUID, platform, externalID string, handle *string, socialAccountID *uuid.UUID) (Identity, error) {
	i, err := scanIdentity(q.QueryRow(ctx, `INSERT INTO customer_identities (tenant_id, customer_id, platform, external_id, handle, social_account_id)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+identityCols, tenantID, customerID, platform, externalID, handle, socialAccountID))
	return i, db.MapErr(err, "identity")
}

// MoveIdentity re-points an identity (and its conversations) to another customer.
func (Repository) MoveIdentity(ctx context.Context, q db.Querier, tenantID, identityID, customerID uuid.UUID) error {
	if _, err := q.Exec(ctx, `UPDATE customer_identities SET customer_id = $3 WHERE tenant_id = $1 AND id = $2`, tenantID, identityID, customerID); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `UPDATE conversations SET customer_id = $3 WHERE tenant_id = $1 AND identity_id = $2`, tenantID, identityID, customerID)
	return err
}

// mergeTables lists every table whose customer_id must follow a merge.
var mergeTables = []string{
	"customer_notes", "subscriptions", "invoices", "payments", "ledger_entries", "conversations",
	"tickets", "reminders", "orders", "portal_tokens",
}

// Merge moves every child row of mergeID to keepID. Identities that would collide on
// (platform, external_id) cannot exist (unique per tenant), so a plain update is safe.
func (Repository) Merge(ctx context.Context, q db.Querier, tenantID, keepID, mergeID uuid.UUID) error {
	if _, err := q.Exec(ctx, `UPDATE customer_identities SET customer_id = $3 WHERE tenant_id = $1 AND customer_id = $2`, tenantID, mergeID, keepID); err != nil {
		return err
	}
	for _, t := range mergeTables {
		if _, err := q.Exec(ctx, `UPDATE `+t+` SET customer_id = $3 WHERE tenant_id = $1 AND customer_id = $2`, tenantID, mergeID, keepID); err != nil {
			return fmt.Errorf("merge %s: %w", t, err)
		}
	}
	_, err := q.Exec(ctx, `UPDATE customers k SET
		due_balance = k.due_balance + m.due_balance,
		lifetime_value = k.lifetime_value + m.lifetime_value,
		tags = ARRAY(SELECT DISTINCT unnest(k.tags || m.tags)),
		phone = COALESCE(k.phone, m.phone), email = COALESCE(k.email, m.email),
		whatsapp = COALESCE(k.whatsapp, m.whatsapp), telegram = COALESCE(k.telegram, m.telegram),
		address = COALESCE(k.address, m.address),
		last_contact_at = GREATEST(k.last_contact_at, m.last_contact_at)
		FROM customers m WHERE k.tenant_id = $1 AND k.id = $2 AND m.tenant_id = $1 AND m.id = $3`, tenantID, keepID, mergeID)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `UPDATE customers SET deleted_at = now(), due_balance = 0, lifetime_value = 0,
		notes = COALESCE(notes || E'\n', '') || 'merged into ' || $3::text WHERE tenant_id = $1 AND id = $2`, tenantID, mergeID, keepID)
	return err
}

func (Repository) FindIdentity(ctx context.Context, q db.Querier, tenantID uuid.UUID, platform, externalID string) (Identity, error) {
	i, err := scanIdentity(q.QueryRow(ctx, `SELECT `+identityCols+` FROM customer_identities
		WHERE tenant_id = $1 AND platform = $2 AND external_id = $3`, tenantID, platform, externalID))
	return i, db.MapErr(err, "identity")
}

func (Repository) FindByPhone(ctx context.Context, q db.Querier, tenantID uuid.UUID, phone string) (Customer, error) {
	c, err := scan(q.QueryRow(ctx, `SELECT `+cols+` FROM customers c WHERE c.tenant_id = $1 AND c.deleted_at IS NULL
		AND (c.phone = $2 OR c.whatsapp = $2) ORDER BY c.created_at LIMIT 1`, tenantID, phone))
	return c, db.MapErr(err, "customer")
}
