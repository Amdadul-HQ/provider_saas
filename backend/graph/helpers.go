package graph

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"provider_saas/graph/model"
	modauth "provider_saas/internal/modules/auth"
	"provider_saas/internal/platform/apperr"
	pauth "provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/pagination"
)

var errNotImplemented = errors.New("not implemented")

func principal(ctx context.Context) (pauth.Principal, error) {
	p, ok := pauth.FromContext(ctx)
	if !ok {
		return p, apperr.Unauthenticated("authentication required")
	}
	return p, nil
}

// tenantOf returns the caller's tenant and user ids.
func tenantOf(ctx context.Context) (tenantID, userID uuid.UUID, err error) {
	p, err := principal(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if p.TenantID == uuid.Nil {
		return uuid.Nil, p.UserID, apperr.Forbidden("no business selected; register a business or switch tenant")
	}
	return p.TenantID, p.UserID, nil
}

// pageOf is the one way a resolver reads a PageInput: first is clamped and after may be a
// Connection endCursor or the last row id of an array list (ADR-010).
func pageOf(p *model.PageInput) (pagination.Page, error) {
	var first *int
	var after *string
	if p != nil {
		first, after = p.First, p.After
	}
	page, err := pagination.New(first, after)
	if err != nil {
		return page, apperr.ValidationField("page.after", "invalid cursor")
	}
	return page, nil
}

func pageInfo[T any](items []T, more bool, cursor func(T) string) model.PageInfo {
	pi := model.PageInfo{HasNextPage: more}
	if len(items) > 0 {
		c := cursor(items[len(items)-1])
		pi.EndCursor = &c
	}
	return pi
}

// Enum helpers: DB stores lower snake_case, GraphQL uses UPPER_SNAKE.
func lower[T ~string](v T) string { return strings.ToLower(string(v)) }

func lowerPtr[T ~string](v *T) *string {
	if v == nil {
		return nil
	}
	s := lower(*v)
	return &s
}

func upper[T ~string](s string) T { return T(strings.ToUpper(s)) }

func upperPtr[T ~string](s *string) *T {
	if s == nil {
		return nil
	}
	v := upper[T](*s)
	return &v
}

func deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

type clientMetaKey struct{}

// WithClientMeta stores user agent / IP for refresh-token bookkeeping.
func WithClientMeta(ctx context.Context, r *http.Request) context.Context {
	ip := r.RemoteAddr
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	if net.ParseIP(ip) == nil {
		ip = ""
	}
	ua := r.UserAgent()
	if len(ua) > 300 {
		ua = ua[:300]
	}
	return context.WithValue(ctx, clientMetaKey{}, modauth.ClientMeta{UserAgent: ua, IP: ip})
}

func clientMeta(ctx context.Context) modauth.ClientMeta {
	m, _ := ctx.Value(clientMetaKey{}).(modauth.ClientMeta)
	return m
}

func principalOK(ctx context.Context) (pauth.Principal, bool) { return pauth.FromContext(ctx) }

// firstOf normalises only the page size, for lists that are not keyset-paged (products by
// sort order, audit logs by bigint id). Everything else uses pageOf (ADR-010).
func firstOf(p *model.PageInput) int {
	pg, _ := pagination.New(pageSizeOf(p), nil)
	return pg.First
}

func pageSizeOf(p *model.PageInput) *int {
	if p == nil {
		return nil
	}
	return p.First
}

// SettingStaffCanVerify (tenants.settings, default true) lets STAFF verify payments (ADR-007).
const SettingStaffCanVerify = "staff_can_verify_payments"

// requirePaymentVerifier enforces ADR-007 for anything that verifies money or marks it
// received: OWNER/ADMIN always, STAFF unless settings.staff_can_verify_payments is false,
// RESELLER/VIEWER never.
func (r *Resolver) requirePaymentVerifier(ctx context.Context, tenantID uuid.UUID) error {
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	switch {
	case p.IsPlatformAdmin || p.HasRole("owner", "admin"):
		return nil
	case p.HasRole("staff"):
		t, err := r.TenantSvc.Get(ctx, tenantID)
		if err != nil {
			return err
		}
		if v, ok := t.Settings[SettingStaffCanVerify].(bool); ok && !v {
			return apperr.Forbidden("staff may not verify payments in this business")
		}
		return nil
	}
	return apperr.Forbidden("your role cannot verify payments")
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
