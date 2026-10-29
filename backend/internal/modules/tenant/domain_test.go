package tenant

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Rahim's Netflix BD", "rahims-netflix-bd"},
		{"  Digital   Shop!! ", "digital-shop"},
		{"ডিজিটাল শপ", "shop"},
		{"AB", "shop-ab"},
		{"Premium Subscriptions & Accounts Store Of Dhaka City", "premium-subscriptions-accounts-store-of"},
	}
	for _, tt := range tests {
		if got := Slugify(tt.in); got != tt.want {
			t.Errorf("Slugify(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
	if !ValidRole("owner") || ValidRole("god") {
		t.Fatal("ValidRole")
	}
}

func TestMaskContact(t *testing.T) {
	e, p := "rahim@example.com", "01712345678"
	if got := *MaskEmail(&e); got != "ra***@example.com" {
		t.Fatalf("email %q", got)
	}
	if got := *MaskPhone(&p); got != "017*****678" {
		t.Fatalf("phone %q", got)
	}
	if MaskEmail(nil) != nil || MaskPhone(nil) != nil {
		t.Fatal("nil")
	}
}
