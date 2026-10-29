package graph

import (
	"context"
	"time"

	"github.com/google/uuid"

	"provider_saas/graph/model"
	"provider_saas/internal/modules/analytics"
	"provider_saas/internal/modules/auth"
	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/pagination"
)

func toUser(u auth.User) *model.User {
	return &model.User{ID: u.ID, Name: u.Name, Email: u.Email, Phone: u.Phone, AvatarURL: u.AvatarURL,
		IsPlatformAdmin: u.IsPlatformAdmin, CreatedAt: u.CreatedAt}
}

func toUserSummary(u tenant.UserSummary) model.User {
	return model.User{ID: u.ID, Name: u.Name, Email: u.Email, Phone: u.Phone, AvatarURL: u.AvatarURL,
		IsPlatformAdmin: u.IsPlatformAdmin, CreatedAt: u.CreatedAt}
}

func toTenant(t tenant.Tenant) *model.Tenant {
	settings := t.Settings
	if settings == nil {
		settings = map[string]any{}
	}
	return &model.Tenant{ID: t.ID, Name: t.Name, Slug: t.Slug, Plan: t.Plan, Status: t.Status, Currency: t.Currency,
		Timezone: t.Timezone, Locale: t.Locale, Settings: settings, TrialEndsAt: t.TrialEndsAt, CreatedAt: t.CreatedAt}
}

func toMembership(m tenant.Membership) model.Membership {
	return model.Membership{ID: m.ID, Tenant: *toTenant(m.Tenant), Role: upper[model.Role](m.Role), CommissionBps: m.CommissionBps, Status: m.Status}
}

func toMember(m tenant.Member) *model.TenantMember {
	return &model.TenantMember{ID: m.ID, User: toUserSummary(m.User), Role: upper[model.Role](m.Role),
		CommissionBps: m.CommissionBps, Status: m.Status, CreatedAt: m.CreatedAt}
}

func toPayload(s auth.Session) *model.AuthPayload {
	return &model.AuthPayload{AccessToken: s.AccessToken, RefreshToken: s.RefreshToken, User: *toUser(s.User)}
}

