package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Refresh-token cookie (SECURITY M6). In cookie mode the refresh token never reaches
// JavaScript: login/register/acceptInvite/switchTenant/refreshToken set it as an
// httpOnly cookie scoped to /graphql and return refreshToken: null.
const (
	RefreshCookie = "psaas_rt"
	// AuthModeHeader: "X-Auth-Mode: cookie" selects cookie mode; anything else keeps the
	// refresh token in the response body (API clients, mobile).
	AuthModeHeader = "X-Auth-Mode"
	// CSRFHeader must be "XMLHttpRequest" on /graphql requests that carry the cookie,
	// unless their Origin is an allowed CORS origin.
	CSRFHeader = "X-Requested-With"
	cookiePath = "/graphql"
)

// CookieOptions configures the refresh cookie.
type CookieOptions struct {
	Secure bool          // false only in development/test (plain http://localhost)
	MaxAge time.Duration // refresh-token TTL
}

type httpCtx struct {
	w    http.ResponseWriter
	r    *http.Request
	opts CookieOptions
}

type httpKey struct{}

// WithHTTP stores the response writer and request so resolvers can read and set cookies.
func WithHTTP(ctx context.Context, w http.ResponseWriter, r *http.Request, opts CookieOptions) context.Context {
	return context.WithValue(ctx, httpKey{}, &httpCtx{w: w, r: r, opts: opts})
}

func httpFrom(ctx context.Context) (*httpCtx, bool) {
	h, ok := ctx.Value(httpKey{}).(*httpCtx)
	return h, ok && h != nil
}

// HTTPMiddleware installs WithHTTP for every request.
func HTTPMiddleware(opts CookieOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(WithHTTP(r.Context(), w, r, opts)))
		})
	}
}

// CookieMode reports whether the client asked for cookie mode (X-Auth-Mode: cookie).
func CookieMode(ctx context.Context) bool {
	h, ok := httpFrom(ctx)
	return ok && strings.EqualFold(strings.TrimSpace(h.r.Header.Get(AuthModeHeader)), "cookie")
}

// RefreshFromCookie returns the refresh token carried by the psaas_rt cookie ("" if none).
func RefreshFromCookie(ctx context.Context) string {
	h, ok := httpFrom(ctx)
	if !ok {
		return ""
	}
	c, err := h.r.Cookie(RefreshCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func (h *httpCtx) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: RefreshCookie, Value: value, Path: cookiePath, MaxAge: maxAge,
		HttpOnly: true, Secure: h.opts.Secure, SameSite: http.SameSiteLaxMode}
}

// SetRefreshCookie sets psaas_rt (HttpOnly, SameSite=Lax, Path=/graphql, Secure outside
// development, Max-Age = refresh TTL). Reports false when there is no HTTP context.
func SetRefreshCookie(ctx context.Context, token string) bool {
	h, ok := httpFrom(ctx)
	if !ok {
		return false
	}
	http.SetCookie(h.w, h.cookie(token, int(h.opts.MaxAge.Seconds())))
	return true
}

// ClearRefreshCookie expires psaas_rt.
func ClearRefreshCookie(ctx context.Context) {
	if h, ok := httpFrom(ctx); ok {
		http.SetCookie(h.w, h.cookie("", -1))
	}
}

// CSRFGuard rejects state-carrying /graphql requests that hold the refresh cookie but
// neither send "X-Requested-With: XMLHttpRequest" nor come from an allowed Origin
// (defence in depth on top of SameSite=Lax and the JSON-only POST transport). A
// cross-site page can send neither: the custom header needs a CORS preflight that only
// allowed origins pass.
func CSRFGuard(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodOptions {
				if _, err := r.Cookie(RefreshCookie); err == nil && !csrfOK(r, allowedOrigins) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"errors":[{"message":"missing X-Requested-With header","extensions":{"code":"CSRF_REJECTED"}}]}`))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csrfOK(r *http.Request, allowed []string) bool {
	if r.Header.Get(CSRFHeader) == "XMLHttpRequest" {
		return true
	}
	o := r.Header.Get("Origin")
	return o != "" && o != "null" && slices.Contains(allowed, o)
}
