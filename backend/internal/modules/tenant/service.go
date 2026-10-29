package tenant

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/apperr"
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
	InviteMember(ctx context.Context, tenantID, actorID uuid.UUID, in InviteInput) (Member, error)
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
}

type service struct {
	pool  *pgxpool.Pool
	repo  Repository
	users UserDirectory
}

func NewService(pool *pgxpool.Pool, users UserDirectory) Service {
	return &service{pool: pool, users: users}
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
	err := db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		slug, err := s.uniqueSlug(ctx, tx, Slugify(name))
		if err != nil {
			return err
		}
		if t, err = s.repo.InsertTenant(ctx, tx, name, slug); err != nil {
			return db.MapErr(err, "tenant")
		}
		return db.WithTenantTx(ctx, s.pool, t.ID, func(ctx context.Context, tx pgx.Tx) error {
			if _, err := s.repo.InsertMember(ctx, tx, t.ID, ownerID, RoleOwner, 0, MemberActive, nil); err != nil {
				return err
			}
			return bootstrap(ctx, tx, t)
		})
	})
	return t, err
}

func (s *service) uniqueSlug(ctx context.Context, q db.Querier, base string) (string, error) {
	slug := base
	for i := 2; i < 1000; i++ {
		exists, err := s.repo.SlugExists(ctx, q, slug)
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
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		userID, err := s.users.FindOrCreateUser(ctx, in.Email, in.Phone, in.Name)
		if err != nil {
			return err
		}
		id, err := s.repo.InsertMember(ctx, tx, tenantID, userID, in.Role, in.CommissionBps, MemberInvited, &actorID)
		if err != nil {
			if db.IsUniqueViolation(err, "") {
				return apperr.Conflict("user is already a member of this business")
			}
			return err
		}
		if err := s.repo.InsertAudit(ctx, tx, tenantID, &actorID, "member.invite", "tenant_member", &id, map[string]any{"role": in.Role}); err != nil {
			return err
		}
		m, err = s.repo.GetMember(ctx, tx, tenantID, id, false)
		return err
	})
	return m, err
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
	err = db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
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
