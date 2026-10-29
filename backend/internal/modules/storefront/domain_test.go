package storefront

import (
	"encoding/json"
	"testing"
)

func sp(s string) *string { return &s }

func TestInputValidate(t *testing.T) {
	in := Input{Slug: sp(" Demo-Store "), PaymentNumbers: []any{map[string]any{"method": "bkash", "number": "01700000000", "type": "personal"}}}
	pn, err := in.Validate()
	if err != nil || *in.Slug != "demo-store" {
		t.Fatalf("%v %v", in.Slug, err)
	}
	var back []map[string]string
	if json.Unmarshal(pn, &back) != nil || back[0]["number"] != "01700000000" {
		t.Fatalf("payment numbers = %s", pn)
	}
	for _, bad := range []Input{
		{Slug: sp("a")}, {Slug: sp("has space")}, {Slug: sp("admin")}, {Slug: sp("-dash")},
		{DisplayName: sp("  ")},
		{PaymentNumbers: []any{"01700000000"}}, {PaymentNumbers: []any{map[string]any{"method": "bkash"}}}, {PaymentNumbers: 42.0},
	} {
		if _, err := bad.Validate(); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if pn, err := (&Input{}).Validate(); err != nil || pn != nil {
		t.Fatal("nil payment numbers must mean unchanged")
	}
}

func TestPortalToken(t *testing.T) {
	tok, hash, err := NewPortalToken()
	if err != nil || len(tok) < 40 || hash != HashToken(tok) || hash == tok {
		t.Fatalf("token=%q hash=%q err=%v", tok, hash, err)
	}
	tok2, _, _ := NewPortalToken()
	if tok2 == tok {
		t.Fatal("tokens must be random")
	}
	sf := Storefront{PaymentNumbers: []byte(`[{"method":"nagad","number":"01800000000"}]`)}
	if v, ok := sf.PaymentNumbersValue().([]any); !ok || len(v) != 1 {
		t.Fatalf("payment numbers value = %#v", sf.PaymentNumbersValue())
	}
	if v, ok := (Storefront{}).PaymentNumbersValue().([]any); !ok || len(v) != 0 {
		t.Fatal("empty payment numbers must be []")
	}
}
