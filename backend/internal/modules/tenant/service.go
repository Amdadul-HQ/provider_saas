package tenant

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/apperr"
	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/db"
)

// UserDirectory is implemented by the auth module (wired in app.Container).
type UserDirectory interface {
	// FindOrCreateUser returns an existing user by email/phone or creates a passwordless one.
	FindOrCreateUser(ctx context.Context, email, phone *string, name string) (uuid.UUID, error)
}

// Service is the tenant module API.
type Service interface {
	// Provision creates a tenant, the owner membership and default data (joins ctx tx).
	Provision(ctx context.Context, ownerID uuid.UUID, businessName string) (Tenant, error)
	Get(ctx context.Context, tenantID uuid.UUID) (Tenant, error)
	Update(ctx context.Context, tenantID, actorID uuid.UUID, in UpdateInput) (Tenant, error)
	Members(ctx context.Context, tenantID uuid.UUID) ([]Member, error)
	// InviteMember creates an invited membership and returns it with a one-time InviteURL.
	InviteMember(ctx context.Context, tenantID, actorID uuid.UUID, in InviteInput) (Member, error)
	// ResendInvite rotates the invite token of an invited membership (old link stops working).
	ResendInvite(ctx context.Context, tenantID, actorID, memberID uuid.UUID) (Member, error)
	// InviteByToken resolves a pending, unexpired invite (public; no tenant known yet).
	InviteByToken(ctx context.Context, token string) (InviteInfo, error)
	// ActivateInvite marks the invitation accepted (joins the caller's transaction).
	ActivateInvite(ctx context.Context, inv InviteInfo, token string) error
	UpdateMemberRole(ctx context.Context, tenantID, actorID, memberID uuid.UUID, role string, bps *int) (Member, error)
	RemoveMember(ctx context.Context, tenantID, actorID, memberID uuid.UUID) error
	MembershipsForUser(ctx context.Context, userID uuid.UUID) ([]Membership, error)
	Membership(ctx context.Context, userID, tenantID uuid.UUID) (Membership, error)
	Notifications(ctx context.Context, tenantID, userID uuid.UUID, unreadOnly bool, limit int) ([]Notification, error)
	MarkNotificationRead(ctx context.Context, tenantID, userID uuid.UUID, id *uuid.UUID) error
	Notify(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, kind, title string, body, link *string) error
	AuditLogs(ctx context.Context, tenantID uuid.UUID, beforeID int64, limit int) ([]AuditLog, error)
	Audit(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, action, entityType string, entityID *uuid.UUID, meta map[string]any) error
	TenantIDs(ctx context.Context) ([]uuid.UUID, error)
	// EnsureSMSWebhookToken creates the SMS-forwarder token on first use (plaintext returned once).
	EnsureSMSWebhookToken(ctx context.Context, tenantID uuid.UUID) (token, hint string, created bool, err error)
	// RotateSMSWebhookToken replaces the SMS-forwarder token; the new plaintext is returned once.
	RotateSMSWebhookToken(ctx context.Context, tenantID, actorID uuid.UUID) (string, error)
	VerifySMSWebhookToken(ctx context.Context, token string) (uuid.UUID, error)
}

type service struct {
	pool         *pgxpool.Pool
	repo         Repository
	users        UserDirectory
	secrets      Secrets
	publicAppURL string
	now          func() time.Time
}

// NewService builds the tenant service. secrets (the vault) backs the SMS-forwarder token;
// publicAppURL is the frontend origin used to build invite links.
func NewService(pool *pgxpool.Pool, users UserDirectory, secrets Secrets, publicAppURL string) Service {
	return &service{pool: pool, users: users, secrets: secrets, publicAppURL: strings.TrimRight(publicAppURL, "/"), now: time.Now}
}

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func (s *service) Provision(ctx context.Context, ownerID uuid.UUID, businessName string) (Tenant, error) {
	name := strings.TrimSpace(businessName)
	if name == "" {
		return Tenant{}, apperr.ValidationField("businessName", "required")
	}
	var t Tenant
	err := db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		slug, err := s.uniqueSlug(ctx, tx, Slugify(name))
		if err != nil {
			return err
		}
		if t, err = s.repo.InsertTenant(ctx, tx, name, slug); err != nil {
			return db.MapErr(err, "tenant")
		}
		return db.WithTenantTx(ctx, s.pool, t.ID, func(ctx context.Context, tx pgx.Tx) error {
			if _, err := s.repo.InsertMember(ctx, tx, NewMember{TenantID: t.ID, UserID: ownerID, Role: RoleOwner, Status: MemberActive}); err != nil {
				return db.MapErr(err, "member")
			}
			return bootstrap(ctx, tx, t)
		})
	})
	return t, err
}

