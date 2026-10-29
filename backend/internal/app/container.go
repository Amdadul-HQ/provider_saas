// Package app wires configuration, infrastructure and module services together.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/graph"
	"provider_saas/internal/modules/analytics"
	"provider_saas/internal/modules/auth"
	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/content"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/delivery"
	"provider_saas/internal/modules/inbox"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/reminder"
	"provider_saas/internal/modules/social"
	"provider_saas/internal/modules/storefront"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/modules/ticket"
	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/config"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/gql"
	"provider_saas/internal/platform/httpx"
	"provider_saas/internal/platform/jobs"
	"provider_saas/internal/platform/lockout"
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
	Social        social.Service
	Inbox         inbox.Service
	Content       content.Service
	Tickets       ticket.Service
	Reminders     reminder.Service
	Storefront    storefront.Service
	Delivery      delivery.Service

	// SocialClient is the Meta/Telegram HTTP client (a *social.Fake when META_APP_ID is empty).
	SocialClient social.Client
	// inline runs just-committed jobs in-process (sendMessage, publishPostNow).
	inline *jobs.Worker

	Resolver *graph.Resolver

	closers []func() error
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
	c.Tenants = tenant.NewService(pool, auth.NewUserDirectory(pool), v, cfg.PublicAppURL)
	idLimiter, ipLimiter := c.loginLimiters()
	c.Auth = auth.NewService(pool, c.Tokens, cfg.JWTRefreshTTL, c.Tenants, idLimiter, ipLimiter)
	c.Customers = customer.NewService(pool)
	c.Catalog = catalog.NewService(pool)
	c.Inventory = inventory.NewService(pool, c.Catalog, v)

	// social: real Graph/Bot API client, or the in-memory fake in dev/tests without a Meta app.
	if cfg.MetaAppID == "" {
		c.SocialClient = social.NewFake()
	} else {
		c.SocialClient = social.NewHTTPClient(cfg.MetaAppID, cfg.MetaAppSecret, cfg.MetaGraphVersion)
	}
	c.Social = social.NewService(pool, v, c.SocialClient, nil, nil, social.Config{AppID: cfg.MetaAppID, AppSecret: cfg.MetaAppSecret,
		GraphVersion: cfg.MetaGraphVersion, PublicAPIURL: cfg.PublicAPIURL, PublicAppURL: cfg.PublicAppURL, Secret: []byte(cfg.JWTSecret)})
	c.inline = jobs.NewWorker(pool, log)
	runInline := func(ctx context.Context, id int64) (bool, error) { return c.inline.RunByID(ctx, id) }
	c.Inbox = inbox.NewService(pool, c.Social, c.Customers, c.Tenants, c.Tenants, c.Queue, runInline, v)

	// billing notifies through the inbox and triggers delivery when an invoice is fully paid.
	c.Billing = billing.NewService(pool, c.Customers, c.Inbox, c.Tenants, billing.PaidHookFunc(c.onInvoicePaid))
	c.Subscriptions = subscription.NewService(pool, c.Customers, c.Catalog, c.Inventory, c.Billing, c.Tenants)
	c.Analytics = analytics.NewService(pool)
	c.Storefront = storefront.NewService(pool, storefront.Deps{Customers: c.Customers, Catalog: c.Catalog, Inventory: c.Inventory,
		Billing: c.Billing, Subscriptions: c.Subscriptions, Messenger: c.Inbox, Notifier: c.Tenants, Auditor: c.Tenants,
		PublicAppURL: cfg.PublicAppURL})
	c.Tickets = ticket.NewService(pool, ticket.Deps{Subscriptions: c.Subscriptions, Slots: c.Inventory, Customers: c.Customers,
		Catalog: c.Catalog, Members: c.Tenants, Messenger: c.Inbox, Queue: c.Queue})
	c.Reminders = reminder.NewService(pool, reminder.Deps{Messenger: c.Inbox, Templates: c.Inbox, Portal: c.Storefront,
		Tenants: c.Tenants, Subscriptions: c.Subscriptions})
	c.Content = content.NewService(pool, content.Deps{Social: c.Social, Catalog: c.Catalog, Tenants: c.Tenants, Queue: c.Queue,
		Inline: runInline, UploadDir: cfg.UploadDir, PublicAPIURL: cfg.PublicAPIURL})
	c.Delivery = delivery.NewService(delivery.Deps{Subscriptions: c.Subscriptions, Catalog: c.Catalog, Customers: c.Customers,
		Inventory: c.Inventory, Secrets: v, Tenants: c.Tenants, Messenger: c.Inbox, Orders: c.Storefront})
	c.RegisterJobs(c.inline)

	c.Resolver = &graph.Resolver{
		AuthSvc: c.Auth, TenantSvc: c.Tenants, CustomerSvc: c.Customers, CatalogSvc: c.Catalog,
		InventorySvc: c.Inventory, SubscriptionSvc: c.Subscriptions, BillingSvc: c.Billing, AnalyticsSvc: c.Analytics,
		SocialSvc: c.Social, InboxSvc: c.Inbox, ContentSvc: c.Content, TicketSvc: c.Tickets, ReminderSvc: c.Reminders,
		StorefrontSvc: c.Storefront,
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
		TrustedProxyCIDRs: c.Cfg.TrustedProxyCIDRs,
		Mount:             c.mountREST,
	})
}

