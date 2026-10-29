package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Session cookies (SECURITY M6, ADR-011). In cookie mode no token reaches JavaScript:
// login/register/acceptInvite/switchTenant/refreshToken set the access JWT (psaas_at) and
// the refresh token (psaas_rt) as httpOnly cookies and return both tokens as null.
const (
	RefreshCookie = "psaas_rt"
	AccessCookie  = "psaas_at"
	// AuthModeHeader: "X-Auth-Mode: cookie" selects cookie mode; anything else keeps the
	// refresh token in the response body (API clients, mobile).
	AuthModeHeader = "X-Auth-Mode"
	// CSRFHeader must be "XMLHttpRequest" on /graphql requests that carry the cookie,
	// unless their Origin is an allowed CORS origin.
	CSRFHeader = "X-Requested-With"
	cookiePath       = "/graphql" // refresh token: only the refreshToken/logout mutations need it
	accessCookiePath = "/"        // access token: /graphql and future authenticated REST routes
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

func (h *httpCtx) accessCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: AccessCookie, Value: value, Path: accessCookiePath, MaxAge: maxAge,
		HttpOnly: true, Secure: h.opts.Secure, SameSite: http.SameSiteLaxMode}
}

// AccessFromRequest returns the access JWT carried by the psaas_at cookie ("" if none).
func AccessFromRequest(r *http.Request) string {
	c, err := r.Cookie(AccessCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// SetAccessCookie sets psaas_at (HttpOnly, SameSite=Lax, Path=/, Secure outside development)
// so it expires with the JWT. Reports false when there is no HTTP context.
func SetAccessCookie(ctx context.Context, token string, expiresAt time.Time) bool {
	h, ok := httpFrom(ctx)
	if !ok {
		return false
	}
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge <= 0 {
		maxAge = -1
	}
	http.SetCookie(h.w, h.accessCookie(token, maxAge))
	return true
}

// ClearAuthCookies expires both session cookies (logout, failed refresh).
func ClearAuthCookies(ctx context.Context) {
	if h, ok := httpFrom(ctx); ok {
		http.SetCookie(h.w, h.cookie("", -1))
		http.SetCookie(h.w, h.accessCookie("", -1))
	}
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

// CSRFGuard rejects state-carrying /graphql requests that hold a session cookie but
// neither send "X-Requested-With: XMLHttpRequest" nor come from an allowed Origin
// (defence in depth on top of SameSite=Lax and the JSON-only POST transport). A
// cross-site page can send neither: the custom header needs a CORS preflight that only
// allowed origins pass.
func CSRFGuard(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodOptions {
				if hasSessionCookie(r) && !csrfOK(r, allowedOrigins) {
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

func hasSessionCookie(r *http.Request) bool {
	for _, name := range []string{RefreshCookie, AccessCookie} {
		if _, err := r.Cookie(name); err == nil {
			return true
		}
	}
	return false
}

func csrfOK(r *http.Request, allowed []string) bool {
	if r.Header.Get(CSRFHeader) == "XMLHttpRequest" {
		return true
	}
	o := r.Header.Get("Origin")
	return o != "" && o != "null" && slices.Contains(allowed, o)
}
