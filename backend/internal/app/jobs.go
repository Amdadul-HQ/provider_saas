package app

import (
	"context"
	"fmt"
	"time"

	"provider_saas/internal/platform/jobs"
)

// Job kinds handled by cmd/worker.
const (
	JobExpireCheck = "subscription.expire_check"
)

// RegisterJobs binds handlers to the worker.
func (c *Container) RegisterJobs(w *jobs.Worker) {
	w.Register(JobExpireCheck, c.expireCheck)
}

// expireCheck flips subscription statuses (active -> expiring (<=3d) -> expired), frees
// slots of expired subscriptions, refreshes master-account expiry statuses and marks
// overdue invoices — per tenant, each in its own tenant transaction.
func (c *Container) expireCheck(ctx context.Context, _ jobs.Job) error {
	now := time.Now()
	tenants, err := c.Tenants.TenantIDs(ctx)
	if err != nil {
		return err
	}
	var failed int
	for _, tid := range tenants {
		res, err := c.Subscriptions.ExpireCheck(ctx, tid, now)
		if err != nil {
			failed++
			c.Log.Error("expire_check: subscriptions", "tenant", tid, "err", err)
			continue
		}
		masters, err := c.Inventory.RefreshExpiryStatuses(ctx, tid, now)
		if err != nil {
			failed++
			c.Log.Error("expire_check: master accounts", "tenant", tid, "err", err)
		}
		overdue, err := c.Billing.MarkOverdue(ctx, tid, now)
		if err != nil {
			failed++
			c.Log.Error("expire_check: invoices", "tenant", tid, "err", err)
		}
		if res.Expired+res.Expiring+res.Activated+masters+overdue > 0 {
			c.Log.Info("expire_check", "tenant", tid, "expired", res.Expired, "expiring", res.Expiring,
				"activated", res.Activated, "slots_freed", res.SlotsFreed, "masters_changed", masters, "invoices_overdue", overdue)
		}
	}
	if failed > 0 {
		return fmt.Errorf("expire_check failed for %d tenant step(s)", failed)
	}
	return nil
}

// ScheduleExpireCheck enqueues one expire_check per hour bucket (dedupe key).
func (c *Container) ScheduleExpireCheck(ctx context.Context, now time.Time) error {
	key := JobExpireCheck + ":" + now.UTC().Format("2006-01-02T15")
	_, err := c.Queue.Enqueue(ctx, JobExpireCheck, map[string]any{"scheduled_for": now.UTC()}, now, key)
	return err
}