// mountREST registers webhooks (ADR-003) and the static /uploads route.
func (c *Container) mountREST(r chi.Router) {
	social.NewWebhooks(social.WebhookDeps{
		Social:       c.Social,
		Ingest:       func(ctx context.Context, m social.InboundMessage) error { _, err := c.Inbox.Ingest(ctx, m); return err },
		AppSecret:    c.Cfg.MetaAppSecret,
		VerifyToken:  c.Cfg.MetaVerifyToken,
		PublicAppURL: c.Cfg.PublicAppURL,
		SMSTenant:    c.Tenants.VerifySMSWebhookToken,
		SMSIngest: func(ctx context.Context, tid uuid.UUID, from, raw string, at time.Time) error {
			_, err := c.Billing.IngestSMS(ctx, tid, from, raw, at)
			return err
		},
		Log: c.Log,
	}).Mount(r)
	files := http.StripPrefix("/uploads/", http.FileServer(http.Dir(c.Cfg.UploadDir)))
	r.Get("/uploads/*", func(w http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/") { // no directory listings
			http.NotFound(w, req)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		files.ServeHTTP(w, req)
	})
}

// onInvoicePaid (billing.PaidHook, inside the payment tx) queues delivery.send for every
// live, undelivered subscription on the invoice whose product has a delivery template.
func (c *Container) onInvoicePaid(ctx context.Context, tid, invoiceID uuid.UUID) error {
	items, err := c.Billing.ItemsByInvoices(ctx, tid, []uuid.UUID{invoiceID})
	if err != nil {
		return err
	}
	for _, it := range items[invoiceID] {
		if it.SubscriptionID == nil {
			continue
		}
		sub, err := c.Subscriptions.Get(ctx, tid, *it.SubscriptionID)
		if err != nil {
			return err
		}
		if sub.DeliveredAt != nil || !subscription.IsLive(sub.Status) {
			continue
		}
		p, err := c.Catalog.Get(ctx, tid, sub.ProductID)
		if err != nil {
			return err
		}
		if p.DeliveryTemplate == nil || strings.TrimSpace(*p.DeliveryTemplate) == "" {
			continue
		}
		if err := delivery.Enqueue(ctx, c.Queue, tid, sub.ID, "invoice paid"); err != nil {
			return err
		}
	}
	return nil
}

// loginLimiters returns the login lockout limiters (SECURITY H6): shared through Redis
// when REDIS_URL is reachable, otherwise bounded in-memory LRUs (per process).
func (c *Container) loginLimiters() (id, ip lockout.Limiter) {
	if c.Cfg.RedisURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		rid, err1 := lockout.NewRedis(ctx, c.Cfg.RedisURL, lockout.DefaultLogin)
		rip, err2 := lockout.NewRedis(ctx, c.Cfg.RedisURL, auth.IPPolicy)
		if err1 == nil && err2 == nil {
			c.closers = append(c.closers, rid.Close, rip.Close)
			return rid, rip
		}
		c.Log.Warn("login limiter: redis unavailable, using in-memory limiter", "err", errors.Join(err1, err2))
	}
	return lockout.NewMemory(lockout.DefaultLogin, 100_000), lockout.NewMemory(auth.IPPolicy, 100_000)
}

// Close releases resources.
func (c *Container) Close() {
	for _, f := range c.closers {
		_ = f()
	}
	c.Pool.Close()
}
