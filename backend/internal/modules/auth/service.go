package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/platform/apperr"
	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/db"
)

// Tenants is what auth needs from the tenant module.
type Tenants interface {
	Provision(ctx context.Context, ownerID uuid.UUID, businessName string) (tenant.Tenant, error)
	MembershipsForUser(ctx context.Context, userID uuid.UUID) ([]tenant.Membership, error)
	Membership(ctx context.Context, userID, tenantID uuid.UUID) (tenant.Membership, error)
}

// Service is the auth module API.
type Service interface {
	Register(ctx context.Context, in RegisterInput, meta ClientMeta) (Session, error)
	Login(ctx context.Context, in LoginInput, meta ClientMeta) (Session, error)
	Refresh(ctx context.Context, refreshToken string, meta ClientMeta) (Session, error)
	Logout(ctx context.Context, refreshToken string) error
	SwitchTenant(ctx context.Context, userID, tenantID uuid.UUID, meta ClientMeta) (Session, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error
	User(ctx context.Context, id uuid.UUID) (User, error)
	UsersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]User, error)
	Memberships(ctx context.Context, userID uuid.UUID) ([]tenant.Membership, error)
	// FindOrCreateUser implements tenant.UserDirectory.
	FindOrCreateUser(ctx context.Context, email, phone *string, name string) (uuid.UUID, error)
}

type service struct {
	pool       *pgxpool.Pool
	repo       Repository
	tokens     *pauth.TokenIssuer
	refreshTTL time.Duration
	tenants    Tenants
}

func NewService(pool *pgxpool.Pool, tokens *pauth.TokenIssuer, refreshTTL time.Duration, tenants Tenants) Service {
	return &service{pool: pool, tokens: tokens, refreshTTL: refreshTTL, tenants: tenants}
}

// NewUserDirectory returns a tenant.UserDirectory without the tenant dependency
// (breaks the construction cycle auth <-> tenant).
func NewUserDirectory(pool *pgxpool.Pool) tenant.UserDirectory { return &service{pool: pool} }

var errBadCredentials = apperr.Unauthenticated("invalid credentials")

func (s *service) Register(ctx context.Context, in RegisterInput, meta ClientMeta) (Session, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return Session{}, apperr.ValidationField("name", "required")
	}
	email, err := NormalizeEmail(in.Email)
	if err != nil {
		return Session{}, err
	}
	ph, err := NormalizePhone(in.Phone)
	if err != nil {
		return Session{}, err
	}
	if email == nil && ph == nil {
		return Session{}, apperr.Validation("email or phone is required")
	}
	if err := ValidatePassword(in.Password); err != nil {
		return Session{}, err
	}
	if strings.TrimSpace(in.BusinessName) == "" {
		return Session{}, apperr.ValidationField("businessName", "required")
	}
	hash, err := pauth.HashPassword(in.Password)
	if err != nil {
		return Session{}, err
	}
	var sess Session
	err = db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.repo.InsertUser(ctx, tx, in.Name, email, ph, &hash)
		if err != nil {
			if db.IsUniqueViolation(err, "") {
				return apperr.Conflict("an account with this email or phone already exists")
			}
			return err
		}
		t, err := s.tenants.Provision(ctx, u.ID, in.BusinessName)
		if err != nil {
			return err
		}
		m := tenant.Membership{UserID: u.ID, Tenant: t, Role: tenant.RoleOwner, Status: tenant.MemberActive}
		sess, err = s.issue(ctx, tx, u, &m, meta)
		return err
	})
	return sess, err
}

func (s *service) Login(ctx context.Context, in LoginInput, meta ClientMeta) (Session, error) {
	id := strings.TrimSpace(in.Identifier)
	if id == "" || in.Password == "" {
		return Session{}, errBadCredentials
	}
	var email, ph *string
	if strings.Contains(id, "@") {
		e := strings.ToLower(id)
		email = &e
	} else {
		p, err := NormalizePhone(&id)
		if err != nil {
			return Session{}, errBadCredentials
		}
		ph = p
	}
	var sess Session
	err := db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.repo.UserByIdentifier(ctx, tx, email, ph)
		if errors.Is(err, apperr.ErrNotFound) {
			_, _ = pauth.VerifyPassword(in.Password, dummyHash) // equalise timing
			return errBadCredentials
		}
		if err != nil {
			return err
		}
		if u.PasswordHash == nil {
			return errBadCredentials
		}
		ok, err := pauth.VerifyPassword(in.Password, *u.PasswordHash)
		if err != nil || !ok {
			return errBadCredentials
		}
		if err := s.repo.TouchLogin(ctx, tx, u.ID); err != nil {
			return err
		}
		m, err := s.defaultMembership(ctx, u.ID)
		if err != nil {
			return err
		}
		sess, err = s.issue(ctx, tx, u, m, meta)
		return err
	})
	return sess, err
}

// dummyHash is verified when the user does not exist so login timing does not leak accounts.
var dummyHash, _ = pauth.HashPassword("dummy-password-1")

func (s *service) defaultMembership(ctx context.Context, userID uuid.UUID) (*tenant.Membership, error) {
	ms, err := s.tenants.MembershipsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		if m.Status != tenant.MemberDisabled && m.Tenant.Status == "active" {
			return &m, nil
		}
	}
	return nil, nil
}

