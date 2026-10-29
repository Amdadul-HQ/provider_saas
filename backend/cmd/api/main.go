// Command api runs the GraphQL API.
//
//	api [serve]   start the HTTP server (auto-migrates in development)
//	api migrate   apply embedded goose migrations
//	api seed      create the demo tenant (demo@provider.test / Demo1234!)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"provider_saas/internal/app"
	"provider_saas/internal/platform/config"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/httpx"
	"provider_saas/internal/platform/logger"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(2)
	}
	log := logger.New(cfg.AppEnv)
	slog.SetDefault(log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "serve":
		err = serve(ctx, cfg, log)
	case "migrate":
		err = db.Migrate(ctx, cfg.DatabaseURL, log)
	case "seed":
		err = seed(ctx, cfg, log)
	default:
		err = fmt.Errorf("unknown command %q (want serve|migrate|seed)", cmd)
	}
	if err != nil {
		log.Error("fatal", "cmd", cmd, "err", err)
		os.Exit(1)
	}
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	if cfg.IsDev() {
		if err := db.Migrate(ctx, cfg.DatabaseURL, log); err != nil {
			return err
		}
	}
	c, err := app.New(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer c.Close()
	srv := httpx.NewServer(cfg.HTTPAddr, c.HTTPHandler())
	errc := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}

func seed(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	if err := db.Migrate(ctx, cfg.DatabaseURL, log); err != nil {
		return err
	}
	c, err := app.New(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer c.Close()
	res, err := c.Seed(ctx)
	if err != nil {
		return err
	}
	if res.Skipped {
		log.Info("seed: demo tenant already exists", "tenant", res.TenantID, "login", app.DemoEmail)
	} else {
		log.Info("seed: done", "tenant", res.TenantID, "products", res.Products, "master_accounts", res.MasterAccts,
			"customers", res.Customers, "subscriptions", res.Subscriptions, "login", app.DemoEmail, "password", app.DemoPassword)
	}
	e := res.Engagement
	if e.Skipped {
		log.Info("seed: engagement data already exists")
		return nil
	}
	log.Info("seed: engagement", "social_accounts", e.SocialAccounts, "conversations", e.Conversations, "messages", e.Messages,
		"templates", e.Templates, "tickets", e.Tickets, "orders", e.Orders, "reminders", e.Reminders, "posts", e.Posts,
		"storefront", app.DemoStoreSlug)
	if e.SMSToken != "" && cfg.IsDev() {
		// Shown once (dev only): the SMS-forwarder bearer token for /webhooks/payments/sms.
		log.Info("seed: sms forwarder token (shown once)", "token", e.SMSToken)
	}
	return nil
}
