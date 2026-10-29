// Package app wires configuration, infrastructure and module services together.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/graph"
	"provider_saas/internal/modules/analytics"
	"provider_saas/internal/modules/auth"
	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/tenant"
	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/config"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/gql"
	"provider_saas/internal/platform/httpx"
	"provider_saas/internal/platform/jobs"
	"provider_saas/internal/platform/vault"
)

// Container holds every long-lived dependency of the process.
type Container struct {
	Cfg    config.Config
	Log    *slog.Logger
	Pool   *pgxpool.Pool
	Tokens *pauth.TokenIssuer
	Vault  *vault.Vault
	Queue  *jobs.Queue

	Auth          auth.Service
	Tenants       tenant.Service
	Customers     customer.Service
	Catalog       catalog.Service
	Inventory     inventory.Service
	Subscriptions subscription.Service
	Billing       billing.Service
	Analytics     analytics.Service

	Resolver *graph.Resolver
}

// New connects to Postgres and builds all services.
func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*Container, error) {
	pool, err := db.Connect(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		return nil, err
	}
	c, err := NewWithPool(cfg, log, pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return c, nil
}

// NewWithPool builds services on an existing pool (used by tests).
func NewWithPool(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) (*Container, error) {
	v, err := vault.New(pool, cfg.VaultKey, cfg.VaultKeyVersion)
	if err != nil {
		return nil, fmt.Errorf("vault: %w", err)
	}
	c := &Container{Cfg: cfg, Log: log, Pool: pool, Vault: v, Queue: jobs.NewQueue(pool),
		Tokens: pauth.NewTokenIssuer(cfg.JWTSecret, cfg.JWTAccessTTL)}

	// auth <-> tenant: tenant gets a user directory that does not depend on tenant.
	c.Tenants = tenant.NewService(pool, auth.NewUserDirectory(pool))
	c.Auth = auth.NewService(pool, c.Tokens, cfg.JWTRefreshTTL, c.Tenants)
	c.Customers = customer.NewService(pool)
	c.Catalog = catalog.NewService(pool)
	c.Inventory = inventory.NewService(pool, c.Catalog, v)
	// Notifier is nil until the inbox module provides inbox.Sender.
	c.Billing = billing.NewService(pool, c.Customers, nil)
	c.Subscriptions = subscription.NewService(pool, c.Customers, c.Catalog, c.Inventory, c.Billing)
	c.Analytics = analytics.NewService(pool)

	c.Resolver = &graph.Resolver{
		AuthSvc: c.Auth, TenantSvc: c.Tenants, CustomerSvc: c.Customers, CatalogSvc: c.Catalog,
		InventorySvc: c.Inventory, SubscriptionSvc: c.Subscriptions, BillingSvc: c.Billing, AnalyticsSvc: c.Analytics,
	}
	return c, nil
}

// HTTPHandler builds the API router.
func (c *Container) HTTPHandler() http.Handler {
	srv := gql.NewServer(graph.NewExecutableSchema(c.Resolver), gql.Options{
		Dev: c.Cfg.IsDev(), ComplexityLimit: c.Cfg.ComplexityLimit, Logger: c.Log,
	})
	return httpx.NewRouter(httpx.Deps{
		Logger: c.Log, Pool: c.Pool, GraphQL: srv,
		Auth:         pauth.Middleware(c.Tokens),
		PerRequest:   c.Resolver.Middleware,
		CORSOrigins:  c.Cfg.CORSOrigins,
		Dev:          c.Cfg.IsDev(),
		RateLimitRPS: c.Cfg.RateLimitRPS, RateBurst: c.Cfg.RateLimitBurst,
	})
}

// Close releases resources.
func (c *Container) Close() { c.Pool.Close() }
