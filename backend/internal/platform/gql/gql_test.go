package gql

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/auth"
)

func TestRequireAuth(t *testing.T) {
	owner := auth.WithPrincipal(context.Background(), auth.Principal{UserID: uuid.New(), Role: "owner"})
	staff := auth.WithPrincipal(context.Background(), auth.Principal{UserID: uuid.New(), Role: "staff"})
	admin := auth.WithPrincipal(context.Background(), auth.Principal{UserID: uuid.New(), Role: "viewer", IsPlatformAdmin: true})
	tests := []struct {
		name  string
		ctx   context.Context
		roles []string
		want  error
	}{
		{"anonymous", context.Background(), nil, apperr.ErrUnauthenticated},
		{"any role", staff, nil, nil},
		{"owner ok", owner, []string{"OWNER", "ADMIN"}, nil},
		{"staff forbidden", staff, []string{"OWNER", "ADMIN"}, apperr.ErrForbidden},
		{"platform admin", admin, []string{"OWNER"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireAuth(tt.ctx, tt.roles)
			if !errors.Is(err, tt.want) && !(err == nil && tt.want == nil) {
				t.Fatalf("got %v want %v", err, tt.want)
			}
		})
	}
}

func TestErrorPresenter(t *testing.T) {
	p := ErrorPresenter(false)
	ctx := context.Background()
	if e := p(ctx, apperr.Conflict("no free slot")); e.Extensions["code"] != "CONFLICT" || e.Message != "no free slot" {
		t.Fatalf("conflict: %+v", e)
	}
	if e := p(ctx, errors.New("pq: secret detail")); e.Extensions["code"] != "INTERNAL_SERVER_ERROR" || e.Message != "internal server error" {
		t.Fatalf("internal: %+v", e)
	}
	if e := p(ctx, apperr.RateLimitedFor("slow down", 2500*time.Millisecond)); e.Extensions["code"] != "RATE_LIMITED" || e.Extensions["retryAfter"] != 3 {
		t.Fatalf("rate limited: %+v", e)
	}
	if e := p(ctx, apperr.RateLimited("slow down")); e.Extensions["retryAfter"] != 1 {
		t.Fatalf("rate limited default retryAfter: %+v", e)
	}
	for _, k := range apperr.Kinds { // L5: every kind keeps its message and gets a code
		if e := p(ctx, &apperr.Error{Kind: k, Msg: "m"}); e.Extensions["code"] == "" || e.Extensions["code"] == "INTERNAL_SERVER_ERROR" || e.Message != "m" {
			t.Fatalf("kind %v: %+v", k, e)
		}
	}
}

func TestLoaderBatches(t *testing.T) {
	var calls atomic.Int32
	l := NewLoader(func(ctx context.Context, keys []int) (map[int]string, error) {
		calls.Add(1)
		m := map[int]string{}
		for _, k := range keys {
			if k != 3 {
				m[k] = "v"
			}
		}
		return m, nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			v, err := l.Load(context.Background(), k)
			if err != nil || (k == 3) != (v == "") {
				t.Errorf("Load(%d) = %q, %v", k, v, err)
			}
		}(i)
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("fetch called %d times, want 1", calls.Load())
	}
}
