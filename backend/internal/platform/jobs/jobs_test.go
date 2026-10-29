package jobs

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	tests := []struct {
		attempt  int
		min, max time.Duration
	}{
		{0, 9 * time.Second, 11 * time.Second},
		{1, 9 * time.Second, 11 * time.Second},
		{2, 18 * time.Second, 22 * time.Second},
		{5, 144 * time.Second, 176 * time.Second},
		{30, 54 * time.Minute, 66 * time.Minute},
	}
	for _, tt := range tests {
		for i := 0; i < 50; i++ {
			if d := Backoff(tt.attempt); d < tt.min || d > tt.max {
				t.Fatalf("Backoff(%d) = %v not in [%v,%v]", tt.attempt, d, tt.min, tt.max)
			}
		}
	}
}
