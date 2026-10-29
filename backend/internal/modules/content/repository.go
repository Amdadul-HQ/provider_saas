package content

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/modules/social"
	"provider_saas/internal/platform/db"
)

type Repository struct{}

const cols = `p.id, p.body, p.media, p.scheduled_at, p.status, p.created_by, p.created_at, p.updated_at`

func scan(r pgx.Row) (Post, error) {
	var p Post
	var media []byte
	err := r.Scan(&p.ID, &p.Body, &media, &p.ScheduledAt, &p.Status, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err == nil && len(media) > 0 {
		_ = json.Unmarshal(media, &p.Media)
	}
	return p, err
}

func mediaJSON(m []social.Attachment) []byte {
	if m == nil {
		m = []social.Attachment{}
	}
	b, _ := json.Marshal(m)
	return b
}

func (Repository) Insert(ctx context.Context, q db.Querier, tenantID uuid.UUID, in PostInput, status string, by *uuid.UUID) (Post, error) {
	p, err := scan(q.QueryRow(ctx, `INSERT INTO posts AS p (tenant_id, body, media, scheduled_at, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+cols, tenantID, in.Body, mediaJSON(in.Media), in.ScheduledAt, status, by))
	return p, db.MapErr(err, "post")
}

func (Repository) Update(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, in PostInput, status string) (Post, error) {
	p, err := scan(q.QueryRow(ctx, `UPDATE posts p SET body = $3, media = $4, scheduled_at = $5, status = $6
		WHERE p.tenant_id = $1 AND p.id = $2 RETURNING `+cols, tenantID, id, in.Body, mediaJSON(in.Media), in.ScheduledAt, status))
	return p, db.MapErr(err, "post")
}

func (Repository) SetStatus(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, status string) (Post, error) {
	p, err := scan(q.QueryRow(ctx, `UPDATE posts p SET status = $3 WHERE p.tenant_id = $1 AND p.id = $2 RETURNING `+cols, tenantID, id, status))
	return p, db.MapErr(err, "post")
}

func (Repository) Get(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Post, error) {
	sql := `SELECT ` + cols + ` FROM posts p WHERE p.tenant_id = $1 AND p.id = $2`
	if lock {
		sql += ` FOR UPDATE`
	}
	p, err := scan(q.QueryRow(ctx, sql, tenantID, id))
	return p, db.MapErr(err, "post")
}

func (Repository) Delete(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) error {
	_, err := q.Exec(ctx, `DELETE FROM posts WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return err
}

func (Repository) List(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter, after *uuid.UUID, limit int) ([]Post, error) {
	args := []any{tenantID}
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	conds := []string{"p.tenant_id = $1"}
	if f.Status != nil {
		conds = append(conds, "p.status = "+add(*f.Status))
	}
	if after != nil {
		conds = append(conds, "(p.created_at, p.id) < (SELECT created_at, id FROM posts WHERE tenant_id = $1 AND id = "+add(*after)+")")
	}
	rows, err := q.Query(ctx, `SELECT `+cols+` FROM posts p WHERE `+strings.Join(conds, " AND ")+
		` ORDER BY p.created_at DESC, p.id DESC LIMIT `+add(limit), args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Post, error) { return scan(r) })
}

// ---- targets

const targetCols = `id, post_id, social_account_id, external_post_id, status, error, published_at`

func (Repository) ReplaceTargets(ctx context.Context, q db.Querier, tenantID, postID uuid.UUID, accountIDs []uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM post_targets WHERE tenant_id = $1 AND post_id = $2 AND NOT (social_account_id = ANY($3))`,
		tenantID, postID, accountIDs); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `INSERT INTO post_targets (tenant_id, post_id, social_account_id)
		SELECT $1, $2, unnest($3::uuid[]) ON CONFLICT (post_id, social_account_id) DO UPDATE SET status = 'pending', error = NULL
		WHERE post_targets.status <> 'published'`, tenantID, postID, accountIDs)
	return err
}

func (Repository) TargetsByPosts(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Target, error) {
	rows, err := q.Query(ctx, `SELECT `+targetCols+` FROM post_targets WHERE tenant_id = $1 AND post_id = ANY($2) ORDER BY id`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Target, error) {
		var t Target
		err := r.Scan(&t.ID, &t.PostID, &t.SocialAccountID, &t.ExternalPostID, &t.Status, &t.Error, &t.PublishedAt)
		return t, err
	})
}

func (Repository) SetTargetResult(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, status string, externalID, errMsg *string, at *time.Time) error {
	_, err := q.Exec(ctx, `UPDATE post_targets SET status = $3, external_post_id = COALESCE($4, external_post_id), error = $5, published_at = $6
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, status, externalID, errMsg, at)
	return err
}
