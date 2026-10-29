package httpx

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	rl := NewRateLimiter(1, 2)
	rl.now = func() time.Time { return now }
	tests := []struct {
		advance time.Duration
		want    bool
	}{{0, true}, {0, true}, {0, false}, {time.Second, true}, {0, false}}
	for i, tt := range tests {
		now = now.Add(tt.advance)
		if got := rl.Allow("ip"); got != tt.want {
			t.Fatalf("step %d: Allow = %v want %v", i, got, tt.want)
		}
	}
	if !rl.Allow("other") {
		t.Fatal("keys must be independent")
	}
}

func TestRouterHealthAndPlayground(t *testing.T) {
	h := NewRouter(Deps{Logger: slog.Default(), GraphQL: http.NotFoundHandler(), Dev: false, CORSOrigins: []string{"http://x"}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 {
		t.Fatalf("healthz = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/playground", nil))
	if rec.Code != 404 {
		t.Fatalf("playground in prod = %d", rec.Code)
	}
}

func TestTrustedRealIPIgnoresSpoofedHeaders(t *testing.T) {
	var got string
	h := TrustedRealIP([]string{"10.0.0.0/8"})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))
	cases := []struct{ remote, xff, want string }{
		{"203.0.113.5:4000", "1.2.3.4", "203.0.113.5:4000"},                    // untrusted peer: header ignored
		{"10.1.2.3:4000", "198.51.100.7", "198.51.100.7:0"},                    // trusted proxy
		{"10.1.2.3:4000", "1.1.1.1, 198.51.100.7, 10.9.9.9", "198.51.100.7:0"}, // right-most untrusted
		{"10.1.2.3:4000", "not-an-ip", "10.1.2.3:4000"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		r.Header.Set("X-Forwarded-For", c.xff)
		h.ServeHTTP(httptest.NewRecorder(), r)
		if got != c.want {
			t.Errorf("remote %s xff %q -> %s, want %s", c.remote, c.xff, got, c.want)
		}
	}
	// No trusted proxies configured: never rewritten.
	h = TrustedRealIP(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.5:1"
	r.Header.Set("X-Real-IP", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got != "203.0.113.5:1" {
		t.Fatalf("got %s", got)
	}
}

func TestRateLimiterIsBounded(t *testing.T) {
	rl := NewRateLimiter(1, 5)
	rl.max = 1000
	for i := 0; i < 20000; i++ {
		rl.Allow("k" + strconv.Itoa(i))
	}
	if n := rl.Len(); n > 1000 {
		t.Fatalf("buckets = %d", n)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := NewRouter(Deps{Logger: slog.Default(), GraphQL: http.NotFoundHandler(), CORSOrigins: []string{"http://x"}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/graphql", nil))
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %v", rec.Header())
	}
}

// The web app sends X-Auth-Mode and X-Requested-With on every call; a preflight that does not
// allow them makes the browser block the whole site ("cannot reach the server").
func TestCORSPreflightAllowsWebAppHeaders(t *testing.T) {
	h := NewRouter(Deps{Logger: slog.Default(), GraphQL: http.NotFoundHandler(), CORSOrigins: []string{"http://localhost:3000"}})
	req := httptest.NewRequest("OPTIONS", "/graphql", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type,authorization,x-auth-mode,x-requested-with")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the web origin", got)
	}
	allowed := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers"))
	for _, h := range []string{"x-auth-mode", "x-requested-with", "authorization", "content-type"} {
		if !strings.Contains(allowed, h) {
			t.Fatalf("Access-Control-Allow-Headers %q is missing %s", allowed, h)
		}
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("credentials must be allowed for the refresh cookie")
	}
}