func toCustomer(c customer.Customer) *model.Customer {
	tags := c.Tags
	if tags == nil {
		tags = []string{}
	}
	return &model.Customer{ID: c.ID, Name: c.Name, Phone: c.Phone, Email: c.Email, Whatsapp: c.Whatsapp, Telegram: c.Telegram,
		Address: c.Address, Tags: tags, Notes: c.Notes, Source: upper[model.CustomerSource](c.Source),
		PreferredChannel: upperPtr[model.Platform](c.PreferredChannel), DueBalance: c.DueBalance, LifetimeValue: c.LifetimeValue,
		Status: upper[model.CustomerStatus](c.Status), LastContactAt: c.LastContactAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func customerCursor(c customer.Customer) string { return pagination.Encode(c.CreatedAt, c.ID) }

func toIdentity(i customer.Identity) model.CustomerIdentity {
	return model.CustomerIdentity{ID: i.ID, Platform: upper[model.Platform](i.Platform), ExternalID: i.ExternalID, Handle: i.Handle,
		AvatarURL: i.AvatarURL, SocialAccountID: i.SocialAccountID, CreatedAt: i.CreatedAt}
}

func toProduct(p catalog.Product) *model.Product {
	return &model.Product{ID: p.ID, Name: p.Name, Category: p.Category, Description: p.Description, ImageURL: p.ImageURL,
		DeliveryType: upper[model.DeliveryType](p.DeliveryType), WarrantyDays: p.WarrantyDays, DeliveryTemplate: p.DeliveryTemplate,
		IsActive: p.IsActive, SortOrder: p.SortOrder, CreatedAt: p.CreatedAt}
}

func toPlan(p catalog.Plan) *model.ProductPlan {
	return &model.ProductPlan{ID: p.ID, ProductID: p.ProductID, Name: p.Name, DurationDays: p.DurationDays, Price: p.Price,
		Cost: p.Cost, Currency: p.Currency, IsActive: p.IsActive, SortOrder: p.SortOrder}
}

func toSupplier(s inventory.Supplier) *model.Supplier {
	return &model.Supplier{ID: s.ID, Name: s.Name, Contact: s.Contact, Notes: s.Notes, Rating: s.Rating, CreatedAt: s.CreatedAt}
}

func toMaster(m inventory.MasterAccount) *model.MasterAccount {
	return &model.MasterAccount{ID: m.ID, ProductID: m.ProductID, SupplierID: m.SupplierID, Label: m.Label, LoginHint: m.LoginHint,
		HasCredential: m.CredentialSecretID != nil, TotalSlots: m.TotalSlots, PurchaseCost: m.PurchaseCost, Currency: m.Currency,
		PurchasedAt: m.PurchasedAt, ExpiresAt: m.ExpiresAt, Status: upper[model.MasterAccountStatus](m.Status), Notes: m.Notes, CreatedAt: m.CreatedAt}
}

func toSlot(s inventory.Slot) *model.Slot {
	return &model.Slot{ID: s.ID, MasterAccountID: s.MasterAccountID, SlotNo: s.SlotNo, Label: s.Label, Pin: s.Pin,
		Status: upper[model.SlotStatus](s.Status), SubscriptionID: s.SubscriptionID}
}

func toSubscription(s subscription.Subscription) *model.Subscription {
	return &model.Subscription{ID: s.ID, CustomerID: s.CustomerID, ProductID: s.ProductID, PlanID: s.PlanID, SlotID: s.SlotID,
		SoldByID: s.SoldBy, StartsAt: s.StartsAt, ExpiresAt: s.ExpiresAt, DaysLeft: subscription.DaysLeft(s.ExpiresAt, time.Now()),
		Price: s.Price, Cost: s.Cost, Currency: s.Currency, Status: upper[model.SubscriptionStatus](s.Status), AutoRenew: s.AutoRenew,
		DeliveredAt: s.DeliveredAt, DeliveryNote: s.DeliveryNote, RenewedFromID: s.RenewedFromID, Notes: s.Notes, CreatedAt: s.CreatedAt}
}

func toSubscriptions(ss []subscription.Subscription) []model.Subscription {
	out := make([]model.Subscription, len(ss))
	for i, s := range ss {
		out[i] = *toSubscription(s)
	}
	return out
}

func toInvoice(i billing.Invoice) *model.Invoice {
	return &model.Invoice{ID: i.ID, Number: i.Number, CustomerID: i.CustomerID, Status: upper[model.InvoiceStatus](i.Status),
		Subtotal: i.Subtotal, Discount: i.Discount, Total: i.Total, PaidAmount: i.PaidAmount, DueAmount: i.DueAmount(),
		Currency: i.Currency, IssuedAt: i.IssuedAt, DueAt: i.DueAt, PaidAt: i.PaidAt, Notes: i.Notes, CreatedAt: i.CreatedAt}
}

func toItem(i billing.InvoiceItem) model.InvoiceItem {
	return model.InvoiceItem{ID: i.ID, SubscriptionID: i.SubscriptionID, Description: i.Description, Quantity: i.Quantity,
		UnitPrice: i.UnitPrice, Total: i.Total}
}

func toPayment(p billing.Payment) *model.Payment {
	return &model.Payment{ID: p.ID, CustomerID: p.CustomerID, InvoiceID: p.InvoiceID, Method: upper[model.PaymentMethod](p.Method),
		TrxID: p.TrxID, SenderNumber: p.SenderNumber, Amount: p.Amount, Currency: p.Currency, Status: upper[model.PaymentStatus](p.Status),
		VerifiedByID: p.VerifiedBy, VerifiedAt: p.VerifiedAt, VerificationSource: p.VerificationSource, Notes: p.Notes,
		ReceivedAt: p.ReceivedAt, CreatedAt: p.CreatedAt}
}

func toInbox(i billing.InboxItem) model.PaymentInboxItem {
	return model.PaymentInboxItem{ID: i.ID, Source: i.Source, Method: upperPtr[model.PaymentMethod](i.Method), SenderNumber: i.SenderNumber,
		TrxID: i.TrxID, Amount: i.Amount, RawText: i.RawText, Status: i.Status, MatchedPaymentID: i.MatchedPaymentID, ReceivedAt: i.ReceivedAt}
}

func toLedger(l billing.LedgerEntry) *model.LedgerEntry {
	return &model.LedgerEntry{ID: l.ID, EntryType: upper[model.LedgerEntryType](l.EntryType), Amount: l.Amount, RefType: l.RefType,
		RefID: l.RefID, Note: l.Note, CreatedByID: l.CreatedBy, CreatedAt: l.CreatedAt}
}

func toDashboard(d analytics.DashboardStats) *model.DashboardStats {
	return &model.DashboardStats{ActiveSubscriptions: d.ActiveSubscriptions, ExpiringIn7Days: d.ExpiringIn7Days,
		ExpiredUnrenewed30Days: d.ExpiredUnrenewed30Days, TotalCustomers: d.TotalCustomers, NewCustomersThisMonth: d.NewCustomersThisMonth,
		Mrr: d.MRR, RevenueThisMonth: d.RevenueThisMonth, CostThisMonth: d.CostThisMonth, ProfitThisMonth: d.ProfitThisMonth,
		TotalDue: d.TotalDue, PendingPayments: d.PendingPayments, OpenTickets: d.OpenTickets, OpenConversations: d.OpenConversations,
		FreeSlots: d.FreeSlots, MasterAccountsExpiringIn7Days: d.MasterAccountsExpiringIn7Days, Currency: d.Currency}
}

func toPaymentInput(p model.PaymentInput) billing.PaymentInput {
	return billing.PaymentInput{Method: lower(p.Method), Amount: p.Amount, TrxID: p.TrxID, SenderNumber: p.SenderNumber,
		ReceivedAt: p.ReceivedAt, Notes: p.Notes}
}

// payload maps a session and prefills the user's memberships (the caller is not yet
// authenticated in ctx during register/login, so the field resolver cannot load them).
func (r *Resolver) payload(ctx context.Context, s auth.Session) (*model.AuthPayload, error) {
	p := toPayload(s)
	ms, err := r.AuthSvc.Memberships(ctx, s.User.ID)
	if err != nil {
		return nil, err
	}
	p.User.Memberships = make([]model.Membership, len(ms))
	for i, m := range ms {
		p.User.Memberships[i] = toMembership(m)
	}
	return p, nil
}

func (r *Resolver) toNote(ctx context.Context, n customer.Note) (*model.CustomerNote, error) {
	out := &model.CustomerNote{ID: n.ID, Body: n.Body, CreatedAt: n.CreatedAt}
	if n.AuthorID != nil {
		u, err := r.loaders(ctx).User.Load(ctx, *n.AuthorID)
		if err != nil {
			return nil, err
		}
		if u != nil {
			out.Author = toUser(*u)
		}
	}
	return out, nil
}

// userByID resolves an optional user reference through the loader.
func (r *Resolver) userByID(ctx context.Context, id *uuid.UUID) (*model.User, error) {
	if id == nil {
		return nil, nil
	}
	u, err := r.loaders(ctx).User.Load(ctx, *id)
	if err != nil || u == nil {
		return nil, err
	}
	return toUser(*u), nil
}

// customerByID resolves a customer reference through the loader.
func (r *Resolver) customerByID(ctx context.Context, id *uuid.UUID) (*model.Customer, error) {
	if id == nil {
		return nil, nil
	}
	c, err := r.loaders(ctx).Customer.Load(ctx, *id)
	if err != nil || c == nil {
		return nil, err
	}
	return toCustomer(*c), nil
}

func (r *Resolver) productByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	p, err := r.loaders(ctx).Product.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperr.NotFound("product")
	}
	return toProduct(*p), nil
}

func toPlanInput(p model.ProductPlanInput) catalog.PlanInput {
	return catalog.PlanInput{Name: p.Name, DurationDays: p.DurationDays, Price: p.Price, Cost: deref(p.Cost, 0),
		IsActive: deref(p.IsActive, true), SortOrder: deref(p.SortOrder, 0)}
}

func paymentPtr(p *model.PaymentInput) *billing.PaymentInput {
	if p == nil {
		return nil
	}
	v := toPaymentInput(*p)
	return &v
}
