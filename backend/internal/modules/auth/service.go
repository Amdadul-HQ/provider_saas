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
	"provider_saas/internal/platform/lockout"
	"provider_saas/internal/platform/logger"
)

// Tenants is what auth needs from the tenant module.
type Tenants interface {
	Provision(ctx context.Context, ownerID uuid.UUID, businessName string) (tenant.Tenant, error)
	MembershipsForUser(ctx context.Context, userID uuid.UUID) ([]tenant.Membership, error)
	Membership(ctx context.Context, userID, tenantID uuid.UUID) (tenant.Membership, error)
	InviteByToken(ctx context.Context, token string) (tenant.InviteInfo, error)
	ActivateInvite(ctx context.Context, inv tenant.InviteInfo, token string) error
}

// Service is the auth module API.
type Service interface {
	Register(ctx context.Context, in RegisterInput, meta ClientMeta) (Session, error)
	Login(ctx context.Context, in LoginInput, meta ClientMeta) (Session, error)
	Refresh(ctx context.Context, refreshToken string, meta ClientMeta) (Session, error)
	Logout(ctx context.Context, refreshToken string) error
	// LogoutAll revokes every refresh token of the user and bumps token_version, so every
	// access token issued before is rejected at once (SECURITY L2).
	LogoutAll(ctx context.Context, userID uuid.UUID) error
	// BumpTokenVersion implements tenant.UserDirectory (member removed / role changed).
	BumpTokenVersion(ctx context.Context, userID uuid.UUID) error
	SwitchTenant(ctx context.Context, userID, tenantID uuid.UUID, meta ClientMeta) (Session, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error
	User(ctx context.Context, id uuid.UUID) (User, error)
	UsersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]User, error)
	Memberships(ctx context.Context, userID uuid.UUID) ([]tenant.Membership, error)
	// FindOrCreateUser implements tenant.UserDirectory.
	FindOrCreateUser(ctx context.Context, email, phone *string, name string) (uuid.UUID, error)
	// Invite is the public preview of a pending invite (masked contact hints).
	Invite(ctx context.Context, token string) (InvitePreview, error)
	// AcceptInvite activates an invitation: a new user sets name + password, an existing
	// user proves ownership with their password. Returns a session in the invited tenant.
	AcceptInvite(ctx context.Context, token, password string, name *string, meta ClientMeta) (Session, error)
}

type service struct {
	pool       *pgxpool.Pool
	repo       Repository
	tokens     *pauth.TokenIssuer
	refreshTTL time.Duration
	tenants    Tenants
	// limiter locks an identifier (and a client IP) after repeated failures (SECURITY H6).
	limiter   lockout.Limiter
	ipLimiter lockout.Limiter
	// versions caches users.token_version for the access-token check; nil-safe.
	versions *pauth.VersionCache
}

// NewService builds the auth service. limiter nil = bounded in-memory login lockout
// (5 failures / 15 min per identifier); the per-IP limiter allows 30 failures / 15 min.
func NewService(pool *pgxpool.Pool, tokens *pauth.TokenIssuer, refreshTTL time.Duration, tenants Tenants, limiter, ipLimiter lockout.Limiter,
	versions *pauth.VersionCache) Service {
	if limiter == nil {
		limiter = lockout.NewMemory(lockout.DefaultLogin, 100_000)
	}
	if ipLimiter == nil {
		ipLimiter = lockout.NewMemory(IPPolicy, 100_000)
	}
	return &service{pool: pool, tokens: tokens, refreshTTL: refreshTTL, tenants: tenants, limiter: limiter, ipLimiter: ipLimiter,
		versions: versions}
}

// IPPolicy bounds failed logins per client IP (many identifiers from one address).
var IPPolicy = lockout.Policy{MaxFailures: 30, Window: 15 * time.Minute, LockFor: 15 * time.Minute}

// NewUserDirectory returns a tenant.UserDirectory without the tenant dependency
// (breaks the construction cycle auth <-> tenant).
func NewUserDirectory(pool *pgxpool.Pool, versions *pauth.VersionCache) tenant.UserDirectory {
	return &service{pool: pool, versions: versions}
}

