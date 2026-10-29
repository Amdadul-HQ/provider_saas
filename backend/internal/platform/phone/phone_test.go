package phone

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"01712345678", "01712345678", true},
		{"+8801712345678", "01712345678", true},
		{"8801712345678", "01712345678", true},
		{"017-1234 5678", "01712345678", true},
		{"০১৭১২৩৪৫৬৭৮", "01712345678", true},
		{"01212345678", "01212345678", false},
		{"+1 415 555", "+1415555", false},
	}
	for _, tt := range tests {
		got, ok := Normalize(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Normalize(%q) = %q,%v want %q,%v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}
