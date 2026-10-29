package pagination

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	ts := time.Date(2026, 10, 29, 10, 15, 0, 123456000, time.UTC)
	id := uuid.New()
	c, err := Decode(Encode(ts, id))
	if err != nil || !c.CreatedAt.Equal(ts) || c.ID != id {
		t.Fatalf("got %+v %v", c, err)
	}
	for _, bad := range []string{"!!", "bm9waXBl", Encode(ts, id)[:5]} {
		if _, err := Decode(bad); err == nil {
			t.Errorf("Decode(%q) accepted", bad)
		}
	}
}

func TestNewPage(t *testing.T) {
	i := func(n int) *int { return &n }
	tests := []struct {
		first *int
		want  int
	}{{nil, 25}, {i(0), 25}, {i(10), 10}, {i(1000), 100}}
	for _, tt := range tests {
		p, err := New(tt.first, nil)
		if err != nil || p.First != tt.want {
			t.Errorf("New(%v) = %d", tt.first, p.First)
		}
	}
	rows, more := Trim([]int{1, 2, 3}, 2)
	if len(rows) != 2 || !more {
		t.Fatal("Trim")
	}
}
