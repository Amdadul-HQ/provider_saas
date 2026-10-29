package auth

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestKeysAreSeparated(t *testing.T) {
	k := NewKeys("0123456789abcdef0123456789abcdef")
	all := [][]byte{k.JWTKey(), k.OAuthStateKey(), k.TelegramSecretKey(), k.EventsTokenKey(), []byte("0123456789abcdef0123456789abcdef")}
	for i := range all {
		if len(all[i]) < 32 {
			t.Fatalf("key %d too short", i)
		}
		for j := i + 1; j < len(all); j++ {
			if bytes.Equal(all[i], all[j]) {
				t.Fatalf("keys %d and %d are equal", i, j)
			}
		}
	}
	if !bytes.Equal(k.JWTKey(), NewKeys("0123456789abcdef0123456789abcdef").JWTKey()) {
		t.Fatal("derivation not deterministic")
	}
}

type fixedVersions map[uuid.UUID]int

func (f fixedVersions) TokenVersion(_ context.Context, id uuid.UUID) (int, error) { return f[id], nil }

func TestTokenVersionAudienceAndEvents(t *testing.T) {
	uid := uuid.New()
	versions := fixedVersions{uid: 3}
	iss := NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Minute).WithVersions(versions)
	p := Principal{UserID: uid, TenantID: uuid.New(), Role: RoleOwner, TokenVersion: 3}
	tok, _, err := iss.Issue(p)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := iss.Verify(context.Background(), tok); err != nil || got != p {
		t.Fatalf("verify current version: %+v %v", got, err)
	}
	versions[uid] = 4 // logoutAll / role change
	if _, err := iss.Verify(context.Background(), tok); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("old tv accepted: %v", err)
	}

	// Events tokens: own key + audience; never interchangeable with access tokens.
	versions[uid] = 3
	ev, _, err := iss.NewEventsToken(p, time.Hour) // ttl is capped at 5m
	if err != nil {
		t.Fatal(err)
	}
	if got, err := iss.ParseEventsToken(context.Background(), ev); err != nil || got.UserID != uid {
		t.Fatalf("events token: %+v %v", got, err)
	}
	if _, err := iss.Parse(ev); err == nil {
		t.Fatal("events token accepted as access token")
	}
	if _, err := iss.ParseEventsToken(context.Background(), tok); err == nil {
		t.Fatal("access token accepted as events token")
	}
	later := NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Minute)
	later.now = func() time.Time { return time.Now().Add(6 * time.Minute) }
	if _, err := later.ParseEventsToken(context.Background(), ev); err == nil {
		t.Fatal("events token outlived 5 minutes")
	}
}

func TestVersionCacheInvalidate(t *testing.T) {
	uid := uuid.New()
	v, loads := 1, 0
	c := NewVersionCache(func(context.Context, uuid.UUID) (int, error) { loads++; return v, nil }, time.Hour)
	if got, _ := c.TokenVersion(context.Background(), uid); got != 1 {
		t.Fatal(got)
	}
	v = 2
	if got, _ := c.TokenVersion(context.Background(), uid); got != 1 || loads != 1 {
		t.Fatalf("not cached: %d loads=%d", got, loads)
	}
	c.Invalidate(uid)
	if got, _ := c.TokenVersion(context.Background(), uid); got != 2 {
		t.Fatalf("invalidate: %d", got)
	}
}

func TestRefreshCookieAndMode(t *testing.T) {
	opts := CookieOptions{Secure: true, MaxAge: 720 * time.Hour}
	var mode bool
	var fromCookie string
	h := HTTPMiddleware(opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode, fromCookie = CookieMode(r.Context()), RefreshFromCookie(r.Context())
		if r.URL.Query().Get("clear") != "" {
			ClearRefreshCookie(r.Context())
			return
		}
		SetRefreshCookie(r.Context(), "tok.en")
	}))
	req := httptest.NewRequest("POST", "/graphql", nil)
	req.Header.Set(AuthModeHeader, "cookie")
	req.AddCookie(&http.Cookie{Name: RefreshCookie, Value: "old"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	sc := rec.Header().Get("Set-Cookie")
	for _, want := range []string{"psaas_rt=tok.en", "Path=/graphql", "Max-Age=2592000", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(sc, want) {
			t.Fatalf("Set-Cookie %q lacks %q", sc, want)
		}
	}
	if !mode || fromCookie != "old" {
		t.Fatalf("mode=%v cookie=%q", mode, fromCookie)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/graphql?clear=1", nil))
	if sc := rec.Header().Get("Set-Cookie"); !strings.Contains(sc, "psaas_rt=;") || !strings.Contains(sc, "Max-Age=0") {
		t.Fatalf("clear: %q", sc)
	}
	if mode {
		t.Fatal("body mode reported as cookie mode")
	}
}

func TestCSRFGuard(t *testing.T) {
	h := CSRFGuard([]string{"http://localhost:3100"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	tests := []struct {
		name   string
		cookie bool
		xrw    string
		origin string
		want   int
	}{
		{"no cookie, no header (API client)", false, "", "", 204},
		{"cookie + XHR header", true, "XMLHttpRequest", "", 204},
		{"cookie + allowed origin", true, "", "http://localhost:3100", 204},
		{"cookie, no header", true, "", "", 403},
		{"cookie, foreign origin", true, "", "https://evil.example", 403},
		{"cookie, null origin", true, "", "null", 403},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("POST", "/graphql", nil)
		if tt.cookie {
			req.AddCookie(&http.Cookie{Name: RefreshCookie, Value: "x"})
		}
		if tt.xrw != "" {
			req.Header.Set(CSRFHeader, tt.xrw)
		}
		if tt.origin != "" {
			req.Header.Set("Origin", tt.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tt.want {
			t.Errorf("%s: status %d want %d", tt.name, rec.Code, tt.want)
		}
		if tt.want == 403 && !strings.Contains(rec.Body.String(), "CSRF_REJECTED") {
			t.Errorf("%s: body %s", tt.name, rec.Body.String())
		}
	}
}
