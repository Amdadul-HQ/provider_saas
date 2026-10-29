package delivery

import (
	"strings"
	"testing"
)

func TestRenderNeverStoresPassword(t *testing.T) {
	tpl := "প্রিয় {{customer}}, আপনার {{product}} প্রস্তুত ✅ মেয়াদ: {{expiry}}\n{{credentials}}"
	r := Render(tpl, Vars{Customer: "Rahim", Product: "Netflix Premium", Expiry: "২৯ নভেম্বর ২০২৬", SlotLabel: "Profile 3",
		Pin: "4321", Username: "nf.dhaka01@gmail.com", Password: "S3cret-Pass", LoginHint: "nf***@gmail.com"})
	for _, want := range []string{"Rahim", "Netflix Premium", "Email/ID: nf.dhaka01@gmail.com", "Password: S3cret-Pass", "Profile: Profile 3", "PIN: 4321"} {
		if !strings.Contains(r.Body, want) {
			t.Errorf("body misses %q:\n%s", want, r.Body)
		}
	}
	if strings.Contains(r.Note, "S3cret") || strings.Contains(r.Note, "4321") || strings.Contains(r.Note, "nf.dhaka01@") {
		t.Fatalf("note leaks a secret: %q", r.Note)
	}
	if !strings.Contains(r.Note, "nf***@gmail.com") || !strings.Contains(r.Note, "Profile 3") {
		t.Fatalf("note = %q", r.Note)
	}
	// explicit placeholders
	if got := Render("{{email}} / {{password}} / {{slot}} / {{pin}}", Vars{Username: "u", Password: "p", SlotLabel: "s", Pin: "1"}).Body; got != "u / p / s / 1" {
		t.Fatalf("placeholders: %q", got)
	}
}

func TestRenderRedactedMasksPasswordAndPin(t *testing.T) {
	r := Render("{{credentials}} | {{password}} | {{pin}}", Vars{Username: "nf@x.com", Password: "S3cret-Pass", Pin: "4321", SlotLabel: "P3"})
	if strings.Contains(r.Redacted, "S3cret") || strings.Contains(r.Redacted, "4321") {
		t.Fatalf("redacted leaks: %q", r.Redacted)
	}
	if !strings.Contains(r.Redacted, "Password: "+Mask) || !strings.Contains(r.Redacted, "PIN: "+Mask) {
		t.Fatalf("redacted = %q", r.Redacted)
	}
	if !strings.Contains(r.Body, "S3cret-Pass") {
		t.Fatalf("body must carry the real password: %q", r.Body)
	}
}
