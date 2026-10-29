package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
)

// Repository is the SQL layer. Every tenant-scoped query filters by tenant_id and runs
// inside db.WithTenantTx (enforced by the service).
type Repository struct{}

const tenantCols = `id, name, slug::text, plan, status, currency, timezone, locale, settings, trial_ends_at, created_at`

func scanTenant(row pgx.Row) (Tenant, error) {
	var t Tenant
	err := row.Scan(&t.ID, &t.Name, &t.Slug, &t.Plan, &t.Status, &t.Currency, &t.Timezone, &t.Locale, &t.Settings, &t.TrialEndsAt, &t.CreatedAt)
	return t, err
}

func (Repository) SlugExists(ctx context.Context, q db.Querier, slug string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tenants WHERE slug = $1) OR EXISTS (SELECT 1 FROM storefronts WHERE slug = $1)`, slug).Scan(&ok)
	return ok, err
}

func (Repository) InsertTenant(ctx context.Context, q db.Querier, name, slug string) (Tenant, error) {
	return scanTenant(q.QueryRow(ctx, `INSERT INTO tenants (name, slug, trial_ends_at) VALUES ($1, $2, now() + interval '14 days')
		RETURNING `+tenantCols, name, slug))
}

func (Repository) GetTenant(ctx context.Context, q db.Querier, id uuid.UUID) (Tenant, error) {
	t, err := scanTenant(q.QueryRow(ctx, `SELECT `+tenantCols+` FROM tenants WHERE id = $1`, id))
	return t, db.MapErr(err, "tenant")
}

func (Repository) UpdateTenant(ctx context.Context, q db.Querier, id uuid.UUID, in UpdateInput) (Tenant, error) {
	var settings []byte
	if in.Settings != nil {
		settings, _ = json.Marshal(in.Settings)
	}
	t, err := scanTenant(q.QueryRow(ctx, `UPDATE tenants SET
		name = COALESCE($2, name), currency = COALESCE($3, currency), timezone = COALESCE($4, timezone),
		locale = COALESCE($5, locale),
		settings = CASE WHEN $6::jsonb IS NULL THEN settings
		  ELSE ($6::jsonb - 'sms_webhook_secret_id' - 'sms_webhook_token_hint' - 'vault_key_fingerprint')
		    || jsonb_strip_nulls(jsonb_build_object('sms_webhook_secret_id', settings->'sms_webhook_secret_id',
		         'sms_webhook_token_hint', settings->'sms_webhook_token_hint',
		         'vault_key_fingerprint', settings->'vault_key_fingerprint')) END
		WHERE id = $1 RETURNING `+tenantCols, id, in.Name, in.Currency, in.Timezone, in.Locale, settings))
	return t, db.MapErr(err, "tenant")
}

const memberSelect = `SELECT m.id, m.tenant_id, m.user_id, m.role, m.commission_bps, m.status, m.created_at, m.invite_name, m.invite_expires_at,
	u.id, u.name, u.email::text, u.phone, u.avatar_url, u.is_platform_admin, u.created_at
	FROM tenant_members m JOIN users u ON u.id = m.user_id`

func scanMember(row pgx.Row) (Member, error) {
	var m Member
	var inviteName *string
	err := row.Scan(&m.ID, &m.TenantID, &m.UserID, &m.Role, &m.CommissionBps, &m.Status, &m.CreatedAt, &inviteName, &m.InviteExpiresAt,
		&m.User.ID, &m.User.Name, &m.User.Email, &m.User.Phone, &m.User.AvatarURL, &m.User.IsPlatformAdmin, &m.User.CreatedAt)
	if err == nil && m.Status == MemberInvited {
		// SECURITY M4: until the invitee accepts, show only what the inviter typed (name) and
		// masked contact hints, never an existing user's real profile.
		m.User = UserSummary{ID: m.User.ID, Name: deref(inviteName, m.User.Name), Email: MaskEmail(m.User.Email),
			Phone: MaskPhone(m.User.Phone), CreatedAt: m.CreatedAt}
	}
	if m.Status != MemberInvited {
		m.InviteExpiresAt = nil
	}
	return m, err
}

func deref(p *string, def string) string {
	if p == nil || *p == "" {
		return def
	}
	return *p
}

// NewMember is a membership row to insert (invite fields only for status invited).
type NewMember struct {
	TenantID, UserID uuid.UUID
	Role             string
	CommissionBps    int
	Status           string
	InvitedBy        *uuid.UUID
	InviteHash       *string
	InviteExpiresAt  *time.Time
	InviteName       *string
}

func (Repository) InsertMember(ctx context.Context, q db.Querier, n NewMember) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `INSERT INTO tenant_members (tenant_id, user_id, role, commission_bps, status, invited_by,
		invite_token_hash, invite_expires_at, invite_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, n.TenantID, n.UserID, n.Role, n.CommissionBps, n.Status, n.InvitedBy,
		n.InviteHash, n.InviteExpiresAt, n.InviteName).Scan(&id)
	return id, err
}

