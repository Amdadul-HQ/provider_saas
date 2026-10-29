package content

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/social"
	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
)

// Dependencies (wired in app.Container).
type (
	Social interface {
		GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]social.Account, error)
		Publish(ctx context.Context, a social.Account, body string, media []social.Attachment) (string, error)
	}
	Catalog interface {
		List(ctx context.Context, tenantID uuid.UUID, f catalog.Filter, limit int) ([]catalog.Product, error)
		PlansByProducts(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]catalog.Plan, error)
	}
	Tenants interface {
		Get(ctx context.Context, tenantID uuid.UUID) (tenant.Tenant, error)
	}
	Queue interface {
		Enqueue(ctx context.Context, kind string, payload any, runAt time.Time, dedupeKey string) (int64, error)
		Cancel(ctx context.Context, dedupeKey string) (bool, error)
	}
	InlineRunner func(ctx context.Context, jobID int64) (bool, error)
)

// PublishPayload is the post.publish job payload.
type PublishPayload struct {
	TenantID uuid.UUID `json:"tenant_id"`
	PostID   uuid.UUID `json:"post_id"`
}

type Deps struct {
	Social       Social
	Catalog      Catalog
	Tenants      Tenants
	Queue        Queue
	Inline       InlineRunner
	UploadDir    string // local directory served at /uploads
	PublicAPIURL string // base URL of /uploads
}

// Service is the content studio API.
type Service interface {
	Create(ctx context.Context, tenantID, actorID uuid.UUID, in PostInput) (Post, error)
	Update(ctx context.Context, tenantID, id uuid.UUID, in PostInput) (Post, error)
	PublishNow(ctx context.Context, tenantID, id uuid.UUID) (Post, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (Post, error)
	List(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) ([]Post, error)
	TargetsByPosts(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Target, error)
	// Publish is the post.publish job.
	Publish(ctx context.Context, p PublishPayload) error
	// GeneratePriceList renders a PNG of active products and plans; returns its public URL.
	GeneratePriceList(ctx context.Context, tenantID uuid.UUID, format string) (string, error)
}

type service struct {
	pool *pgxpool.Pool
	repo Repository
	d    Deps
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, d Deps) Service {
	if d.UploadDir == "" {
		d.UploadDir = "./uploads"
	}
	return &service{pool: pool, d: d, now: time.Now}
}

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func jobKey(id uuid.UUID) string { return JobPublish + ":" + id.String() }

func (s *service) checkAccounts(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error {
	accts, err := s.d.Social.GetByIDs(ctx, tenantID, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		a, ok := accts[id]
		switch {
		case !ok:
			return apperr.ValidationField("socialAccountIds", "unknown social account "+id.String())
		case a.Status == social.StatusDisconnected:
			return apperr.ValidationField("socialAccountIds", a.Name+" is disconnected")
		case a.Platform == social.PlatformWhatsApp:
			return apperr.ValidationField("socialAccountIds", "WhatsApp numbers cannot publish posts")
		}
	}
	return nil
}

// schedule (re)queues the publish job for a scheduled post (joins the tx).
func (s *service) schedule(ctx context.Context, tenantID uuid.UUID, p Post) (int64, error) {
	if _, err := s.d.Queue.Cancel(ctx, jobKey(p.ID)); err != nil {
		return 0, err
	}
	if p.Status != StatusScheduled && p.Status != StatusPublishing {
		return 0, nil
	}
	at := s.now()
	if p.Status == StatusScheduled && p.ScheduledAt != nil {
		at = *p.ScheduledAt
	}
	return s.d.Queue.Enqueue(ctx, JobPublish, PublishPayload{TenantID: tenantID, PostID: p.ID}, at, jobKey(p.ID))
}

func (s *service) Create(ctx context.Context, tenantID, actorID uuid.UUID, in PostInput) (p Post, err error) {
	if err := in.Validate(s.now()); err != nil {
		return p, err
	}
	status := StatusDraft
	if in.ScheduledAt != nil {
		status = StatusScheduled
	}
	var by *uuid.UUID
	if actorID != uuid.Nil {
		by = &actorID
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.checkAccounts(ctx, tenantID, in.SocialAccountIDs); err != nil {
			return err
		}
		if p, err = s.repo.Insert(ctx, tx, tenantID, in, status, by); err != nil {
			return err
		}
		if err := s.repo.ReplaceTargets(ctx, tx, tenantID, p.ID, in.SocialAccountIDs); err != nil {
			return err
		}
		_, err = s.schedule(ctx, tenantID, p)
		return err
	})
	return p, err
}

func (s *service) Update(ctx context.Context, tenantID, id uuid.UUID, in PostInput) (p Post, err error) {
	if err := in.Validate(s.now()); err != nil {
		return p, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.Get(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if !cur.Editable() {
			return apperr.Validation("a " + cur.Status + " post cannot be edited")
		}
		if err := s.checkAccounts(ctx, tenantID, in.SocialAccountIDs); err != nil {
			return err
		}
		status := StatusDraft
		if in.ScheduledAt != nil {
			status = StatusScheduled
		}
		if p, err = s.repo.Update(ctx, tx, tenantID, id, in, status); err != nil {
			return err
		}
		if err := s.repo.ReplaceTargets(ctx, tx, tenantID, id, in.SocialAccountIDs); err != nil {
			return err
		}
		_, err = s.schedule(ctx, tenantID, p)
		return err
	})
	return p, err
}

func (s *service) PublishNow(ctx context.Context, tenantID, id uuid.UUID) (p Post, err error) {
	ownTx := !inTx(ctx)
	var jobID int64
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.Get(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if cur.Status == StatusPublishing || cur.Status == StatusPublished {
			return apperr.Validation("post is already " + cur.Status)
		}
		if p, err = s.repo.SetStatus(ctx, tx, tenantID, id, StatusPublishing); err != nil {
			return err
		}
		jobID, err = s.schedule(ctx, tenantID, p)
		return err
	})
	if err != nil || !ownTx || s.d.Inline == nil || jobID == 0 {
		return p, err
	}
	if _, err := s.d.Inline(ctx, jobID); err == nil {
		if cur, err := s.Get(ctx, tenantID, id); err == nil {
			p = cur
		}
	}
	return p, nil
}

