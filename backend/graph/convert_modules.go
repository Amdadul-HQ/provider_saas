package graph

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/graph/model"
	"provider_saas/internal/modules/content"
	"provider_saas/internal/modules/inbox"
	"provider_saas/internal/modules/reminder"
	"provider_saas/internal/modules/social"
	"provider_saas/internal/modules/storefront"
	"provider_saas/internal/modules/ticket"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/pagination"
)

// attachmentsOf decodes a JSON attachments/media argument (array or single object).
func attachmentsOf(v any, field string) ([]social.Attachment, error) {
	if v == nil {
		return nil, nil
	}
	if m, ok := v.(map[string]any); ok {
		v = []any{m}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, apperr.ValidationField(field, "invalid JSON")
	}
	var out []social.Attachment
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, apperr.ValidationField(field, "must be a list of {type, url}")
	}
	return out, nil
}

func attachmentsJSON(a []social.Attachment) any {
	if a == nil {
		return []social.Attachment{}
	}
	return a
}

func toSocialAccount(a social.Account) *model.SocialAccount {
	scopes := a.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return &model.SocialAccount{ID: a.ID, Platform: upper[model.SocialPlatform](a.Platform), ExternalID: a.ExternalID, Name: a.Name,
		Username: a.Username, AvatarURL: a.AvatarURL, TokenExpiresAt: a.TokenExpiresAt, Scopes: scopes, WebhookVerified: a.WebhookVerified,
		Status: upper[model.SocialAccountStatus](a.Status), LastError: a.LastError, LastSyncedAt: a.LastSyncedAt, CreatedAt: a.CreatedAt}
}

func toSocialAccounts(as []social.Account) []model.SocialAccount {
	out := make([]model.SocialAccount, len(as))
	for i, a := range as {
		out[i] = *toSocialAccount(a)
	}
	return out
}

func toConversation(c inbox.Conversation) *model.Conversation {
	labels := c.Labels
	if labels == nil {
		labels = []string{}
	}
	return &model.Conversation{ID: c.ID, SocialAccountID: c.SocialAccountID, CustomerID: c.CustomerID, IdentityID: c.IdentityID,
		AssignedToID: c.AssignedTo, Platform: upper[model.SocialPlatform](c.Platform), ExternalThreadID: c.ExternalThreadID,
		Status: upper[model.ConversationStatus](c.Status), UnreadCount: c.UnreadCount, LastMessageAt: c.LastMessageAt,
		LastMessagePreview: c.LastMessagePreview, LastInboundAt: c.LastInboundAt, CanReply: c.CanReply(time.Now()), Labels: labels,
		CreatedAt: c.CreatedAt}
}

func conversationCursor(c inbox.Conversation) string { return pagination.Encode(c.ActivityAt(), c.ID) }

func toMessage(m inbox.Message) *model.Message {
	return &model.Message{ID: m.ID, ConversationID: m.ConversationID, Direction: upper[model.MessageDirection](m.Direction),
		ExternalID: m.ExternalID, Kind: upper[model.MessageKind](m.Kind), Body: m.Body, Attachments: attachmentsJSON(m.Attachments),
		SentByID: m.SentBy, Status: m.Status, Error: m.Error, SentAt: m.SentAt}
}

func toTemplate(t inbox.Template) *model.MessageTemplate {
	return &model.MessageTemplate{ID: t.ID, Key: t.Key, Name: t.Name, Body: t.Body, Category: upper[model.TemplateCategory](t.Category), IsActive: t.IsActive}
}

