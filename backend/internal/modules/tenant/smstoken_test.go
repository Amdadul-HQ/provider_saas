package tenant

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSMSToken(t *testing.T) {
	tid := uuid.New()
	tok, err := NewSMSToken(tid)
	if err != nil || !strings.HasPrefix(tok, "sms_") {
		t.Fatal(tok, err)
	}
	got, ok := ParseSMSToken(tok)
	if !ok || got != tid {
		t.Fatalf("parse = %v %v", got, ok)
	}
	for _, bad := range []string{"", "sms_", "sms_zz_abc", "Bearer x", "sms_" + strings.Repeat("0", 31) + "_x"} {
		if _, ok := ParseSMSToken(bad); ok {
			t.Errorf("parsed %q", bad)
		}
	}
	if h := SMSTokenHint(tok); !strings.HasPrefix(h, "sms_…") || strings.Contains(h, tok[4:36]) {
		t.Fatalf("hint = %q", h)
	}
	in := map[string]any{SettingSMSSecretID: "x", SettingSMSTokenHint: "sms_…abcd", "x": 1}
	out := PublicSettings(in)
	if _, leaked := out[SettingSMSSecretID]; leaked || out[SettingSMSTokenHint] != "sms_…abcd" || len(PublicSettings(nil)) != 0 {
		t.Fatalf("public settings = %v", out)
	}
}
