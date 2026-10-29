// Package db owns the pgx pool, tenant-scoped transactions (ADR-002) and migrations.
package db

import (
	"context"
	"errors"
	"fmt"
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

// WithTx runs fn in a transaction without tenant scope (users, tokens, platform jobs).
// Reuses an outer transaction from ctx.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn TxFn) error {
	if st, ok := ctx.Value(txKey{}).(*txState); ok {
		return fn(ctx, st.tx)
	}
	return run(ctx, pool, uuid.Nil, fn)
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

// Conn returns the transaction in ctx or the pool.
func Conn(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return pool
}

// Postgres error codes we translate.
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
	codeNotNullViolation    = "23502"
	codeInvalidText         = "22P02"
)

// MapErr converts pgx/Postgres errors into domain errors. what names the entity for NotFound.
func MapErr(err error, what string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound(what)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case codeUniqueViolation:
			return &apperr.Error{Kind: apperr.ErrConflict, Msg: what + " already exists (" + pgErr.ConstraintName + ")"}
		case codeForeignKeyViolation:
			return &apperr.Error{Kind: apperr.ErrValidation, Msg: "referenced record does not exist or is still in use (" + pgErr.ConstraintName + ")"}
		case codeCheckViolation, codeNotNullViolation, codeInvalidText:
			return &apperr.Error{Kind: apperr.ErrValidation, Msg: "invalid " + what + ": " + pgErr.Message}
		}
	}
	return err
}

// IsUniqueViolation reports whether err is a unique violation on the named constraint ("" = any).
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == codeUniqueViolation {
		return constraint == "" || pgErr.ConstraintName == constraint
	}
	return false
}
