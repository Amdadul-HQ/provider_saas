package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
)

// Repository: users and refresh_tokens are platform tables (not tenant-scoped).
type Repository struct{}

const userCols = `id, name, email::text, phone, avatar_url, is_platform_admin, password_hash, created_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.AvatarURL, &u.IsPlatformAdmin, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

func (Repository) InsertUser(ctx context.Context, q db.Querier, name string, email, phone, hash *string) (User, error) {
	u, err := scanUser(q.QueryRow(ctx, `INSERT INTO users (name, email, phone, password_hash) VALUES ($1,$2,$3,$4) RETURNING `+userCols,
		name, email, phone, hash))
	return u, err
}

func (Repository) UserByID(ctx context.Context, q db.Querier, id uuid.UUID) (User, error) {
	u, err := scanUser(q.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
	return u, db.MapErr(err, "user")
}

func (Repository) UsersByIDs(ctx context.Context, q db.Querier, ids []uuid.UUID) ([]User, error) {
	rows, err := q.Query(ctx, `SELECT `+userCols+` FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (User, error) { return scanUser(r) })
}

// UserByIdentifier finds a user by email (case-insensitive) or phone.
func (Repository) UserByIdentifier(ctx context.Context, q db.Querier, email, phone *string) (User, error) {
	u, err := scanUser(q.QueryRow(ctx, `SELECT `+userCols+` FROM users
		WHERE ($1::text IS NOT NULL AND email = $1::citext) OR ($2::text IS NOT NULL AND phone = $2) LIMIT 1`, email, phone))
	return u, db.MapErr(err, "user")
}

func (Repository) SetPassword(ctx context.Context, q db.Querier, id uuid.UUID, hash string) error {
	_, err := q.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, hash)
	return err
}

func (Repository) SetName(ctx context.Context, q db.Querier, id uuid.UUID, name string) error {
	_, err := q.Exec(ctx, `UPDATE users SET name = $2 WHERE id = $1`, id, name)
	return err
}

// InsertPlatformAudit writes a tenant-less audit row (failed logins, lockouts). RLS only
// allows inserting such rows; nobody can read them through the API role.
func (Repository) InsertPlatformAudit(ctx context.Context, q db.Querier, userID *uuid.UUID, action string, meta map[string]any) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	var ip *string
	if v, _ := meta["ip"].(string); v != "" {
		ip = &v
	}
	_, err = q.Exec(ctx, `INSERT INTO audit_logs (tenant_id, user_id, action, entity_type, entity_id, meta, ip)
		VALUES (NULL, $1, $2, 'user', $1, $3, $4::inet)`, userID, action, b, ip)
	return err
}

func (Repository) TouchLogin(ctx context.Context, q db.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	return err
}

func (Repository) InsertRefresh(ctx context.Context, q db.Querier, userID uuid.UUID, hash string, meta ClientMeta, exp time.Time) error {
	var ua, ip *string
	if meta.UserAgent != "" {
		ua = &meta.UserAgent
	}
	if meta.IP != "" {
		ip = &meta.IP
	}
	_, err := q.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, user_agent, ip, expires_at) VALUES ($1,$2,$3,$4::inet,$5)`,
		userID, hash, ua, ip, exp)
	return err
}

type refreshRow struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

func (Repository) RefreshByHash(ctx context.Context, q db.Querier, hash string) (refreshRow, error) {
	var r refreshRow
	err := q.QueryRow(ctx, `SELECT id, user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`, hash).
		Scan(&r.ID, &r.UserID, &r.ExpiresAt, &r.RevokedAt)
	return r, db.MapErr(err, "refresh token")
}

func (Repository) RevokeRefresh(ctx context.Context, q db.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

func (Repository) RevokeAllForUser(ctx context.Context, q db.Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}
