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

// Middleware puts the Principal from a valid `Authorization: Bearer` token into the
// request context. Missing/invalid tokens leave the request anonymous; the GraphQL
// @auth directive rejects anonymous calls with UNAUTHENTICATED.
func Middleware(issuer *TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if tok, ok := strings.CutPrefix(h, "Bearer "); ok && tok != "" {
				if p, err := issuer.Parse(strings.TrimSpace(tok)); err == nil {
					r = r.WithContext(WithPrincipal(r.Context(), p))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
