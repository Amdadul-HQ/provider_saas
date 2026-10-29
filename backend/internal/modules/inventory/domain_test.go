package inventory

import (
	"testing"
	"time"
)

func TestMaskLogin(t *testing.T) {
	tests := []struct{ in, want string }{
		{"rahim.netflix@gmail.com", "ra***@gmail.com"},
		{"a@x.com", "a***@x.com"},
		{"01712345678", "01***78"},
		{"abc", "ab***"},
	}
	for _, tt := range tests {
		if got := MaskLogin(tt.in); got != tt.want {
			t.Errorf("MaskLogin(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestExpiryStatus(t *testing.T) {
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	tests := []struct {
		cur  string
		exp  *time.Time
		want string
	}{
		{MasterActive, nil, MasterActive},
		{MasterActive, at(30 * 24 * time.Hour), MasterActive},
		{MasterActive, at(3 * 24 * time.Hour), MasterExpiring},
		{MasterExpiring, at(-time.Hour), MasterExpired},
		{MasterExpired, at(40 * 24 * time.Hour), MasterActive}, // renewed upstream
		{MasterBanned, at(-time.Hour), MasterBanned},
	}
	for _, tt := range tests {
		if got := ExpiryStatus(tt.cur, tt.exp, now); got != tt.want {
			t.Errorf("ExpiryStatus(%s,%v) = %s want %s", tt.cur, tt.exp, got, tt.want)
		}
	}
}

func TestValidateSlotStatusChange(t *testing.T) {
	tests := []struct {
		from, to string
		ok       bool
	}{
		{SlotFree, SlotBanned, true}, {SlotFree, SlotAssigned, false}, {SlotAssigned, SlotFree, false},
		{SlotAssigned, SlotBanned, true}, {SlotBanned, SlotFree, true}, {SlotFree, "lost", false}, {SlotAssigned, SlotAssigned, true},
	}
	for _, tt := range tests {
		if err := ValidateSlotStatusChange(tt.from, tt.to); (err == nil) != tt.ok {
			t.Errorf("%s->%s: %v", tt.from, tt.to, err)
		}
	}
}

func TestCreateMasterValidate(t *testing.T) {
	u, p := "rahim@gmail.com", "secret"
	in := CreateMasterInput{Label: "NF-01", TotalSlots: 5, Username: &u, Password: &p}
	if err := in.Validate(); err != nil || *in.LoginHint != "ra***@gmail.com" {
		t.Fatalf("%v %v", err, in.LoginHint)
	}
	bad := []CreateMasterInput{
		{Label: "", TotalSlots: 1}, {Label: "x", TotalSlots: 0}, {Label: "x", TotalSlots: 51},
		{Label: "x", TotalSlots: 1, PurchaseCost: -1}, {Label: "x", TotalSlots: 1, Username: &u},
	}
	for i, b := range bad {
		if err := b.Validate(); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}
