package social

import (
	"context"
	"strings"
	"unicode/utf8"
)

// WhatsAppTemplate is an approved WhatsApp message template plus its body parameters
// ({{1}}, {{2}}, ... in order). WhatsApp only allows business-initiated messages outside
// the 24h customer-service window as templates (Sprint 2 #8).
type WhatsAppTemplate struct {
	Name     string   `json:"name"`
	Language string   `json:"language"`
	Params   []string `json:"params,omitempty"`
}

type waTemplateKey struct{}

// WithWhatsAppTemplate makes the WhatsApp sender send tpl instead of free-form text.
func WithWhatsAppTemplate(ctx context.Context, tpl WhatsAppTemplate) context.Context {
	return context.WithValue(ctx, waTemplateKey{}, tpl)
}

func whatsAppTemplateFrom(ctx context.Context) (WhatsAppTemplate, bool) {
	t, ok := ctx.Value(waTemplateKey{}).(WhatsAppTemplate)
	return t, ok && t.Name != ""
}

// MaxTemplateParam is WhatsApp's limit for one body parameter.
const MaxTemplateParam = 1024

// TemplateParam makes text acceptable as a template body parameter: WhatsApp rejects
// newlines, tabs and more than four consecutive spaces in parameters.
func TemplateParam(s string) string {
	lines := strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' })
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	out := strings.Join(nonEmpty(lines), " · ")
	if utf8.RuneCountInString(out) > MaxTemplateParam {
		out = string([]rune(out)[:MaxTemplateParam-1]) + "…"
	}
	return out
}

func nonEmpty(v []string) []string {
	out := v[:0]
	for _, s := range v {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// whatsAppTemplateBody is the Cloud API request for a template message.
func whatsAppTemplateBody(to string, tpl WhatsAppTemplate) map[string]any {
	lang := tpl.Language
	if lang == "" {
		lang = "en"
	}
	t := map[string]any{"name": tpl.Name, "language": map[string]any{"code": lang}}
	if len(tpl.Params) > 0 {
		params := make([]map[string]any, len(tpl.Params))
		for i, p := range tpl.Params {
			params[i] = map[string]any{"type": "text", "text": TemplateParam(p)}
		}
		t["components"] = []map[string]any{{"type": "body", "parameters": params}}
	}
	return map[string]any{"messaging_product": "whatsapp", "to": to, "type": "template", "template": t}
}
