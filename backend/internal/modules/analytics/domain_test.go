package analytics

import "testing"

func TestRenewalRate(t *testing.T) {
	tests := []struct {
		renewed, ended int
		want           float64
	}{{0, 0, 0}, {1, 3, 0.333}, {2, 2, 1}, {5, 2, 1}, {1, 8, 0.125}}
	for _, tt := range tests {
		if got := RenewalRate(tt.renewed, tt.ended); got != tt.want {
			t.Errorf("RenewalRate(%d,%d) = %v want %v", tt.renewed, tt.ended, got, tt.want)
		}
	}
	for in, want := range map[int]int{0: 30, -5: 30, 7: 7, 1000: 366} {
		if got := ClampDays(in); got != want {
			t.Errorf("ClampDays(%d) = %d", in, got)
		}
	}
}
