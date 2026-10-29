package social

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
)

type Repository struct{}

const cols = `id, tenant_id, platform, external_id, name, username, avatar_url, token_secret_id, token_expires_at, scopes,
	webhook_verified, status, last_error, last_synced_at, connected_by, created_at`

func scan(r pgx.Row) (Account, error) {
	var a Account
	err := r.Scan(&a.ID, &a.TenantID, &a.Platform, &a.ExternalID, &a.Name, &a.Username, &a.AvatarURL, &a.TokenSecretID,
		&a.TokenExpiresAt, &a.Scopes, &a.WebhookVerified, &a.Status, &a.LastError, &a.LastSyncedAt, &a.ConnectedBy, &a.CreatedAt)
	return a, err
}

func collect(rows pgx.Rows, err error) ([]Account, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Account, error) { return scan(r) })
}

func (Repository) List(ctx context.Context, q db.Querier, tenantID uuid.UUID) ([]Account, error) {
	return collect(q.Query(ctx, `SELECT `+cols+` FROM social_accounts WHERE tenant_id = $1
		ORDER BY (status = 'connected') DESC, created_at`, tenantID))
}

func (Repository) Get(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (Account, error) {
	a, err := scan(q.QueryRow(ctx, `SELECT `+cols+` FROM social_accounts WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	return a, db.MapErr(err, "social account")
}

func (Repository) GetByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Account, error) {
	return collect(q.Query(ctx, `SELECT `+cols+` FROM social_accounts WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, ids))
}

func (Repository) Find(ctx context.Context, q db.Querier, tenantID uuid.UUID, platform, externalID string) (Account, error) {
	a, err := scan(q.QueryRow(ctx, `SELECT `+cols+` FROM social_accounts WHERE tenant_id = $1 AND platform = $2 AND external_id = $3`,
		tenantID, platform, externalID))
	return a, err
}

// ResolveInbound finds the connected account for a webhook across tenants (platform
// level, no tenant scope). If several tenants connected the same page, the most
// recently updated connection wins.
func (Repository) ResolveInbound(ctx context.Context, q db.Querier, platform, externalID string) (Account, error) {
	a, err := scan(q.QueryRow(ctx, `SELECT `+cols+` FROM social_accounts WHERE platform = $1 AND external_id = $2
		AND status IN ('connected','token_expired','error') ORDER BY updated_at DESC LIMIT 1`, platform, externalID))
	return a, db.MapErr(err, "social account")
}

// ByID loads an account without tenant scope (webhook routes carry the account id).
func (Repository) ByID(ctx context.Context, q db.Querier, id uuid.UUID) (Account, error) {
	a, err := scan(q.QueryRow(ctx, `SELECT `+cols+` FROM social_accounts WHERE id = $1`, id))
	return a, db.MapErr(err, "social account")
}

// Upsert inserts or refreshes a connection (same tenant + platform + external id).
func (Repository) Upsert(ctx context.Context, q db.Querier, tenantID uuid.UUID, a Account) (Account, error) {
	if a.Scopes == nil {
		a.Scopes = []string{}
	}
	out, err := scan(q.QueryRow(ctx, `INSERT INTO social_accounts (tenant_id, platform, external_id, name, username, avatar_url,
		token_secret_id, token_expires_at, scopes, webhook_verified, status, connected_by, last_synced_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'connected',$11, now())
		ON CONFLICT (tenant_id, platform, external_id) DO UPDATE SET name = EXCLUDED.name, username = EXCLUDED.username,
		  avatar_url = COALESCE(EXCLUDED.avatar_url, social_accounts.avatar_url), token_secret_id = EXCLUDED.token_secret_id,
		  token_expires_at = EXCLUDED.token_expires_at, scopes = EXCLUDED.scopes, webhook_verified = EXCLUDED.webhook_verified,
		  status = 'connected', last_error = NULL, connected_by = EXCLUDED.connected_by, last_synced_at = now()
		RETURNING `+cols, tenantID, a.Platform, a.ExternalID, a.Name, a.Username, a.AvatarURL, a.TokenSecretID, a.TokenExpiresAt,
		a.Scopes, a.WebhookVerified, a.ConnectedBy))
	return out, db.MapErr(err, "social account")
}

func (Repository) SetStatus(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, status string, lastError *string, synced bool) (Account, error) {
	a, err := scan(q.QueryRow(ctx, `UPDATE social_accounts SET status = $3, last_error = $4,
		last_synced_at = CASE WHEN $5 THEN now() ELSE last_synced_at END
		WHERE tenant_id = $1 AND id = $2 RETURNING `+cols, tenantID, id, status, lastError, synced))
	return a, db.MapErr(err, "social account")
}

func (Repository) UpdateProfile(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, name string, username *string) error {
	_, err := q.Exec(ctx, `UPDATE social_accounts SET name = $3, username = COALESCE($4, username) WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, name, username)
	return err
}

func (Repository) Disconnect(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE social_accounts SET status = 'disconnected', token_secret_id = NULL, webhook_verified = false
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return err
}

func (Repository) MarkWebhookVerified(ctx context.Context, q db.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE social_accounts SET webhook_verified = true WHERE id = $1 AND NOT webhook_verified`, id)
	return err
}
