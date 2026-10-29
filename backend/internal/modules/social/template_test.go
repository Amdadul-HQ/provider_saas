package social

import (
	"context"
	"strings"
	"testing"
)

func TestTemplateParamSanitises(t *testing.T) {
	got := TemplateParam("প্রিয় রহিম,\n\nআপনার Netflix\tমেয়াদ     শেষ\r\n")
	if got != "প্রিয় রহিম, · আপনার Netflix মেয়াদ শেষ" {
		t.Fatalf("got %q", got)
	}
	long := TemplateParam(strings.Repeat("ক", 2000))
	if n := len([]rune(long)); n != MaxTemplateParam {
		t.Fatalf("len = %d", n)
	}
}

func TestWhatsAppTemplateBody(t *testing.T) {
	b := whatsAppTemplateBody("8801711", WhatsAppTemplate{Name: "renewal_reminder", Language: "bn", Params: []string{"Rahim", "a\nb"}})
	if b["type"] != "template" || b["to"] != "8801711" {
		t.Fatalf("body = %v", b)
	}
	tpl := b["template"].(map[string]any)
	comps := tpl["components"].([]map[string]any)
	params := comps[0]["parameters"].([]map[string]any)
	if tpl["name"] != "renewal_reminder" || len(params) != 2 || params[1]["text"] != "a · b" {
		t.Fatalf("template = %v", tpl)
	}
}

func TestWhatsAppSenderUsesTemplateFromContext(t *testing.T) {
	fake := NewFake()
	s := WhatsAppSender{Client: fake, Tokens: staticTokens("EAAG-token")}
	ctx := WithWhatsAppTemplate(context.Background(), WhatsAppTemplate{Name: "delivery_update", Language: "en", Params: []string{"hello"}})
	if _, err := s.Send(ctx, Account{Platform: PlatformWhatsApp, ExternalID: "pn1"}, "8801700", "ignored body", nil); err != nil {
		t.Fatal(err)
	}
	calls := fake.CallsOf("SendWhatsAppTemplate")
	if len(calls) != 1 || calls[0].Args[1] != "delivery_update" || calls[0].Args[3] != "hello" || len(fake.CallsOf("SendWhatsApp")) != 0 {
		t.Fatalf("calls = %+v", fake.Calls)
	}
	// Without a template in ctx, free-form text is sent.
	if _, err := s.Send(context.Background(), Account{Platform: PlatformWhatsApp, ExternalID: "pn1"}, "8801700", "hi", nil); err != nil {
		t.Fatal(err)
	}
	if len(fake.CallsOf("SendWhatsApp")) != 1 {
		t.Fatalf("calls = %+v", fake.Calls)
	}
}

type staticTokens string

func (s staticTokens) Token(context.Context, Account) (string, error) { return string(s), nil }
