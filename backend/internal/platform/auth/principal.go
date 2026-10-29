package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Roles as stored in tenant_members.role.
const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleStaff    = "staff"
	RoleReseller = "reseller"
	RoleViewer   = "viewer"
)

// Principal is the authenticated caller.
type Principal struct {
	UserID          uuid.UUID
	TenantID        uuid.UUID
	Role            string
	IsPlatformAdmin bool
	// TokenVersion is users.token_version when the token was issued (SECURITY L2).
	TokenVersion int
}

// HasRole reports whether the principal has one of roles (case-insensitive). Empty roles = any.
func (p Principal) HasRole(roles ...string) bool {
	if len(roles) == 0 {
		return true
	}
	for _, r := range roles {
		if strings.EqualFold(r, p.Role) {
			return true
		}
	}
	return false
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok && p.UserID != uuid.Nil
}

// Middleware puts the Principal from a valid access token into the request context: an
// `Authorization: Bearer` header (API and mobile clients) wins, else the psaas_at cookie
// (web app, ADR-011). Missing/invalid tokens leave the request anonymous; the GraphQL @auth
// directive rejects anonymous calls with UNAUTHENTICATED. Cookie-carrying requests must
// also pass CSRFGuard.
func Middleware(issuer *TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tok := requestToken(r); tok != "" {
				// Verify = signature/aud/exp + token_version: a revoked token is anonymous.
				if p, err := issuer.Verify(r.Context(), tok); err == nil {
					r = r.WithContext(WithPrincipal(r.Context(), p))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestToken(r *http.Request) string {
	if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		if tok = strings.TrimSpace(tok); tok != "" {
			return tok
		}
	}
	return AccessFromRequest(r)
}

// ResellerScope returns the caller's user id when the caller is a RESELLER (ADR-007):
// repositories then restrict customers, subscriptions and orders to rows the reseller
// owns (owner_user_id / sold_by / created_by). Background jobs carry no principal and
// are never scoped.
func ResellerScope(ctx context.Context) (uuid.UUID, bool) {
	p, ok := FromContext(ctx)
	if !ok || p.IsPlatformAdmin || !strings.EqualFold(p.Role, RoleReseller) {
		return uuid.Nil, false
	}
	return p.UserID, true
}