// TokenVersionLoader reads users.token_version for pauth.NewVersionCache.
func TokenVersionLoader(pool *pgxpool.Pool) func(ctx context.Context, userID uuid.UUID) (int, error) {
	return func(ctx context.Context, userID uuid.UUID) (v int, err error) {
		err = db.WithPlatformTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			v, err = Repository{}.TokenVersion(ctx, tx, userID)
			return err
		})
		return v, err
	}
}

// errRegister is returned for every register attempt that cannot create an account
// because the email/phone is taken (SECURITY L1): it never says which detail exists and
// carries no session. Repeated attempts from one IP are rate-limited. Field "account" (not
// a form field) lets the web app offer "sign in instead" rather than "check your input".
var errRegister = &apperr.Error{Kind: apperr.ErrValidation, Field: "account",
	Msg: "we could not create an account with these details; if you already have an account, sign in or ask your business owner for an invite"}

// RegisterIPPolicy bounds register attempts that hit an existing account per client IP.
var RegisterIPPolicy = lockout.Policy{MaxFailures: 10, Window: time.Hour, LockFor: time.Hour}

var errBadCredentials = apperr.Unauthenticated("invalid credentials")

func (s *service) Register(ctx context.Context, in RegisterInput, meta ClientMeta) (Session, error) {
	regKey := "register-ip:" + meta.IP
	if meta.IP != "" {
		if l, _, err := s.ipLimiter.Locked(ctx, regKey); err != nil {
			return Session{}, err
		} else if l {
			return Session{}, errLocked
		}
	}
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
	err = db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.repo.InsertUser(ctx, tx, in.Name, email, ph, &hash)
		if err != nil {
			if db.IsUniqueViolation(err, "") {
				return errRegister
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
	if errors.Is(err, errRegister) {
		if meta.IP != "" {
			if _, ferr := s.ipLimiter.Fail(ctx, regKey); ferr != nil {
				logger.From(ctx).Warn("register limiter", "err", ferr)
			}
		}
		s.auditPlatform(ctx, nil, "auth.register_duplicate", map[string]any{"ip": meta.IP})
		return Session{}, errRegister
	}
	return sess, err
}

var errLocked = apperr.RateLimited("too many failed sign-in attempts; try again in 15 minutes")

func (s *service) Login(ctx context.Context, in LoginInput, meta ClientMeta) (Session, error) {
	id := strings.TrimSpace(in.Identifier)
	if id == "" || in.Password == "" {
		return Session{}, errBadCredentials
	}
	var email, ph *string
	if strings.Contains(id, "@") {
		e := strings.ToLower(id)
		email = &e
	} else if p, err := NormalizePhone(&id); err == nil {
		ph = p
	} else {
		e := strings.ToLower(id) // unknown format: still counted against its own key
		email = &e
	}
	idKey := "login:" + pauth.HashToken(deref(email)+"|"+deref(ph))
	ipKey := "ip:" + meta.IP
	if locked, err := s.isLocked(ctx, idKey, ipKey, meta.IP != ""); err != nil {
		return Session{}, err
	} else if locked {
		return Session{}, errLocked // not audited per attempt (the lock itself was)
	}
	var sess Session
	var failedUser *uuid.UUID
	err := db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.repo.UserByIdentifier(ctx, tx, email, ph)
		if errors.Is(err, apperr.ErrNotFound) {
			_, _ = pauth.VerifyPassword(in.Password, dummyHash) // equalise timing
			return errBadCredentials
		}
		if err != nil {
			return err
		}
		if u.PasswordHash == nil { // invited user who has not accepted yet (L1: same timing)
			_, _ = pauth.VerifyPassword(in.Password, dummyHash)
			failedUser = &u.ID
			return errBadCredentials
		}
		ok, err := pauth.VerifyPassword(in.Password, *u.PasswordHash)
		if err != nil || !ok {
			failedUser = &u.ID
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
	if errors.Is(err, errBadCredentials) {
		s.loginFailed(ctx, idKey, ipKey, meta, failedUser)
		return Session{}, errBadCredentials
	}
	if err == nil {
		_ = s.limiter.Reset(ctx, idKey)
	}
	return sess, err
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *service) isLocked(ctx context.Context, idKey, ipKey string, checkIP bool) (bool, error) {
	if l, _, err := s.limiter.Locked(ctx, idKey); err != nil || l {
		return l, err
	}
	if !checkIP {
		return false, nil
	}
	l, _, err := s.ipLimiter.Locked(ctx, ipKey)
	return l, err
}

// loginFailed counts the failure and writes a platform audit row (SECURITY M9). The
// identifier is stored only as a hash.
func (s *service) loginFailed(ctx context.Context, idKey, ipKey string, meta ClientMeta, userID *uuid.UUID) {
	locked, err := s.limiter.Fail(ctx, idKey)
	if err != nil {
		logger.From(ctx).Warn("login limiter", "err", err)
	}
	if meta.IP != "" {
		if _, err := s.ipLimiter.Fail(ctx, ipKey); err != nil {
			logger.From(ctx).Warn("login ip limiter", "err", err)
		}
	}
	m := map[string]any{"identifier_hash": idKey, "ip": meta.IP}
	s.auditPlatform(ctx, userID, "auth.login_failed", m)
	if locked {
		s.auditPlatform(ctx, userID, "auth.login_locked", m)
	}
}

// auditPlatform writes a tenant-less audit row in its own transaction (it must survive
// the rollback of the failed login).
func (s *service) auditPlatform(ctx context.Context, userID *uuid.UUID, action string, meta map[string]any) {
	err := db.WithPlatformTx(context.WithoutCancel(ctx), s.pool, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.InsertPlatformAudit(ctx, tx, userID, action, meta)
	})
	if err != nil {
		logger.From(ctx).Error("audit "+action, "err", err)
	}
}

// dummyHash is verified when the user does not exist so login timing does not leak accounts.
var dummyHash, _ = pauth.HashPassword("dummy-password-1")

func (s *service) defaultMembership(ctx context.Context, userID uuid.UUID) (*tenant.Membership, error) {
	ms, err := s.tenants.MembershipsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		if m.Status == tenant.MemberActive && m.Tenant.Status == "active" {
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
	err := db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
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
			if err == nil && mm.Status == tenant.MemberActive && mm.Tenant.Status == "active" {
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
		_ = db.WithPlatformTx(context.WithoutCancel(ctx), s.pool, func(ctx context.Context, tx pgx.Tx) error {
			return s.revokeEverything(ctx, tx, reusedUser)
		})
		s.versions.Invalidate(reusedUser)
	}
	return sess, err
}

func (s *service) Logout(ctx context.Context, refreshToken string) error {
	return db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
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
	if m.Status != tenant.MemberActive {
		return Session{}, apperr.Forbidden("membership is not active")
	}
	if m.Tenant.Status != "active" {
		return Session{}, apperr.Forbidden("this business is suspended")
	}
	var sess Session
	err = db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
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
	return db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
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
		return s.revokeEverything(ctx, tx, userID)
	})
}

// revokeEverything revokes all refresh tokens and bumps token_version (in tx); the
// cached version is dropped so this process rejects old access tokens immediately.
func (s *service) revokeEverything(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	if err := s.repo.RevokeAllForUser(ctx, tx, userID); err != nil {
		return err
	}
	if err := s.repo.BumpTokenVersion(ctx, tx, userID); err != nil {
		return err
	}
	s.versions.Invalidate(userID)
	return nil
}

func (s *service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	err := db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		return s.revokeEverything(ctx, tx, userID)
	})
	s.versions.Invalidate(userID) // again after commit: a concurrent reload may have cached the old value
	if err == nil {
		s.auditPlatform(ctx, &userID, "auth.logout_all", map[string]any{})
	}
	return err
}

