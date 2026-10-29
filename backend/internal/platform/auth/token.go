package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Audiences: an access token is only accepted by the API, an events token only by the
// SSE endpoint (different audience AND different HKDF key).
const (
	AudienceAPI    = "provider_saas/api"
	AudienceEvents = "provider_saas/events"
)

// Claims of an access token: sub = user id, tid = tenant id, role = membership role,
// tv = users.token_version at issue time (SECURITY L2: bumping it revokes the token).
type Claims struct {
	TenantID        string `json:"tid,omitempty"`
	Role            string `json:"role,omitempty"`
	IsPlatformAdmin bool   `json:"pa,omitempty"`
	TokenVersion    int    `json:"tv"`
	jwt.RegisteredClaims
}

// TokenIssuer signs and verifies HS256 access tokens and SSE events tokens.
type TokenIssuer struct {
	secret    []byte // JWTKey
	eventsKey []byte // EventsTokenKey
	ttl       time.Duration
	issuer    string
	now       func() time.Time
	versions  VersionSource
}

// VersionSource returns the current users.token_version (cached; see VersionCache).
type VersionSource interface {
	TokenVersion(ctx context.Context, userID uuid.UUID) (int, error)
}

// NewTokenIssuer derives its keys from secret (JWT_SECRET) with HKDF (Keys).
func NewTokenIssuer(secret string, ttl time.Duration) *TokenIssuer {
	k := NewKeys(secret)
	return &TokenIssuer{secret: k.JWTKey(), eventsKey: k.EventsTokenKey(), ttl: ttl, issuer: "provider_saas", now: time.Now}
}

// WithVersions makes Verify/ParseEventsTokenCtx reject tokens whose tv is older than the
// user's current token_version (logoutAll, password change, role change, removal).
func (t *TokenIssuer) WithVersions(v VersionSource) *TokenIssuer {
	t.versions = v
	return t
}

// TTL is the access token lifetime.
func (t *TokenIssuer) TTL() time.Duration { return t.ttl }

func (t *TokenIssuer) sign(key []byte, aud string, p Principal, ttl time.Duration) (string, time.Time, error) {
	now := t.now()
	exp := now.Add(ttl)
	claims := Claims{
		Role:            p.Role,
		IsPlatformAdmin: p.IsPlatformAdmin,
		TokenVersion:    p.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.UserID.String(),
			Issuer:    t.issuer,
			Audience:  jwt.ClaimStrings{aud},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	}
	if p.TenantID != uuid.Nil {
		claims.TenantID = p.TenantID.String()
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	return s, exp, err
}

func (t *TokenIssuer) parse(key []byte, aud, token string) (Principal, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithAudience(aud),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return Principal{}, fmt.Errorf("invalid token: %w", err)
	}
	uid, err := uuid.Parse(c.Subject)
	if err != nil {
		return Principal{}, errors.New("invalid token subject")
	}
	p := Principal{UserID: uid, Role: c.Role, IsPlatformAdmin: c.IsPlatformAdmin, TokenVersion: c.TokenVersion}
	if c.TenantID != "" {
		if p.TenantID, err = uuid.Parse(c.TenantID); err != nil {
			return Principal{}, errors.New("invalid token tenant")
		}
	}
	return p, nil
}

// Issue creates an access token for the principal.
func (t *TokenIssuer) Issue(p Principal) (string, time.Time, error) {
	return t.sign(t.secret, AudienceAPI, p, t.ttl)
}

// Parse validates signature, issuer, audience and expiry (no revocation check; see Verify).
func (t *TokenIssuer) Parse(token string) (Principal, error) { return t.parse(t.secret, AudienceAPI, token) }

// ErrTokenRevoked: the token predates the user's current token_version.
var ErrTokenRevoked = errors.New("token revoked")

// Verify is Parse plus the token_version revocation check (when a VersionSource is set).
func (t *TokenIssuer) Verify(ctx context.Context, token string) (Principal, error) {
	p, err := t.Parse(token)
	if err != nil {
		return p, err
	}
	return p, t.checkVersion(ctx, p)
}

func (t *TokenIssuer) checkVersion(ctx context.Context, p Principal) error {
	if t.versions == nil {
		return nil
	}
	cur, err := t.versions.TokenVersion(ctx, p.UserID)
	if err != nil {
		return fmt.Errorf("token version: %w", err)
	}
	if p.TokenVersion != cur {
		return ErrTokenRevoked
	}
	return nil
}

// EventsTokenTTL is the lifetime of an SSE events token (createEventsToken).
const EventsTokenTTL = 5 * time.Minute

// NewEventsToken signs a short-lived token for GET /events?token=... (EventSource cannot
// send an Authorization header). Separate HKDF key and audience: it is never accepted as
// an access token, and an access token is never accepted by the events endpoint.
func (t *TokenIssuer) NewEventsToken(p Principal, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 || ttl > EventsTokenTTL {
		ttl = EventsTokenTTL
	}
	return t.sign(t.eventsKey, AudienceEvents, p, ttl)
}

// ParseEventsToken validates an events token and applies the token_version check, so
// logoutAll / member removal also cuts an open SSE stream at its next reconnect.
func (t *TokenIssuer) ParseEventsToken(ctx context.Context, token string) (Principal, error) {
	p, err := t.parse(t.eventsKey, AudienceEvents, token)
	if err != nil {
		return p, err
	}
	return p, t.checkVersion(ctx, p)
}

// NewRefreshToken returns an opaque random token and its storage hash.
func NewRefreshToken() (plain, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(b)
	return plain, HashToken(plain), nil
}

// HashToken is the sha256 hex used to store refresh/portal tokens.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