func (s *service) Refresh(ctx context.Context, refreshToken string, meta ClientMeta) (Session, error) {
	tenantID, ok := splitRefresh(refreshToken)
	if !ok || refreshToken == "" {
		return Session{}, apperr.Unauthenticated("invalid refresh token")
	}
	var sess Session
	var reused bool
	var reusedUser uuid.UUID
	err := db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.repo.RefreshByHash(ctx, tx, pauth.HashToken(refreshToken))
		if err != nil {
			return apperr.Unauthenticated("invalid refresh token")
		}
		if row.RevokedAt != nil {
			reused, reusedUser = true, row.UserID
			return apperr.Unauthenticated("refresh token revoked")
		}
		if time.Now().After(row.ExpiresAt) {
			return apperr.Unauthenticated("refresh token expired")
		}
		if err := s.repo.RevokeRefresh(ctx, tx, row.ID); err != nil {
			return err
		}
		u, err := s.repo.UserByID(ctx, tx, row.UserID)
		if err != nil {
			return err
		}
		var m *tenant.Membership
		if tenantID != uuid.Nil {
			mm, err := s.tenants.Membership(ctx, u.ID, tenantID)
			if err == nil && mm.Status != tenant.MemberDisabled {
				m = &mm
			}
		}
		if m == nil {
			if m, err = s.defaultMembership(ctx, u.ID); err != nil {
				return err
			}
		}
		sess, err = s.issue(ctx, tx, u, m, meta)
		return err
	})
	if reused {
		// A revoked token was replayed: assume theft and revoke every session of the user.
		_ = db.WithTx(context.WithoutCancel(ctx), s.pool, func(ctx context.Context, tx pgx.Tx) error {
			return s.repo.RevokeAllForUser(ctx, tx, reusedUser)
		})
	}
	return sess, err
}

func (s *service) Logout(ctx context.Context, refreshToken string) error {
	return db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.repo.RefreshByHash(ctx, tx, pauth.HashToken(refreshToken))
		if errors.Is(err, apperr.ErrNotFound) {
			return nil // idempotent
		}
		if err != nil {
			return err
		}
		return s.repo.RevokeRefresh(ctx, tx, row.ID)
	})
}

func (s *service) SwitchTenant(ctx context.Context, userID, tenantID uuid.UUID, meta ClientMeta) (Session, error) {
	m, err := s.tenants.Membership(ctx, userID, tenantID)
	if err != nil {
		return Session{}, err
	}
	if m.Status == tenant.MemberDisabled {
		return Session{}, apperr.Forbidden("membership disabled")
	}
	var sess Session
	err = db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.repo.UserByID(ctx, tx, userID)
		if err != nil {
			return err
		}
		sess, err = s.issue(ctx, tx, u, &m, meta)
		return err
	})
	return sess, err
}

func (s *service) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	if err := ValidatePassword(next); err != nil {
		return err
	}
	return db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.repo.UserByID(ctx, tx, userID)
		if err != nil {
			return err
		}
		if u.PasswordHash != nil {
			if ok, _ := pauth.VerifyPassword(current, *u.PasswordHash); !ok {
				return apperr.ValidationField("currentPassword", "current password is wrong")
			}
		}
		hash, err := pauth.HashPassword(next)
		if err != nil {
			return err
		}
		if err := s.repo.SetPassword(ctx, tx, userID, hash); err != nil {
			return err
		}
		return s.repo.RevokeAllForUser(ctx, tx, userID)
	})
}

func (s *service) issue(ctx context.Context, tx pgx.Tx, u User, m *tenant.Membership, meta ClientMeta) (Session, error) {
	p := pauth.Principal{UserID: u.ID, IsPlatformAdmin: u.IsPlatformAdmin}
	if m != nil {
		p.TenantID, p.Role = m.Tenant.ID, m.Role
	}
	access, exp, err := s.tokens.Issue(p)
	if err != nil {
		return Session{}, err
	}
	plain, _, err := pauth.NewRefreshToken()
	if err != nil {
		return Session{}, err
	}
	if p.TenantID != uuid.Nil {
		plain += "." + p.TenantID.String()
	}
	if err := s.repo.InsertRefresh(ctx, tx, u.ID, pauth.HashToken(plain), meta, time.Now().Add(s.refreshTTL)); err != nil {
		return Session{}, err
	}
	return Session{AccessToken: access, RefreshToken: plain, ExpiresAt: exp, User: u, Membership: m}, nil
}

func (s *service) User(ctx context.Context, id uuid.UUID) (u User, err error) {
	err = db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err = s.repo.UserByID(ctx, tx, id)
		return err
	})
	return u, err
}

func (s *service) UsersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]User, error) {
	out := make(map[uuid.UUID]User, len(ids))
	err := db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		us, err := s.repo.UsersByIDs(ctx, tx, ids)
		for _, u := range us {
			out[u.ID] = u
		}
		return err
	})
	return out, err
}

func (s *service) Memberships(ctx context.Context, userID uuid.UUID) ([]tenant.Membership, error) {
	return s.tenants.MembershipsForUser(ctx, userID)
}

func (s *service) FindOrCreateUser(ctx context.Context, email, phone *string, name string) (uuid.UUID, error) {
	e, err := NormalizeEmail(email)
	if err != nil {
		return uuid.Nil, err
	}
	p, err := NormalizePhone(phone)
	if err != nil {
		return uuid.Nil, err
	}
	if e == nil && p == nil {
		return uuid.Nil, apperr.Validation("email or phone is required")
	}
	var id uuid.UUID
	err = db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.repo.UserByIdentifier(ctx, tx, e, p)
		if err == nil {
			id = u.ID
			return nil
		}
		if !errors.Is(err, apperr.ErrNotFound) {
			return err
		}
		u, err = s.repo.InsertUser(ctx, tx, strings.TrimSpace(name), e, p, nil)
		if err != nil {
			return db.MapErr(err, "user")
		}
		id = u.ID
		return nil
	})
	return id, err
}