// BumpTokenVersion joins the caller's transaction (tenant member changes). Refresh tokens
// stay valid: the next refresh re-reads membership and role.
func (s *service) BumpTokenVersion(ctx context.Context, userID uuid.UUID) error {
	err := db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.BumpTokenVersion(ctx, tx, userID)
	})
	s.versions.Invalidate(userID)
	return err
}

func (s *service) issue(ctx context.Context, tx pgx.Tx, u User, m *tenant.Membership, meta ClientMeta) (Session, error) {
	p := pauth.Principal{UserID: u.ID, IsPlatformAdmin: u.IsPlatformAdmin, TokenVersion: u.TokenVersion}
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
	err = db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err = s.repo.UserByID(ctx, tx, id)
		return err
	})
	return u, err
}

func (s *service) UsersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]User, error) {
	out := make(map[uuid.UUID]User, len(ids))
	err := db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
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
	err = db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
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

// ---- invites (SPRINT-01-QA BUG-02/BUG-03)

var errInviteLink = apperr.ValidationField("token", "this invite link is invalid or has expired; ask for a new one")

func (s *service) Invite(ctx context.Context, token string) (InvitePreview, error) {
	inv, err := s.tenants.InviteByToken(ctx, token)
	if err != nil {
		return InvitePreview{}, err
	}
	return InvitePreview{TenantName: inv.TenantName, Role: inv.Role, InviteeName: inv.InviteeName,
		EmailHint: tenant.MaskEmail(inv.Email), PhoneHint: tenant.MaskPhone(inv.Phone), ExpiresAt: inv.ExpiresAt,
		UserExists: inv.UserHasPassword}, nil
}

func (s *service) AcceptInvite(ctx context.Context, token, password string, name *string, meta ClientMeta) (Session, error) {
	inv, err := s.tenants.InviteByToken(ctx, token)
	if errors.Is(err, apperr.ErrNotFound) {
		return Session{}, errInviteLink
	}
	if err != nil {
		return Session{}, err
	}
	key := "accept:" + inv.UserID.String()
	if l, _, err := s.limiter.Locked(ctx, key); err != nil {
		return Session{}, err
	} else if l {
		return Session{}, errLocked
	}
	var sess Session
	err = db.WithPlatformTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.repo.UserByID(ctx, tx, inv.UserID)
		if err != nil {
			return err
		}
		if u.PasswordHash != nil {
			// Existing account: prove ownership; the profile is not changed.
			if ok, _ := pauth.VerifyPassword(password, *u.PasswordHash); !ok {
				return errBadCredentials
			}
		} else {
			if err := ValidatePassword(password); err != nil {
				return err
			}
			hash, err := pauth.HashPassword(password)
			if err != nil {
				return err
			}
			if err := s.repo.SetPassword(ctx, tx, u.ID, hash); err != nil {
				return err
			}
			if name != nil && strings.TrimSpace(*name) != "" {
				n := strings.TrimSpace(*name)
				if len([]rune(n)) > 100 {
					return apperr.ValidationField("name", "max 100 characters")
				}
				if err := s.repo.SetName(ctx, tx, u.ID, n); err != nil {
					return err
				}
				u.Name = n
			}
		}
		if err := s.tenants.ActivateInvite(ctx, inv, token); err != nil {
			if errors.Is(err, apperr.ErrNotFound) {
				return errInviteLink
			}
			return err
		}
		m, err := s.tenants.Membership(ctx, u.ID, inv.TenantID)
		if err != nil {
			return err
		}
		sess, err = s.issue(ctx, tx, u, &m, meta)
		return err
	})
	if errors.Is(err, errBadCredentials) {
		if _, ferr := s.limiter.Fail(ctx, key); ferr != nil {
			logger.From(ctx).Warn("accept limiter", "err", ferr)
		}
		s.auditPlatform(ctx, &inv.UserID, "auth.invite_accept_failed", map[string]any{"ip": meta.IP})
	}
	return sess, err
}
