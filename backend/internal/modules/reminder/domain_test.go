package reminder

import (
	"testing"
	"time"
)

func TestSendAtAndShift(t *testing.T) {
	dhaka := time.FixedZone("Asia/Dhaka", 6*3600)
	// Expires 2026-11-01 01:30 Dhaka (= 2026-10-31 19:30 UTC): the local day is Nov 1.
	exp := time.Date(2026, 10, 31, 19, 30, 0, 0, time.UTC)
	tests := []struct {
		trigger string
		offset  int
		want    time.Time
	}{
		{BeforeExpiry, 3, time.Date(2026, 10, 29, 10, 0, 0, 0, dhaka)},
		{OnExpiry, 0, time.Date(2026, 11, 1, 10, 0, 0, 0, dhaka)},
		{AfterExpiry, 2, time.Date(2026, 11, 3, 10, 0, 0, 0, dhaka)},
		{InvoiceOverdue, 1, time.Date(2026, 11, 2, 10, 0, 0, 0, dhaka)},
		{MasterExpiry, 7, time.Date(2026, 10, 25, 10, 0, 0, 0, dhaka)},
	}
	for _, tt := range tests {
		got := SendAt(exp, DayShift(tt.trigger, tt.offset), 10, dhaka)
		if !got.Equal(tt.want) {
			t.Errorf("%s/%d: got %v want %v", tt.trigger, tt.offset, got, tt.want)
		}
	}
}

func TestRuleInputValidate(t *testing.T) {
	ok := RuleInput{Name: "3 days", Trigger: "BEFORE_EXPIRY", OffsetDays: 3, SendHour: 10, Channel: ""}
	if err := ok.Validate(); err != nil || ok.Trigger != BeforeExpiry || ok.Channel != ChannelPreferred {
		t.Fatalf("%+v %v", ok, err)
	}
	on := RuleInput{Name: "on", Trigger: OnExpiry, OffsetDays: 5, SendHour: 9}
	if err := on.Validate(); err != nil || on.OffsetDays != 0 {
		t.Fatalf("on_expiry offset must be forced to 0: %+v", on)
	}
	for _, bad := range []RuleInput{
		{Name: "", Trigger: OnExpiry}, {Name: "x", Trigger: "weekly"}, {Name: "x", Trigger: OnExpiry, SendHour: 24},
		{Name: "x", Trigger: BeforeExpiry, OffsetDays: -1}, {Name: "x", Trigger: OnExpiry, Channel: "pigeon"},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}
