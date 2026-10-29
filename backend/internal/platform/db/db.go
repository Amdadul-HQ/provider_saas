// Package db owns the pgx pool, tenant-scoped transactions (ADR-002) and migrations.
package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/apperr"
)

// Querier is satisfied by pgx.Tx, *pgxpool.Pool and *pgx.Conn.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Connect opens and pings a pgx pool.
func Connect(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

type txKey struct{}

type txState struct {
	tx       pgx.Tx
	tenantID uuid.UUID
}

// TxFn is the unit of work run inside a transaction. ctx carries the tx so that
// nested service calls (e.g. subscription -> billing) join the same transaction.
type TxFn func(ctx context.Context, tx pgx.Tx) error

// WithTenantTx runs fn inside a transaction with `app.tenant_id` set locally (RLS,
// ADR-002). If ctx already carries a transaction it is reused (same tenant only).
// Note: `SET LOCAL x = $1` cannot take bind parameters, so set_config(..., true) is used.
func WithTenantTx(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, fn TxFn) error {
	if tenantID == uuid.Nil {
		return apperr.Unauthenticated("no tenant selected")
	}
	if st, ok := ctx.Value(txKey{}).(*txState); ok {
		switch st.tenantID {
		case tenantID:
		case uuid.Nil:
			if err := setTenant(ctx, st.tx, tenantID); err != nil {
				return err
			}
			st.tenantID = tenantID
		default:
			return errors.New("db: nested transaction for a different tenant")
		}
		return fn(ctx, st.tx)
	}
	return run(ctx, pool, tenantID, fn)
}

// WithPlatformTx runs fn in a transaction without tenant scope. Only platform tables
// (tenants, users, refresh_tokens, jobs) are visible: every tenant table has a fail-closed
// RLS policy (migration 00009), so a tenant query that forgot WithTenantTx sees no rows
// instead of every tenant's rows. Reuses an outer transaction from ctx.
func WithPlatformTx(ctx context.Context, pool *pgxpool.Pool, fn TxFn) error {
	if st, ok := ctx.Value(txKey{}).(*txState); ok {
		return fn(ctx, st.tx)
	}
	return run(ctx, pool, uuid.Nil, fn)
}

// Lookup names a key-scoped RLS policy (migration 00009/00012). Unauthenticated platform
// paths that must find a tenant row before the tenant is known set exactly one lookup key;
// the matching policy then exposes only rows with that key (never a whole table).
type Lookup string

const (
	LookupUserID     Lookup = "app.lookup_user_id"     // tenant_members of one user (login, switchTenant)
	LookupSocialID   Lookup = "app.lookup_social_id"   // social_accounts by id (Telegram webhook route)
	LookupSocialKey  Lookup = "app.lookup_social_key"  // social_accounts by "platform:external_id" (Meta webhooks)
	LookupSlug       Lookup = "app.lookup_slug"        // storefronts by public slug
	LookupTokenHash  Lookup = "app.lookup_token_hash"  // portal_tokens by sha256(token)
	LookupInviteHash Lookup = "app.lookup_invite_hash" // tenant_members by sha256(invite token)
)

// WithLookup runs fn with one lookup key set (transaction-local). It joins a transaction
// carried by ctx (the key is cleared again when fn returns) or opens a platform one.
func WithLookup(ctx context.Context, pool *pgxpool.Pool, key Lookup, value string, fn TxFn) error {
	set := func(ctx context.Context, tx pgx.Tx, v string) error {
		if _, err := tx.Exec(ctx, `SELECT set_config($1, $2, true)`, string(key), v); err != nil {
			return fmt.Errorf("set lookup: %w", err)
		}
		return nil
	}
	body := func(ctx context.Context, tx pgx.Tx) (err error) {
		if err := set(ctx, tx, value); err != nil {
			return err
		}
		defer func() {
			if rerr := set(context.WithoutCancel(ctx), tx, ""); err == nil {
				err = rerr
			}
		}()
		return fn(ctx, tx)
	}
	if st, ok := ctx.Value(txKey{}).(*txState); ok {
		return body(ctx, st.tx)
	}
	return run(ctx, pool, uuid.Nil, body)
}

func run(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, fn TxFn) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if tenantID != uuid.Nil {
		if err = setTenant(ctx, tx, tenantID); err != nil {
			return err
		}
	}
	st := &txState{tx: tx, tenantID: tenantID}
	if err = fn(context.WithValue(ctx, txKey{}, st), tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func setTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID.String()); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	return nil
}

