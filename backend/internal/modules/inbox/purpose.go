package inbox

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"provider_saas/internal/modules/social"
)

// System message purposes. Each maps to an approved WhatsApp template in
// tenant.settings.whatsapp_templates, used automatically outside the 24h window.
const (
	PurposeReminder = "reminder"
	PurposeDelivery = "delivery"
	PurposeCampaign = "campaign"
)

// SettingWhatsAppTemplates is the tenant settings key: {"reminder": {"name": "renewal_reminder",
// "language": "bn", "params": ["customer", "body"]}, "delivery": "delivery_update", ...}.
// A bare string is the template name. params are filled in order from: "body" (the
// rendered message), "customer" (customer name) or literal text; default ["body"].
const SettingWhatsAppTemplates = "whatsapp_templates"

type purposeKey struct{}

// WithPurpose tags system messages queued with ctx (reminder/delivery/campaign).
func WithPurpose(ctx context.Context, purpose string) context.Context {
	return context.WithValue(ctx, purposeKey{}, purpose)
}

// PurposeFrom returns the purpose set by WithPurpose ("" = none).
func PurposeFrom(ctx context.Context) string {
	p, _ := ctx.Value(purposeKey{}).(string)
	return p
}

// WATemplateSpec is the template choice stored in the inbox.send payload. Params are
// resolved when the job runs (so "body" can be the vault-sealed original text).
type WATemplateSpec struct {
	Name     string   `json:"name"`
	Language string   `json:"language"`
	Params   []string `json:"params"`
	Customer string   `json:"customer,omitempty"`
}

// Resolve fills the parameters.
func (w WATemplateSpec) Resolve(body string) social.WhatsAppTemplate {
	out := social.WhatsAppTemplate{Name: w.Name, Language: w.Language}
	for _, p := range w.Params {
		switch strings.ToLower(p) {
		case "body":
			out.Params = append(out.Params, body)
		case "customer":
			out.Params = append(out.Params, w.Customer)
		default:
			out.Params = append(out.Params, p)
		}
	}
	return out
}

// WhatsAppTemplateFor reads the configured template for purpose from tenant settings.
func WhatsAppTemplateFor(settings map[string]any, locale, purpose string) (WATemplateSpec, bool) {
	all, _ := settings[SettingWhatsAppTemplates].(map[string]any)
	if purpose == "" || all == nil {
		return WATemplateSpec{}, false
	}
	lang := "en"
	if strings.HasPrefix(strings.ToLower(locale), "bn") {
		lang = "bn"
	}
	spec := WATemplateSpec{Language: lang, Params: []string{"body"}}
	switch v := all[purpose].(type) {
	case string:
		spec.Name = strings.TrimSpace(v)
	case map[string]any:
		spec.Name, _ = v["name"].(string)
		spec.Name = strings.TrimSpace(spec.Name)
		if l, ok := v["language"].(string); ok && strings.TrimSpace(l) != "" {
			spec.Language = strings.TrimSpace(l)
		}
		if ps, ok := v["params"].([]any); ok {
			spec.Params = spec.Params[:0]
			for _, p := range ps {
				if s, ok := p.(string); ok {
					spec.Params = append(spec.Params, s)
				}
			}
		}
	}
	return spec, spec.Name != ""
}

// PurposeSender is the inbox seen by a module that only sends one kind of system
// message (reminder -> "reminder", delivery -> "delivery"); it tags ctx so WhatsApp
// conversations outside the 24h window get the matching approved template.
type PurposeSender struct {
	Inbox   Service
	Purpose string
}

func (p PurposeSender) SendSystem(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) (Message, error) {
	return p.Inbox.SendSystem(WithPurpose(ctx, p.Purpose), tenantID, customerID, channel, body)
}

func (p PurposeSender) SendSystemSensitive(ctx context.Context, tenantID, customerID uuid.UUID, channel, storedBody, secretBody string) (Message, error) {
	return p.Inbox.SendSystemSensitive(WithPurpose(ctx, p.Purpose), tenantID, customerID, channel, storedBody, secretBody)
}

func (p PurposeSender) SendToCustomer(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) error {
	return p.Inbox.SendToCustomer(WithPurpose(ctx, p.Purpose), tenantID, customerID, channel, body)
}
