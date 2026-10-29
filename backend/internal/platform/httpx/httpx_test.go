package httpx

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
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