func toPost(p content.Post) *model.Post {
	return &model.Post{ID: p.ID, Body: p.Body, Media: attachmentsJSON(p.Media), ScheduledAt: p.ScheduledAt,
		Status: upper[model.PostStatus](p.Status), CreatedByID: p.CreatedBy, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func toPostInput(in model.PostInput) (content.PostInput, error) {
	media, err := attachmentsOf(in.Media, "media")
	return content.PostInput{Body: in.Body, Media: media, ScheduledAt: in.ScheduledAt, SocialAccountIDs: in.SocialAccountIds}, err
}

func toTicket(t ticket.Ticket) *model.Ticket {
	return &model.Ticket{ID: t.ID, Number: t.Number, CustomerID: t.CustomerID, SubscriptionID: t.SubscriptionID, ConversationID: t.ConversationID,
		Kind: upper[model.TicketKind](t.Kind), Priority: upper[model.TicketPriority](t.Priority), Status: upper[model.TicketStatus](t.Status),
		Title: t.Title, Description: t.Description, Reason: t.Reason, OldSlotID: t.OldSlotID, NewSlotID: t.NewSlotID,
		AssignedToID: t.AssignedTo, ResolvedAt: t.ResolvedAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func toComment(c ticket.Comment) *model.TicketComment {
	return &model.TicketComment{ID: c.ID, AuthorID: c.AuthorID, Body: c.Body, IsInternal: c.IsInternal, CreatedAt: c.CreatedAt}
}

func toRule(r reminder.Rule) *model.ReminderRule {
	return &model.ReminderRule{ID: r.ID, Name: r.Name, Trigger: upper[model.ReminderTrigger](r.Trigger), OffsetDays: r.OffsetDays,
		SendHour: r.SendHour, Channel: upper[model.ReminderChannel](r.Channel), TemplateID: r.TemplateID, ProductID: r.ProductID,
		IsActive: r.IsActive, CreatedAt: r.CreatedAt}
}

func toReminder(r reminder.Reminder) *model.Reminder {
	return &model.Reminder{ID: r.ID, RuleID: r.RuleID, CustomerID: r.CustomerID, SubscriptionID: r.SubscriptionID, InvoiceID: r.InvoiceID,
		MasterAccountID: r.MasterAccountID, Channel: strings.ToUpper(r.Channel), ScheduledAt: r.ScheduledAt, SentAt: r.SentAt,
		Status: upper[model.ReminderStatus](r.Status), RenderedBody: r.RenderedBody, Error: r.Error}
}

func toOrder(o storefront.Order) *model.Order {
	return &model.Order{ID: o.ID, Number: o.Number, CustomerID: o.CustomerID, ProductID: o.ProductID, PlanID: o.PlanID, PaymentID: o.PaymentID,
		Quantity: o.Quantity, Amount: o.Amount, Currency: o.Currency, Source: upper[model.OrderSource](o.Source),
		Status: upper[model.OrderStatus](o.Status), SubscriptionID: o.SubscriptionID, CustomerName: o.CustomerName,
		CustomerPhone: o.CustomerPhone, CustomerNote: o.CustomerNote, CreatedAt: o.CreatedAt}
}

func (r *Resolver) toStorefront(s storefront.Storefront) *model.Storefront {
	return &model.Storefront{ID: s.ID, Slug: s.Slug, DisplayName: s.DisplayName, Tagline: s.Tagline, LogoURL: s.LogoURL, CoverURL: s.CoverURL,
		Theme: s.Theme, Whatsapp: s.Whatsapp, MessengerURL: s.MessengerURL, Telegram: s.Telegram, PaymentInstructions: s.PaymentInstructions,
		PaymentNumbers: s.PaymentNumbersValue(), IsPublished: s.IsPublished, PublicURL: r.StorefrontSvc.PublicURL(s.Slug)}
}

func toPublicStorefront(p storefront.PublicStorefront) *model.PublicStorefront {
	out := &model.PublicStorefront{Slug: p.Slug, DisplayName: p.DisplayName, Tagline: p.Tagline, LogoURL: p.LogoURL, CoverURL: p.CoverURL,
		Theme: p.Theme, Whatsapp: p.Whatsapp, MessengerURL: p.MessengerURL, Telegram: p.Telegram, PaymentInstructions: p.PaymentInstructions,
		PaymentNumbers: p.PaymentNumbersValue(), Products: make([]model.PublicProduct, len(p.Products))}
	for i, pr := range p.Products {
		pp := model.PublicProduct{ID: pr.ID, Name: pr.Name, Category: pr.Category, Description: pr.Description, ImageURL: pr.ImageURL,
			InStock: pr.InStock, Plans: make([]model.PublicPlan, len(pr.Plans))}
		for j, pl := range pr.Plans {
			pp.Plans[j] = model.PublicPlan{ID: pl.ID, Name: pl.Name, DurationDays: pl.DurationDays, Price: pl.Price, Currency: pl.Currency}
		}
		out.Products[i] = pp
	}
	return out
}

func toPublicOrder(o storefront.PublicOrder) *model.PublicOrder {
	return &model.PublicOrder{Number: o.Number, Status: upper[model.OrderStatus](o.Status), Amount: o.Amount, Currency: o.Currency,
		ProductName: o.ProductName, PaymentInstructions: o.PaymentInstructions, PaymentNumbers: o.PaymentNumbers}
}

func toPortal(v storefront.PortalView) *model.PortalView {
	out := &model.PortalView{Customer: model.PortalCustomer{Name: v.CustomerName, Phone: v.CustomerPhone, DueBalance: v.DueBalance},
		Subscriptions: make([]model.PortalSubscription, len(v.Subscriptions)), Invoices: make([]model.PortalInvoice, len(v.Invoices)),
		Storefront: *toPublicStorefront(v.Storefront)}
	for i, s := range v.Subscriptions {
		out.Subscriptions[i] = model.PortalSubscription{ID: s.ID, ProductName: s.ProductName, PlanName: s.PlanName, StartsAt: s.StartsAt,
			ExpiresAt: s.ExpiresAt, DaysLeft: s.DaysLeft, Status: upper[model.SubscriptionStatus](s.Status), DeliveryNote: s.DeliveryNote}
	}
	for i, inv := range v.Invoices {
		out.Invoices[i] = model.PortalInvoice{ID: inv.ID, Number: inv.Number, Total: inv.Total, PaidAmount: inv.PaidAmount,
			Status: upper[model.InvoiceStatus](inv.Status), IssuedAt: inv.IssuedAt, DueAt: inv.DueAt}
	}
	return out
}

// socialAccountByID resolves a social account through the loader.
func (r *Resolver) socialAccountByID(ctx context.Context, id uuid.UUID) (*model.SocialAccount, error) {
	a, err := r.loaders(ctx).SocialAccount.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, apperr.NotFound("social account")
	}
	return toSocialAccount(*a), nil
}

func (r *Resolver) slotByID(ctx context.Context, id *uuid.UUID) (*model.Slot, error) {
	if id == nil {
		return nil, nil
	}
	s, err := r.loaders(ctx).Slot.Load(ctx, *id)
	if err != nil || s == nil {
		return nil, err
	}
	return toSlot(ctx, *s), nil
}

func (r *Resolver) subscriptionByID(ctx context.Context, id *uuid.UUID) (*model.Subscription, error) {
	if id == nil {
		return nil, nil
	}
	s, err := r.loaders(ctx).Subscription.Load(ctx, *id)
	if err != nil || s == nil {
		return nil, err
	}
	return toSubscription(*s), nil
}

// nilIfNotFound turns NotFound into a null result for optional single-object queries.
func nilIfNotFound[T any](v *T, err error) (*T, error) {
	if errors.Is(err, apperr.ErrNotFound) {
		return nil, nil
	}
	return v, err
}

// convUpdate runs a tenant-scoped conversation mutation and maps the result.
func (r *Resolver) convUpdate(ctx context.Context, fn func(tid uuid.UUID) (inbox.Conversation, error)) (*model.Conversation, error) {
	tid, _, err := tenantOf(ctx)
	if err != nil {
		return nil, err
	}
	c, err := fn(tid)
	if err != nil {
		return nil, err
	}
	return toConversation(c), nil
}
