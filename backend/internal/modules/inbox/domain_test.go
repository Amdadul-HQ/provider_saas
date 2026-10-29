package inbox

import "testing"

func TestRender(t *testing.T) {
	tests := []struct{ tpl, want string }{
		{"প্রিয় {{customer}}, {{product}} শেষ {{expiry}}", "প্রিয় Rahim, Netflix শেষ ২৯ অক্টোবর"},
		{"{{ customer }} owes {{amount}}{{link}}", "Rahim owes "}, // known but missing -> empty
		{"Hi {{nickname}}", "Hi {{nickname}}"},                    // unknown stays visible
		{"{{CUSTOMER}}", "Rahim"},                                 // case-insensitive
		{"{{credentials}}", "{{credentials}}"},                    // not a message var unless provided
	}
	vars := map[string]string{"customer": "Rahim", "product": "Netflix", "expiry": "২৯ অক্টোবর"}
	for _, tt := range tests {
		if got := Render(tt.tpl, vars); got != tt.want {
			t.Errorf("Render(%q) = %q, want %q", tt.tpl, got, tt.want)
		}
	}
}

func TestPreviewLabelsAndNames(t *testing.T) {
	long := "a b c " + string(make([]byte, 0))
	for i := 0; i < 50; i++ {
		long += "word "
	}
	p := Preview("text", &long)
	if len([]rune(p)) != 120 {
		t.Fatalf("preview len = %d", len([]rune(p)))
	}
	if Preview("image", nil) != "📷 Photo" {
		t.Fatal("image preview")
	}
	if l, err := NormalizeLabel("  VIP "); err != nil || l != "vip" {
		t.Fatalf("label = %q %v", l, err)
	}
	if _, err := NormalizeLabel(""); err == nil {
		t.Fatal("empty label accepted")
	}
	if DefaultName("telegram", "111222333") != "Telegram user 2333" {
		t.Fatal(DefaultName("telegram", "111222333"))
	}
	in := TemplateInput{Key: " Quick.Price ", Name: "Price", Body: "x", Category: "QUICK_REPLY"}
	if err := in.Validate(); err != nil || in.Key != "quick.price" || in.Category != "quick_reply" {
		t.Fatalf("template validate: %+v %v", in, err)
	}
	if err := (&TemplateInput{Key: "k", Name: "n", Body: "b", Category: "bogus"}).Validate(); err == nil {
		t.Fatal("bad category accepted")
	}
}
