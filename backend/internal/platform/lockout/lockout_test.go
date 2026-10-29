package lockout

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMemoryLocksAfterMaxFailuresAndExpires(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	m := NewMemory(DefaultLogin, 10)
	m.SetClock(func() time.Time { return now })
	for i := 1; i <= 4; i++ {
		if locked, _ := m.Fail(ctx, "u"); locked {
			t.Fatalf("locked after %d failures", i)
		}
	}
	if locked, _ := m.Fail(ctx, "u"); !locked {
		t.Fatal("5th failure must lock")
	}
	if l, d, _ := m.Locked(ctx, "u"); !l || d != 15*time.Minute {
		t.Fatalf("locked=%v d=%v", l, d)
	}
	if l, _, _ := m.Locked(ctx, "other"); l {
		t.Fatal("other key must not be locked")
	}
	now = now.Add(15*time.Minute + time.Second)
	if l, _, _ := m.Locked(ctx, "u"); l {
		t.Fatal("lock must expire")
	}
}

func TestMemoryWindowResetsAndResetClears(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	m := NewMemory(Policy{MaxFailures: 3, Window: time.Minute, LockFor: time.Minute}, 10)
	m.SetClock(func() time.Time { return now })
	m.Fail(ctx, "k")
	m.Fail(ctx, "k")
	now = now.Add(2 * time.Minute) // outside the window: count restarts
	if locked, _ := m.Fail(ctx, "k"); locked {
		t.Fatal("failures outside the window must not accumulate")
	}
	m.Fail(ctx, "k")
	_ = m.Reset(ctx, "k")
	if locked, _ := m.Fail(ctx, "k"); locked {
		t.Fatal("reset must clear failures")
	}
}

func TestMemoryIsBounded(t *testing.T) {
	ctx := context.Background()
	m := NewMemory(DefaultLogin, 100)
	for i := 0; i < 10_000; i++ {
		_, _ = m.Fail(ctx, fmt.Sprintf("ip-%d", i))
	}
	if m.Len() != 100 {
		t.Fatalf("len = %d, want 100", m.Len())
	}
}
