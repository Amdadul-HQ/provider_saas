package subscription

import (
	"testing"
	"time"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{StatusActive, StatusExpiring, true}, {StatusExpiring, StatusExpired, true}, {StatusExpired, StatusActive, true},
		{StatusCancelled, StatusActive, false}, {StatusReplaced, StatusActive, false}, {StatusExpired, StatusCancelled, false},
		{StatusPending, StatusActive, true}, {StatusActive, StatusPending, false}, {StatusActive, StatusActive, true},
	}
	for _, tt := range tests {
		if got := CanTransition(tt.from, tt.to); got != tt.want {
			t.Errorf("%s->%s = %v", tt.from, tt.to, got)
		}
	}
}

func TestStatusAt(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	tests := []struct {
		name          string
		start, expire time.Duration
		want          string
	}{
		{"active", -day, 20 * day, StatusActive},
		{"expiring 2d", -day, 2 * day, StatusExpiring},
		{"expiring exactly 3d", -day, 3 * day, StatusExpiring},
		{"expired", -30 * day, -time.Second, StatusExpired},
		{"expires now", -30 * day, 0, StatusExpired},
		{"future start", day, 31 * day, StatusPending},
	}
	for _, tt := range tests {
		if got := StatusAt(now.Add(tt.start), now.Add(tt.expire), now); got != tt.want {
			t.Errorf("%s: got %s want %s", tt.name, got, tt.want)
		}
	}
}

func TestDaysLeftAndExpiry(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		exp  time.Time
		want int
	}{
		{now.Add(36 * time.Hour), 2}, {now.Add(24 * time.Hour), 1}, {now, 0}, {now.Add(-36 * time.Hour), -2},
	}
	for _, tt := range tests {
		if got := DaysLeft(tt.exp, now); got != tt.want {
			t.Errorf("DaysLeft(%v) = %d want %d", tt.exp, got, tt.want)
		}
	}
	if got := ExpiryFor(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), 30); !got.Equal(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("ExpiryFor = %v", got)
	}
}

func TestResolveTerms(t *testing.T) {
	i := func(v int) *int { return &v }
	m := func(v int64) *int64 { return &v }
	tests := []struct {
		name                string
		pd                  *int
		pp, pc              *int64
		dd                  *int
		price, cost         *int64
		wantDur             int
		wantPrice, wantCost int64
		wantErr             bool
	}{
		{"plan defaults", i(30), m(35000), m(20000), nil, nil, nil, 30, 35000, 20000, false},
		{"overrides", i(30), m(35000), m(20000), i(60), m(65000), m(1), 60, 65000, 1, false},
		{"no plan no duration", nil, nil, nil, nil, m(100), nil, 0, 0, 0, true},
		{"no plan no price", nil, nil, nil, i(30), nil, nil, 0, 0, 0, true},
		{"no plan explicit", nil, nil, nil, i(30), m(100), nil, 30, 100, 0, false},
		{"bad duration", i(30), m(1), nil, i(0), nil, nil, 0, 0, 0, true},
		{"negative price", i(30), m(1), nil, nil, m(-5), nil, 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTerms(tt.pd, tt.pp, tt.pc, "", tt.dd, tt.price, tt.cost)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if err == nil && (got.DurationDays != tt.wantDur || got.Price != tt.wantPrice || got.Cost != tt.wantCost || got.Currency != "BDT") {
				t.Fatalf("got %+v", got)
			}
		})
	}
}