// SetInvite stores a new invite token hash/expiry on an invited membership (resendInvite).
func (Repository) SetInvite(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, hash string, exp time.Time) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE tenant_members SET invite_token_hash = $3, invite_expires_at = $4
		WHERE tenant_id = $1 AND id = $2 AND status = 'invited'`, tenantID, id, hash, exp)
	return tag.RowsAffected() == 1, err
}

// InviteByHash resolves an invite token hash (needs db.LookupInviteHash) to its invitation.
func (Repository) InviteByHash(ctx context.Context, q db.Querier, hash string) (InviteInfo, error) {
	var i InviteInfo
	var name *string
	err := q.QueryRow(ctx, `SELECT m.id, m.tenant_id, t.name, m.user_id, m.role, m.invite_name, u.name, u.email::text, u.phone,
		u.password_hash IS NOT NULL, m.invite_expires_at
		FROM tenant_members m JOIN tenants t ON t.id = m.tenant_id JOIN users u ON u.id = m.user_id
		WHERE m.invite_token_hash = $1 AND m.status = 'invited' AND t.status = 'active'`, hash).
		Scan(&i.MemberID, &i.TenantID, &i.TenantName, &i.UserID, &i.Role, &name, &i.InviteeName, &i.Email, &i.Phone,
			&i.UserHasPassword, &i.ExpiresAt)
	if name != nil && *name != "" {
		i.InviteeName = *name
	}
	return i, db.MapErr(err, "invite")
}

// ActivateInvite turns an open invitation into an active membership (single use).
func (Repository) ActivateInvite(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, hash string) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE tenant_members SET status = 'active', invite_token_hash = NULL, invite_expires_at = NULL
		WHERE tenant_id = $1 AND id = $2 AND status = 'invited' AND invite_token_hash = $3 AND invite_expires_at > now()`,
		tenantID, id, hash)
	return tag.RowsAffected() == 1, err
}

func (Repository) ListMembers(ctx context.Context, q db.Querier, tenantID uuid.UUID) ([]Member, error) {
	rows, err := q.Query(ctx, memberSelect+` WHERE m.tenant_id = $1 ORDER BY m.created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Member, error) { return scanMember(r) })
}

func (Repository) GetMember(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Member, error) {
	sql := memberSelect + ` WHERE m.tenant_id = $1 AND m.id = $2`
	if lock {
		sql += ` FOR UPDATE OF m`
	}
	m, err := scanMember(q.QueryRow(ctx, sql, tenantID, id))
	return m, db.MapErr(err, "member")
}

func (Repository) UpdateMember(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, role string, bps *int) error {
	_, err := q.Exec(ctx, `UPDATE tenant_members SET role = $3, commission_bps = COALESCE($4, commission_bps)
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, role, bps)
	return db.MapErr(err, "member")
}

func (Repository) DeleteMember(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) error {
	_, err := q.Exec(ctx, `DELETE FROM tenant_members WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return err
}

func (Repository) CountOwners(ctx context.Context, q db.Querier, tenantID uuid.UUID) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM tenant_members WHERE tenant_id = $1 AND role = 'owner' AND status = 'active'`, tenantID).Scan(&n)
	return n, err
}

