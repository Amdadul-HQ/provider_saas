//go:build integration

package lockout

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Runs against REDIS_URL / TEST_REDIS_URL (default redis://localhost:6379/0).
func TestRedisLimiter(t *testing.T) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379/0"
	}
	ctx := context.Background()
	r, err := NewRedis(ctx, url, Policy{MaxFailures: 3, Window: time.Minute, LockFor: 2 * time.Second})
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	defer r.Close()
	key := "it-" + uuid.NewString()
	for i := 1; i <= 2; i++ {
		if locked, err := r.Fail(ctx, key); err != nil || locked {
			t.Fatalf("fail %d: locked=%v err=%v", i, locked, err)
		}
	}
	if locked, _ := r.Fail(ctx, key); !locked {
		t.Fatal("3rd failure must lock")
	}
	if l, d, _ := r.Locked(ctx, key); !l || d <= 0 {
		t.Fatalf("locked=%v d=%v", l, d)
	}
	_ = r.Reset(ctx, key)
	if l, _, _ := r.Locked(ctx, key); l {
		t.Fatal("reset must unlock")
	}
}
