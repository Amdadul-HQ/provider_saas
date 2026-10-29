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

	// Scheduler: subscription.expire_check + reminder.plan hourly, reminder.dispatch every
	// minute. Dedupe keys (per hour / per minute) keep several worker replicas from
	// double-scheduling.
	go func() {
		schedule := func() {
			now := time.Now()
			if err := c.ScheduleExpireCheck(ctx, now); err != nil && ctx.Err() == nil {
				log.Error("worker: schedule expire_check", "err", err)
			}
			if err := c.ScheduleReminders(ctx, now); err != nil && ctx.Err() == nil {
				log.Error("worker: schedule reminders", "err", err)
			}
		}
		schedule()
		t := time.NewTicker(time.Minute)
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