// MembershipsForUser is a cross-tenant lookup by user (login/switch, db.LookupUserID); it
// filters by user_id. Open invitations are not memberships until accepted (BUG-03).
func (Repository) MembershipsForUser(ctx context.Context, q db.Querier, userID uuid.UUID) ([]Membership, error) {
	rows, err := q.Query(ctx, `SELECT m.id, m.user_id, m.role, m.commission_bps, m.status,
		t.id, t.name, t.slug::text, t.plan, t.status, t.currency, t.timezone, t.locale, t.settings, t.trial_ends_at, t.created_at
		FROM tenant_members m JOIN tenants t ON t.id = m.tenant_id
		WHERE m.user_id = $1 AND m.status <> 'invited' AND t.status <> 'deleted'
		ORDER BY (m.role = 'owner') DESC, m.created_at`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Membership, error) {
		var m Membership
		t := &m.Tenant
		err := r.Scan(&m.ID, &m.UserID, &m.Role, &m.CommissionBps, &m.Status,
			&t.ID, &t.Name, &t.Slug, &t.Plan, &t.Status, &t.Currency, &t.Timezone, &t.Locale, &t.Settings, &t.TrialEndsAt, &t.CreatedAt)
		return m, err
	})
}

func (Repository) ListNotifications(ctx context.Context, q db.Querier, tenantID, userID uuid.UUID, unreadOnly bool, limit int) ([]Notification, error) {
	rows, err := q.Query(ctx, `SELECT id, kind, title, body, link, read_at, created_at FROM notifications
		WHERE tenant_id = $1 AND (user_id = $2 OR user_id IS NULL) AND ($3 = false OR read_at IS NULL)
		ORDER BY created_at DESC LIMIT $4`, tenantID, userID, unreadOnly, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Notification, error) {
		var n Notification
		err := r.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.Link, &n.ReadAt, &n.CreatedAt)
		return n, err
	})
}

func (Repository) MarkNotificationsRead(ctx context.Context, q db.Querier, tenantID, userID uuid.UUID, id *uuid.UUID) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE notifications SET read_at = now()
		WHERE tenant_id = $1 AND (user_id = $2 OR user_id IS NULL) AND read_at IS NULL AND ($3::uuid IS NULL OR id = $3)`,
		tenantID, userID, id)
	return tag.RowsAffected(), err
}

func (Repository) InsertNotification(ctx context.Context, q db.Querier, tenantID uuid.UUID, userID *uuid.UUID, kind, title string, body, link *string) error {
	_, err := q.Exec(ctx, `INSERT INTO notifications (tenant_id, user_id, kind, title, body, link) VALUES ($1,$2,$3,$4,$5,$6)`,
		tenantID, userID, kind, title, body, link)
	return err
}

func (Repository) ListAuditLogs(ctx context.Context, q db.Querier, tenantID uuid.UUID, beforeID int64, limit int) ([]AuditLog, error) {
	rows, err := q.Query(ctx, `SELECT id, user_id, action, entity_type, entity_id, meta, created_at FROM audit_logs
		WHERE tenant_id = $1 AND ($2 = 0 OR id < $2) ORDER BY id DESC LIMIT $3`, tenantID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (AuditLog, error) {
		var a AuditLog
		err := r.Scan(&a.ID, &a.UserID, &a.Action, &a.EntityType, &a.EntityID, &a.Meta, &a.CreatedAt)
		return a, err
	})
}

func (Repository) InsertAudit(ctx context.Context, q db.Querier, tenantID uuid.UUID, userID *uuid.UUID, action, entityType string, entityID *uuid.UUID, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("audit meta: %w", err)
	}
	_, err = q.Exec(ctx, `INSERT INTO audit_logs (tenant_id, user_id, action, entity_type, entity_id, meta) VALUES ($1,$2,$3,$4,$5,$6)`,
		tenantID, userID, action, entityType, entityID, b)
	return err
}