func (s *service) uniqueSlug(ctx context.Context, q db.Querier, base string) (string, error) {
	slug := base
	for i := 2; i < 1000; i++ {
		var exists bool
		// storefronts is a tenant table: the slug lookup policy exposes only rows with this slug.
		err := db.WithLookup(ctx, s.pool, db.LookupSlug, slug, func(ctx context.Context, _ pgx.Tx) (err error) {
			exists, err = s.repo.SlugExists(ctx, q, slug)
			return err
		})
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return base + "-" + uuid.NewString()[:8], nil
}

func (s *service) Get(ctx context.Context, tenantID uuid.UUID) (t Tenant, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		t, err = s.repo.GetTenant(ctx, tx, tenantID)
		return err
	})
	return t, err
}

var validCurrencies = map[string]bool{"BDT": true, "USD": true, "INR": true, "EUR": true, "GBP": true}

func (s *service) Update(ctx context.Context, tenantID, actorID uuid.UUID, in UpdateInput) (t Tenant, err error) {
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return t, apperr.ValidationField("name", "must not be empty")
	}
	if in.Currency != nil {
		c := strings.ToUpper(*in.Currency)
		if !validCurrencies[c] {
			return t, apperr.ValidationField("currency", "unsupported currency")
		}
		in.Currency = &c
	}
	if in.Locale != nil && *in.Locale != "bn" && *in.Locale != "en" {
		return t, apperr.ValidationField("locale", "must be bn or en")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if t, err = s.repo.UpdateTenant(ctx, tx, tenantID, in); err != nil {
			return err
		}
		return s.repo.InsertAudit(ctx, tx, tenantID, &actorID, "tenant.update", "tenant", &tenantID, nil)
	})
	return t, err
}

func (s *service) Members(ctx context.Context, tenantID uuid.UUID) (out []Member, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ListMembers(ctx, tx, tenantID)
		return err
	})
	return out, err
}

func (s *service) InviteMember(ctx context.Context, tenantID, actorID uuid.UUID, in InviteInput) (m Member, err error) {
	in.Role = strings.ToLower(in.Role)
	switch {
	case !ValidRole(in.Role):
		return m, apperr.ValidationField("role", "invalid role")
	case in.Role == RoleOwner:
		return m, apperr.ValidationField("role", "invite as admin, then transfer ownership")
	case (in.Email == nil || *in.Email == "") && (in.Phone == nil || *in.Phone == ""):
		return m, apperr.Validation("email or phone is required")
	case strings.TrimSpace(in.Name) == "":
		return m, apperr.ValidationField("name", "required")
	case in.CommissionBps < 0 || in.CommissionBps > 10000:
		return m, apperr.ValidationField("commissionBps", "must be 0..10000")
	}
	if s.users == nil {
		return m, fmt.Errorf("tenant: user directory not configured")
	}
	token, hash, err := newInviteToken()
	if err != nil {
		return m, err
	}
	exp := s.now().Add(InviteTTL)
	name := strings.TrimSpace(in.Name)
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		// Creates a passwordless user only when none exists; an existing user is NOT attached
		// until they accept with their own password (BUG-03 / M4).
		userID, err := s.users.FindOrCreateUser(ctx, in.Email, in.Phone, name)
		if err != nil {
			return err
		}
		id, err := s.repo.InsertMember(ctx, tx, NewMember{TenantID: tenantID, UserID: userID, Role: in.Role, CommissionBps: in.CommissionBps,
			Status: MemberInvited, InvitedBy: &actorID, InviteHash: &hash, InviteExpiresAt: &exp, InviteName: &name})
		if err != nil {
			if db.IsUniqueViolation(err, "") {
				return apperr.Conflict("this person is already a member or has a pending invite (use resend invite)")
			}
			return err
		}
		if err := s.repo.InsertAudit(ctx, tx, tenantID, &actorID, "member.invite", "tenant_member", &id, map[string]any{"role": in.Role}); err != nil {
			return err
		}
		m, err = s.repo.GetMember(ctx, tx, tenantID, id, false)
		return err
	})
	if err != nil {
		return m, err
	}
	m.InviteURL = s.inviteURL(token)
	return m, nil
}

func (s *service) inviteURL(token string) string { return s.publicAppURL + "/invite/" + token }

// newInviteToken returns a 256-bit url-safe token and its SHA-256 (stored).
func newInviteToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, pauth.HashToken(token), nil
}

func (s *service) ResendInvite(ctx context.Context, tenantID, actorID, memberID uuid.UUID) (m Member, err error) {
	token, hash, err := newInviteToken()
	if err != nil {
		return m, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.SetInvite(ctx, tx, tenantID, memberID, hash, s.now().Add(InviteTTL))
		if err != nil {
			return err
		}
		if !ok {
			if _, err := s.repo.GetMember(ctx, tx, tenantID, memberID, false); err != nil {
				return err
			}
			return apperr.Validation("this member has already accepted the invite")
		}
		if err := s.repo.InsertAudit(ctx, tx, tenantID, &actorID, "member.invite_resend", "tenant_member", &memberID, nil); err != nil {
			return err
		}
		m, err = s.repo.GetMember(ctx, tx, tenantID, memberID, false)
		return err
	})
	if err != nil {
		return m, err
	}
	m.InviteURL = s.inviteURL(token)
	return m, nil
}

// errInvite is deliberately generic: unknown, expired and used tokens look the same.
var errInvite = apperr.NotFound("invite")

