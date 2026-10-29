// Package jobs is the Postgres-backed job queue (ADR-006). Producers call Enqueue
// (inside their own transaction when ctx carries one); cmd/worker runs a Worker that
// claims due jobs with FOR UPDATE SKIP LOCKED and retries failures with backoff.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/db"
)

// Job is a claimed job.
type Job struct {
	ID          int64
	TenantID    *uuid.UUID
	Kind        string
	Payload     json.RawMessage
	Attempts    int
	MaxAttempts int
	RunAt       time.Time
}

// SealedPayloadKey is the payload field that carries a vault-sealed transient secret
// (vault.SealTransient). The worker deletes it as soon as the job finishes, successfully
// or for good, so no ciphertext of a delivered credential lingers in the jobs table.
const SealedPayloadKey = "sealed_body"

// Decode unmarshals the payload.
func (j Job) Decode(v any) error { return json.Unmarshal(j.Payload, v) }

// Handler processes a job; returning an error schedules a retry.
type Handler func(ctx context.Context, job Job) error

// Queue enqueues jobs.
type Queue struct{ pool *pgxpool.Pool }

func NewQueue(pool *pgxpool.Pool) *Queue { return &Queue{pool: pool} }

// Enqueue inserts a job. A non-empty dedupeKey makes the call idempotent while a job
// with the same key is pending (returns id 0 when deduplicated). The tenant is taken
// from the tenant transaction in ctx, if any.
func (q *Queue) Enqueue(ctx context.Context, kind string, payload any, runAt time.Time, dedupeKey string) (int64, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("jobs: marshal payload: %w", err)
	}
	if runAt.IsZero() {
		runAt = time.Now()
	}
	var tenant *uuid.UUID
	if t := db.TenantFromContext(ctx); t != uuid.Nil {
		tenant = &t
	}
	var dk *string
	if dedupeKey != "" {
		dk = &dedupeKey
	}
	var id int64
	err = db.PlatformConn(ctx, q.pool).QueryRow(ctx, `
		INSERT INTO jobs (tenant_id, kind, payload, run_at, dedupe_key)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL AND done_at IS NULL DO NOTHING
		RETURNING id`, tenant, kind, body, runAt, dk).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// Cancel finishes a pending (not running) job by dedupe key so the key can be reused,
// e.g. when a scheduled post is rescheduled. Reports whether a job was cancelled.
func (q *Queue) Cancel(ctx context.Context, dedupeKey string) (bool, error) {
	tag, err := db.PlatformConn(ctx, q.pool).Exec(ctx, `UPDATE jobs SET done_at = now(), last_error = 'cancelled'
		WHERE dedupe_key = $1 AND done_at IS NULL AND locked_at IS NULL`, dedupeKey)
	return tag.RowsAffected() > 0, err
}

// Worker polls and runs jobs.
type Worker struct {
	pool         *pgxpool.Pool
	log          *slog.Logger
	mu           sync.RWMutex
	handlers     map[string]Handler
	ID           string
	PollInterval time.Duration
	LockTimeout  time.Duration
	JobTimeout   time.Duration
	Concurrency  int
}

func NewWorker(pool *pgxpool.Pool, log *slog.Logger) *Worker {
	host, _ := os.Hostname()
	return &Worker{
		pool: pool, log: log, handlers: map[string]Handler{},
		ID:           fmt.Sprintf("%s-%d", host, os.Getpid()),
		PollInterval: 2 * time.Second, LockTimeout: 10 * time.Minute, JobTimeout: 5 * time.Minute, Concurrency: 4,
	}
}

// Register binds a handler to a job kind.
func (w *Worker) Register(kind string, h Handler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[kind] = h
}

func (w *Worker) handler(kind string) (Handler, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	h, ok := w.handlers[kind]
	return h, ok
}

// Run polls until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for i := 0; i < max(1, w.Concurrency); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				ran, err := w.RunOnce(ctx)
				if err != nil && ctx.Err() == nil {
					w.log.Error("jobs: poll failed", "err", err)
				}
				if !ran {
					select {
					case <-ctx.Done():
					case <-time.After(w.PollInterval):
					}
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

// RunOnce claims and executes at most one due job. It reports whether a job ran.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	return w.claimAndRun(ctx, `
			SELECT id FROM jobs
			WHERE done_at IS NULL AND run_at <= now()
			  AND (locked_at IS NULL OR locked_at < now() - $2::interval)
			ORDER BY run_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1`)
}

// RunByID claims one specific due job and executes it now (used to run a just-committed
// job inline, e.g. an outbound message, instead of waiting for the poller). It is safe
// against a concurrent worker: whoever claims the row first runs it, the other skips.
func (w *Worker) RunByID(ctx context.Context, id int64) (bool, error) {
	return w.claimAndRun(ctx, `
			SELECT id FROM jobs
			WHERE id = $3 AND done_at IS NULL AND run_at <= now()
			  AND (locked_at IS NULL OR locked_at < now() - $2::interval)
			FOR UPDATE SKIP LOCKED`, id)
}

func (w *Worker) claimAndRun(ctx context.Context, pick string, extra ...any) (bool, error) {
	var j Job
	args := append([]any{w.ID, fmt.Sprintf("%d seconds", int(w.LockTimeout.Seconds()))}, extra...)
	err := w.pool.QueryRow(ctx, `
		UPDATE jobs SET locked_at = now(), locked_by = $1, attempts = attempts + 1
		WHERE id = (`+pick+`)
		RETURNING id, tenant_id, kind, payload, attempts, max_attempts, run_at`, args...).
		Scan(&j.ID, &j.TenantID, &j.Kind, &j.Payload, &j.Attempts, &j.MaxAttempts, &j.RunAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	runErr := w.execute(ctx, j)
	bg := context.WithoutCancel(ctx)
	if runErr == nil {
		_, err = w.pool.Exec(bg, `UPDATE jobs SET done_at = now(), locked_at = NULL, last_error = NULL, payload = payload - $2 WHERE id = $1`,
			j.ID, SealedPayloadKey)
		return true, err
	}
	log := w.log.With("job_id", j.ID, "kind", j.Kind, "attempt", j.Attempts)
	if j.Attempts >= j.MaxAttempts {
		log.Error("jobs: giving up", "err", runErr)
		_, err = w.pool.Exec(bg, `UPDATE jobs SET done_at = now(), locked_at = NULL, last_error = $2, payload = payload - $3 WHERE id = $1`,
			j.ID, "dead: "+runErr.Error(), SealedPayloadKey)
		return true, err
	}
	next := time.Now().Add(Backoff(j.Attempts))
	log.Warn("jobs: failed, retrying", "err", runErr, "next_run", next)
	_, err = w.pool.Exec(bg, `UPDATE jobs SET run_at = $2, locked_at = NULL, locked_by = NULL, last_error = $3 WHERE id = $1`,
		j.ID, next, runErr.Error())
	return true, err
}

func (w *Worker) execute(ctx context.Context, j Job) (err error) {
	h, ok := w.handler(j.Kind)
	if !ok {
		return fmt.Errorf("no handler for job kind %q", j.Kind)
	}
	ctx, cancel := context.WithTimeout(ctx, w.JobTimeout)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return h(ctx, j)
}

// Backoff returns the retry delay after the given attempt: 10s·2^(n-1) capped at 1h, ±20% jitter.
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := 10 * time.Second << min(attempt-1, 10)
	if d > time.Hour {
		d = time.Hour
	}
	jitter := time.Duration(rand.Int64N(int64(d)/5+1)) - d/10
	return d + jitter
}