func inTx(ctx context.Context) bool {
	_, ok := db.TxFromContext(ctx)
	return ok
}

func (s *service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.Get(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if cur.Status == StatusPublishing {
			return apperr.Validation("post is being published; try again in a moment")
		}
		if _, err := s.d.Queue.Cancel(ctx, jobKey(id)); err != nil {
			return err
		}
		return s.repo.Delete(ctx, tx, tenantID, id)
	})
}

func (s *service) Get(ctx context.Context, tenantID, id uuid.UUID) (p Post, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		p, err = s.repo.Get(ctx, tx, tenantID, id, false)
		return err
	})
	return p, err
}

func (s *service) List(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) (out []Post, err error) {
	if f.Status != nil {
		v := strings.ToLower(*f.Status)
		f.Status = &v
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.repo.List(ctx, tx, tenantID, f, page)
		out, _ = pagination.Trim(rows, page.First)
		return err
	})
	return out, err
}

func (s *service) TargetsByPosts(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Target, error) {
	out := map[uuid.UUID][]Target{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ts, err := s.repo.TargetsByPosts(ctx, tx, tenantID, ids)
		for _, t := range ts {
			out[t.PostID] = append(out[t.PostID], t)
		}
		return err
	})
	return out, err
}

// Publish posts to every target that is not yet published (HTTP calls happen outside
// any DB transaction) and records per-target and overall status.
func (s *service) Publish(ctx context.Context, pl PublishPayload) error {
	tid := pl.TenantID
	var post Post
	var targets []Target
	err := s.tx(ctx, tid, func(ctx context.Context, tx pgx.Tx) error {
		p, err := s.repo.Get(ctx, tx, tid, pl.PostID, true)
		if err != nil {
			return err
		}
		switch {
		case p.Status == StatusScheduled && p.ScheduledAt != nil && p.ScheduledAt.After(s.now().Add(time.Minute)):
			return errSkip // rescheduled later: a newer job exists
		case p.Status != StatusScheduled && p.Status != StatusPublishing:
			return errSkip
		}
		if post, err = s.repo.SetStatus(ctx, tx, tid, p.ID, StatusPublishing); err != nil {
			return err
		}
		targets, err = s.repo.TargetsByPosts(ctx, tx, tid, []uuid.UUID{p.ID})
		return err
	})
	if errors.Is(err, errSkip) || errors.Is(err, apperr.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(targets))
	for i, t := range targets {
		ids[i] = t.SocialAccountID
	}
	accts, err := s.d.Social.GetByIDs(ctx, tid, ids)
	if err != nil {
		return err
	}
	for i, t := range targets {
		if t.Status == TargetPublished {
			continue
		}
		a, ok := accts[t.SocialAccountID]
		var ext string
		var perr error
		if !ok {
			perr = apperr.Validation("social account was removed")
		} else {
			ext, perr = s.d.Social.Publish(ctx, a, post.Body, post.Media)
		}
		now := s.now()
		if perr != nil {
			msg := social.SafeError(perr)
			targets[i].Status, targets[i].Error = TargetFailed, &msg
			err = s.tx(ctx, tid, func(ctx context.Context, tx pgx.Tx) error {
				return s.repo.SetTargetResult(ctx, tx, tid, t.ID, TargetFailed, nil, &msg, nil)
			})
		} else {
			targets[i].Status = TargetPublished
			err = s.tx(ctx, tid, func(ctx context.Context, tx pgx.Tx) error {
				return s.repo.SetTargetResult(ctx, tx, tid, t.ID, TargetPublished, &ext, nil, &now)
			})
		}
		if err != nil {
			return err
		}
	}
	return s.tx(ctx, tid, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.repo.SetStatus(ctx, tx, tid, post.ID, OverallStatus(targets))
		return err
	})
}

var errSkip = errors.New("skip")