// TxFromContext returns the transaction carried by ctx, if any.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	st, ok := ctx.Value(txKey{}).(*txState)
	if !ok {
		return nil, false
	}
	return st.tx, true
}

// TenantFromContext returns the tenant of the transaction carried by ctx (uuid.Nil if none).
func TenantFromContext(ctx context.Context) uuid.UUID {
	if st, ok := ctx.Value(txKey{}).(*txState); ok {
		return st.tenantID
	}
	return uuid.Nil
}

// PlatformConn returns the transaction in ctx or the pool. Use it only for platform
// tables (jobs); tenant tables must go through WithTenantTx (RLS hides them otherwise).
func PlatformConn(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return pool
}

// NextNumber returns the next human-facing number of kind ("invoice", "ticket", "order")
// for a tenant. The counter row stays locked until the caller's transaction ends, so
// numbers are contiguous per tenant and a rollback leaves no gap (migration 00013).
func NextNumber(ctx context.Context, q Querier, tenantID uuid.UUID, kind string) (int64, error) {
	var n int64
	err := q.QueryRow(ctx, `INSERT INTO tenant_counters AS c (tenant_id, kind, value) VALUES ($1, $2, 1)
		ON CONFLICT (tenant_id, kind) DO UPDATE SET value = c.value + 1, updated_at = now()
		RETURNING value`, tenantID, kind).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("next %s number: %w", kind, err)
	}
	return n, nil
}

// Postgres error codes we translate.
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
	codeNotNullViolation    = "23502"
	codeInvalidText         = "22P02"
)

// constraintMsgs maps known constraint names to friendly field errors (SECURITY L5:
// raw constraint names and Postgres messages never reach clients).
var constraintMsgs = map[string]struct{ field, msg string }{
	"payments_trx_unique":                  {"trxId", "this TrxID has already been used for another payment"},
	"social_accounts_external_active_key":  {"externalId", "this account is already connected to another business"},
	"tenant_members_tenant_id_user_id_key": {"", "user is already a member of this business"},
	"storefronts_slug_key":                 {"slug", "this storefront address is already taken"},
	"tenants_slug_key":                     {"slug", "this address is already taken"},
	"users_email_key":                      {"email", "an account with this email already exists"},
	"users_phone_key":                      {"phone", "an account with this phone already exists"},
	"message_templates_tenant_id_key_key":  {"key", "a template with this key already exists"},
	"slots_master_account_id_slot_no_key":  {"slotNo", "this slot number already exists"},
	"subscriptions_check":                  {"expiresAt", "expiry must be after the start"},
}

// MapErr converts pgx/Postgres errors into domain errors. what names the entity for NotFound.
func MapErr(err error, what string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound(what)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	kind := apperr.ErrValidation
	switch pgErr.Code {
	case codeUniqueViolation:
		kind = apperr.ErrConflict
	case codeForeignKeyViolation, codeCheckViolation, codeNotNullViolation, codeInvalidText:
	default:
		return err
	}
	if m, ok := constraintMsgs[pgErr.ConstraintName]; ok {
		return &apperr.Error{Kind: kind, Msg: m.msg, Field: m.field}
	}
	switch pgErr.Code {
	case codeUniqueViolation:
		return &apperr.Error{Kind: kind, Msg: what + " already exists"}
	case codeForeignKeyViolation:
		return &apperr.Error{Kind: kind, Msg: "referenced record does not exist or is still in use"}
	default:
		return &apperr.Error{Kind: kind, Msg: "invalid " + what}
	}
}

// IsUniqueViolation reports whether err is a unique violation on the named constraint ("" = any).
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == codeUniqueViolation {
		return constraint == "" || pgErr.ConstraintName == constraint
	}
	return false
}

// Contains builds an ILIKE pattern matching s anywhere, with LIKE wildcards in s escaped
// (SECURITY I1). Backslash is Postgres' default LIKE escape character.
func Contains(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(strings.TrimSpace(s)) + "%"
}
