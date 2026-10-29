package auth

import (
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

// Claims of an access token: sub = user id, tid = tenant id, role = membership role.
type Claims struct {
	TenantID        string `json:"tid,omitempty"`
	Role            string `json:"role,omitempty"`
	IsPlatformAdmin bool   `json:"pa,omitempty"`
	jwt.RegisteredClaims
}

// TokenIssuer signs and verifies HS256 access tokens.
type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
	issuer string
	now    func() time.Time
}

func NewTokenIssuer(secret string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), ttl: ttl, issuer: "provider_saas", now: time.Now}
}

// Issue creates an access token for the principal.
func (t *TokenIssuer) Issue(p Principal) (string, time.Time, error) {
	now := t.now()
	exp := now.Add(t.ttl)
	claims := Claims{
		Role:            p.Role,
		IsPlatformAdmin: p.IsPlatformAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.UserID.String(),
			Issuer:    t.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	}
	if p.TenantID != uuid.Nil {
		claims.TenantID = p.TenantID.String()
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	return s, exp, err
}

// Parse validates the token and returns the Principal it carries.
func (t *TokenIssuer) Parse(token string) (Principal, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
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
	p := Principal{UserID: uid, Role: c.Role, IsPlatformAdmin: c.IsPlatformAdmin}
	if c.TenantID != "" {
		if p.TenantID, err = uuid.Parse(c.TenantID); err != nil {
			return Principal{}, errors.New("invalid token tenant")
		}
	}
	return p, nil
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