func (s *service) InviteByToken(ctx context.Context, token string) (inv InviteInfo, err error) {
	token = strings.TrimSpace(token)
	if len(token) < 20 || len(token) > 100 {
		return inv, errInvite
	}
	hash := pauth.HashToken(token)
	err = db.WithLookup(ctx, s.pool, db.LookupInviteHash, hash, func(ctx context.Context, tx pgx.Tx) error {
		inv, err = s.repo.InviteByHash(ctx, tx, hash)
		return err
	})
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return inv, errInvite
		}
		return inv, err
	}
	if !s.now().Before(inv.ExpiresAt) {
		return inv, errInvite
	}
	return inv, nil
}

func (s *service) ActivateInvite(ctx context.Context, inv InviteInfo, token string) error {
	return s.tx(ctx, inv.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.ActivateInvite(ctx, tx, inv.TenantID, inv.MemberID, pauth.HashToken(token))
		if err != nil {
			return err
		}
		if !ok {
			return errInvite
		}
		return s.repo.InsertAudit(ctx, tx, inv.TenantID, &inv.UserID, "member.accept", "tenant_member", &inv.MemberID, map[string]any{"role": inv.Role})
	})
}

func (s *service) UpdateMemberRole(ctx context.Context, tenantID, actorID, memberID uuid.UUID, role string, bps *int) (m Member, err error) {
	role = strings.ToLower(role)
	if !ValidRole(role) {
		return m, apperr.ValidationField("role", "invalid role")
	}
	if bps != nil && (*bps < 0 || *bps > 10000) {
		return m, apperr.ValidationField("commissionBps", "must be 0..10000")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.GetMember(ctx, tx, tenantID, memberID, true)
		if err != nil {
			return err
		}
		if cur.Role == RoleOwner && role != RoleOwner {
			n, err := s.repo.CountOwners(ctx, tx, tenantID)
			if err != nil {
				return err
			}
			if n <= 1 {
				return apperr.Validation("a business must keep at least one owner")
			}
		}
		if err := s.repo.UpdateMember(ctx, tx, tenantID, memberID, role, bps); err != nil {
			return err
		}
		if err := s.repo.InsertAudit(ctx, tx, tenantID, &actorID, "member.role", "tenant_member", &memberID,
			map[string]any{"from": cur.Role, "to": role}); err != nil {
			return err
		}
		m, err = s.repo.GetMember(ctx, tx, tenantID, memberID, false)
		return err
	})
	return m, err
}

func (s *service) RemoveMember(ctx context.Context, tenantID, actorID, memberID uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.GetMember(ctx, tx, tenantID, memberID, true)
		if err != nil {
			return err
		}
		if cur.Role == RoleOwner {
			n, err := s.repo.CountOwners(ctx, tx, tenantID)
			if err != nil {
				return err
			}
			if n <= 1 {
				return apperr.Validation("cannot remove the last owner")
			}
		}
		if err := s.repo.DeleteMember(ctx, tx, tenantID, memberID); err != nil {
			return err
		}
		return s.repo.InsertAudit(ctx, tx, tenantID, &actorID, "member.remove", "tenant_member", &memberID, nil)
	})
}

func (s *service) MembershipsForUser(ctx context.Context, userID uuid.UUID) (out []Membership, err error) {
	err = db.WithLookup(ctx, s.pool, db.LookupUserID, userID.String(), func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.MembershipsForUser(ctx, tx, userID)
		return err
	})
	return out, err
}

func (s *service) Membership(ctx context.Context, userID, tenantID uuid.UUID) (Membership, error) {
	ms, err := s.MembershipsForUser(ctx, userID)
	if err != nil {
		return Membership{}, err
	}
	for _, m := range ms {
		if m.Tenant.ID == tenantID {
			return m, nil
		}
	}
	return Membership{}, apperr.Forbidden("not a member of this business")
}

func (s *service) Notifications(ctx context.Context, tenantID, userID uuid.UUID, unreadOnly bool, limit int) (out []Notification, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ListNotifications(ctx, tx, tenantID, userID, unreadOnly, limit)
		return err
	})
	return out, err
}

func (s *service) MarkNotificationRead(ctx context.Context, tenantID, userID uuid.UUID, id *uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.repo.MarkNotificationsRead(ctx, tx, tenantID, userID, id)
		return err
	})
}

func (s *service) Notify(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, kind, title string, body, link *string) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.InsertNotification(ctx, tx, tenantID, userID, kind, title, body, link)
	})
}

func (s *service) AuditLogs(ctx context.Context, tenantID uuid.UUID, beforeID int64, limit int) (out []AuditLog, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ListAuditLogs(ctx, tx, tenantID, beforeID, limit)
		return err
	})
	return out, err
}

func (s *service) Audit(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, action, entityType string, entityID *uuid.UUID, meta map[string]any) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.InsertAudit(ctx, tx, tenantID, userID, action, entityType, entityID, meta)
	})
}

// TenantIDs lists active tenants (platform-level, used by background jobs).
func (s *service) TenantIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM tenants WHERE status = 'active' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}
