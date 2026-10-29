// Command worker runs background jobs (ADR-006) and schedules periodic ones.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"provider_saas/internal/app"
	"provider_saas/internal/platform/config"
	"provider_saas/internal/platform/jobs"
	"provider_saas/internal/platform/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(2)
	}
	log := logger.New(cfg.AppEnv)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Error("worker: init", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	w := jobs.NewWorker(c.Pool, log)
	c.RegisterJobs(w)

	// Scheduler: enqueue subscription.expire_check now and every hour (deduped per hour,
	// so several worker replicas do not double-schedule).
	go func() {
		schedule := func() {
			if err := c.ScheduleExpireCheck(ctx, time.Now()); err != nil && ctx.Err() == nil {
				log.Error("worker: schedule expire_check", "err", err)
			}
		}
		schedule()
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				schedule()
			}
		}
	}()

	log.Info("worker started", "id", w.ID)
	if err := w.Run(ctx); err != nil {
		log.Error("worker stopped", "err", err)
		os.Exit(1)
	}
	log.Info("worker stopped")
}
