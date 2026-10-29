package money

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		amt  int64
		cur  string
		sym  bool
		want string
	}{
		{35000, "BDT", false, "350.00 BDT"},
		{125000, "BDT", true, "৳1,250.00"},
		{123456789, "USD", false, "1,234,567.89 USD"},
		{-505, "", true, "৳-5.05"},
		{0, "BDT", false, "0.00 BDT"},
	}
	for _, tt := range tests {
		if got := Format(tt.amt, tt.cur, tt.sym); got != tt.want {
			t.Errorf("Format(%d) = %q want %q", tt.amt, got, tt.want)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"350.00", 35000, false}, {"1,250.00", 125000, false}, {"350", 35000, false},
		{"350.5", 35050, false}, {"", 0, true}, {"abc", 0, true}, {"1.234", 0, true}, {"-5", 0, true},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("Parse(%q) = %d,%v", tt.in, got, err)
		}
	}
}
