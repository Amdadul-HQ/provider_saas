package bn

import (
	"testing"
	"time"
)

func TestDate(t *testing.T) {
	ts := time.Date(2026, 10, 28, 20, 0, 0, 0, time.UTC) // 02:00 on the 29th in Dhaka
	if got := Date(ts, Dhaka); got != "২৯ অক্টোবর ২০২৬" {
		t.Fatalf("Date = %q", got)
	}
	if got := Digits("Tk 350.00"); got != "Tk ৩৫০.০০" {
		t.Fatalf("Digits = %q", got)
	}
	if Location("Nowhere/Invalid") != Dhaka || Location("Asia/Dhaka") == nil {
		t.Fatal("Location fallback")
	}
}
