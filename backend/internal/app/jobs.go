package app

import (
	"context"
	"fmt"
	"time"

	"provider_saas/internal/modules/content"
	"provider_saas/internal/modules/delivery"
	"provider_saas/internal/modules/inbox"
	"provider_saas/internal/modules/reminder"
	"provider_saas/internal/platform/jobs"
)

// Job kinds handled by cmd/worker.
const (
	JobExpireCheck      = "subscription.expire_check"
	JobInboxSend        = inbox.JobSend
	JobPostPublish      = content.JobPublish
	JobDeliverySend     = delivery.JobSend
	JobReminderPlan     = reminder.JobPlan
	JobReminderDispatch = reminder.JobDispatch
)

// RegisterJobs binds handlers to the worker.
func (c *Container) RegisterJobs(w *jobs.Worker) {
	w.Register(JobExpireCheck, c.expireCheck)
	w.Register(JobInboxSend, func(ctx context.Context, j jobs.Job) error {
		var p inbox.SendPayload
		if err := j.Decode(&p); err != nil {
			return err
		}
		return c.Inbox.Deliver(ctx, p, j.Attempts >= j.MaxAttempts)
	})
	w.Register(JobPostPublish, func(ctx context.Context, j jobs.Job) error {
		var p content.PublishPayload
		if err := j.Decode(&p); err != nil {
			return err
		}
		return c.Content.Publish(ctx, p)
	})
	w.Register(JobDeliverySend, func(ctx context.Context, j jobs.Job) error {
		var p delivery.Payload
		if err := j.Decode(&p); err != nil {
			return err
		}
		return c.Delivery.Deliver(ctx, p)
	})
	w.Register(JobReminderPlan, c.reminderPlan)
	w.Register(JobReminderDispatch, c.reminderDispatch)
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

// reminderPlan materialises reminders for every active tenant.
func (c *Container) reminderPlan(ctx context.Context, _ jobs.Job) error {
	now := time.Now()
	tenants, err := c.Tenants.TenantIDs(ctx)
	if err != nil {
		return err
	}
	failed := 0
	for _, tid := range tenants {
		n, err := c.Reminders.Plan(ctx, tid, now)
		if err != nil {
			failed++
			c.Log.Error("reminder.plan", "tenant", tid, "err", err)
			continue
		}
		if n > 0 {
			c.Log.Info("reminder.plan", "tenant", tid, "created", n)
		}
	}
	if failed > 0 {
		return fmt.Errorf("reminder.plan failed for %d tenant(s)", failed)
	}
	return nil
}

// reminderDispatch sends due reminders, tenant by tenant.
func (c *Container) reminderDispatch(ctx context.Context, _ jobs.Job) error {
	now := time.Now()
	tenants, err := c.Reminders.TenantsWithDue(ctx, now)
	if err != nil {
		return err
	}
	failed := 0
	for _, tid := range tenants {
		sent, nfail, err := c.Reminders.Dispatch(ctx, tid, now)
		if err != nil {
			failed++
			c.Log.Error("reminder.dispatch", "tenant", tid, "err", err)
			continue
		}
		if sent+nfail > 0 {
			c.Log.Info("reminder.dispatch", "tenant", tid, "sent", sent, "failed", nfail)
		}
	}
	if failed > 0 {
		return fmt.Errorf("reminder.dispatch failed for %d tenant(s)", failed)
	}
	return nil
}

// ScheduleExpireCheck enqueues one expire_check per hour bucket (dedupe key).
func (c *Container) ScheduleExpireCheck(ctx context.Context, now time.Time) error {
	key := JobExpireCheck + ":" + now.UTC().Format("2006-01-02T15")
	_, err := c.Queue.Enqueue(ctx, JobExpireCheck, map[string]any{"scheduled_for": now.UTC()}, now, key)
	return err
}

// ScheduleReminders enqueues reminder.plan once per hour and reminder.dispatch once per
// minute (dedupe keys make it safe to call from every worker replica every minute).
func (c *Container) ScheduleReminders(ctx context.Context, now time.Time) error {
	u := now.UTC()
	if _, err := c.Queue.Enqueue(ctx, JobReminderPlan, map[string]any{"scheduled_for": u}, now,
		JobReminderPlan+":"+u.Format("2006-01-02T15")); err != nil {
		return err
	}
	_, err := c.Queue.Enqueue(ctx, JobReminderDispatch, map[string]any{"scheduled_for": u}, now,
		JobReminderDispatch+":"+u.Format("2006-01-02T15:04"))
	return err
}
