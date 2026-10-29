package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inbox"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/bn"
)

// Dependencies (wired in app.Container).
type (
	Subscriptions interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (subscription.Subscription, error)
		MarkDelivered(ctx context.Context, tenantID, id uuid.UUID, note *string) (subscription.Subscription, error)
	}
	Catalog interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (catalog.Product, error)
		GetPlan(ctx context.Context, tenantID, id uuid.UUID) (catalog.Plan, error)
	}
	Customers interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (customer.Customer, error)
	}
	Inventory interface {
		SlotsByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]inventory.Slot, error)
		MasterAccount(ctx context.Context, tenantID, id uuid.UUID) (inventory.MasterAccount, error)
	}
	// Secrets opens a vault secret without the generic reveal audit (this package writes
	// its own delivery.render audit row).
	Secrets interface {
		Open(ctx context.Context, tenantID, secretID uuid.UUID) ([]byte, error)
	}
	Tenants interface {
		Get(ctx context.Context, tenantID uuid.UUID) (tenant.Tenant, error)
		Audit(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, action, entityType string, entityID *uuid.UUID, meta map[string]any) error
		Notify(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, kind, title string, body, link *string) error
	}
	Messenger interface {
		SendSystem(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) (inbox.Message, error)
	}
	Orders interface {
		MarkDeliveredBySubscription(ctx context.Context, tenantID, subscriptionID uuid.UUID) error
	}
)

type Deps struct {
	Subscriptions Subscriptions
	Catalog       Catalog
	Customers     Customers
	Inventory     Inventory
	Secrets       Secrets
	Tenants       Tenants
	Messenger     Messenger
	Orders        Orders
}

// Service renders and sends deliveries.
type Service interface {
	// Deliver is the delivery.send job handler. It is idempotent: an already delivered
	// subscription is skipped.
	Deliver(ctx context.Context, p Payload) error
}

type service struct {
	d   Deps
	now func() time.Time
}

func NewService(d Deps) Service { return &service{d: d, now: time.Now} }

// Rendered is the outcome of rendering a delivery template.
type Rendered struct {
	Body string // sent to the customer (may contain credentials)
	Note string // stored on the subscription: never contains the password
}

// Vars are the values a delivery template can use.
type Vars struct {
	Customer, Product, Plan, Expiry string
	SlotLabel, Pin                  string
	Username, Password, LoginHint   string
}

// Render fills a delivery template. Supported placeholders: {{customer}} {{product}}
// {{plan}} {{expiry}} {{credentials}} {{email}} {{username}} {{password}} {{slot}} {{pin}}.
func Render(tpl string, v Vars) Rendered {
	var cred []string
	if v.Username != "" {
		cred = append(cred, "Email/ID: "+v.Username)
	}
	if v.Password != "" {
		cred = append(cred, "Password: "+v.Password)
	}
	if v.SlotLabel != "" {
		cred = append(cred, "Profile: "+v.SlotLabel)
	}
	if v.Pin != "" {
		cred = append(cred, "PIN: "+v.Pin)
	}
	body := inbox.Render(tpl, map[string]string{
		"customer": v.Customer, "product": v.Product, "plan": v.Plan, "expiry": v.Expiry,
		"credentials": strings.Join(cred, "\n"), "email": v.Username, "username": v.Username, "password": v.Password,
		"slot": v.SlotLabel, "pin": v.Pin,
	})
	note := v.Product
	if v.LoginHint != "" {
		note += " · " + v.LoginHint
	}
	if v.SlotLabel != "" {
		note += " · " + v.SlotLabel
	}
	if v.Pin != "" {
		note += " · PIN ****"
	}
	return Rendered{Body: strings.TrimSpace(body), Note: note}
}

func (s *service) Deliver(ctx context.Context, p Payload) error {
	tid := p.TenantID
	sub, err := s.d.Subscriptions.Get(ctx, tid, p.SubscriptionID)
	if errors.Is(err, apperr.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if sub.DeliveredAt != nil || !subscription.IsLive(sub.Status) {
		return nil
	}
	product, err := s.d.Catalog.Get(ctx, tid, sub.ProductID)
	if err != nil {
		return err
	}
	if product.DeliveryTemplate == nil || strings.TrimSpace(*product.DeliveryTemplate) == "" {
		return nil // nothing to send automatically
	}
	cust, err := s.d.Customers.Get(ctx, tid, sub.CustomerID)
	if err != nil {
		return err
	}
	t, err := s.d.Tenants.Get(ctx, tid)
	if err != nil {
		return err
	}
	v := Vars{Customer: cust.Name, Product: product.Name, Expiry: bn.Date(sub.ExpiresAt, bn.Location(t.Timezone))}
	if sub.PlanID != nil {
		if pl, err := s.d.Catalog.GetPlan(ctx, tid, *sub.PlanID); err == nil {
			v.Plan = pl.Name
		}
	}
	if sub.SlotID != nil {
		if err := s.slotVars(ctx, tid, sub, &v); err != nil {
			return err
		}
	}
	r := Render(*product.DeliveryTemplate, v)
	if _, err := s.d.Messenger.SendSystem(ctx, tid, cust.ID, "", r.Body); err != nil {
		if errors.Is(err, apperr.ErrValidation) { // no reachable channel: tell the seller, do not retry
			body := err.Error()
			link := "/subscriptions"
			return s.d.Tenants.Notify(ctx, tid, nil, "delivery.failed", "অটো ডেলিভারি হয়নি — "+cust.Name, &body, &link)
		}
		return err
	}
	note := r.Note + " · sent " + s.now().In(bn.Location(t.Timezone)).Format("2006-01-02 15:04")
	if _, err := s.d.Subscriptions.MarkDelivered(ctx, tid, sub.ID, &note); err != nil {
		return err
	}
	return s.d.Orders.MarkDeliveredBySubscription(ctx, tid, sub.ID)
}

// slotVars adds slot label/PIN and the master-account credential (vault, audit-logged).
func (s *service) slotVars(ctx context.Context, tid uuid.UUID, sub subscription.Subscription, v *Vars) error {
	slots, err := s.d.Inventory.SlotsByIDs(ctx, tid, []uuid.UUID{*sub.SlotID})
	if err != nil {
		return err
	}
	sl, ok := slots[*sub.SlotID]
	if !ok {
		return nil
	}
	v.SlotLabel = fmt.Sprintf("Slot %d", sl.SlotNo)
	if sl.Label != nil && *sl.Label != "" {
		v.SlotLabel = *sl.Label
	}
	if sl.Pin != nil {
		v.Pin = *sl.Pin
	}
	m, err := s.d.Inventory.MasterAccount(ctx, tid, sl.MasterAccountID)
	if err != nil {
		return err
	}
	if m.LoginHint != nil {
		v.LoginHint = *m.LoginHint
	}
	if m.CredentialSecretID == nil {
		return nil
	}
	raw, err := s.d.Secrets.Open(ctx, tid, *m.CredentialSecretID)
	if err != nil {
		return err
	}
	var cred inventory.Credential
	if err := json.Unmarshal(raw, &cred); err != nil {
		return fmt.Errorf("delivery: credential of %s is not valid JSON", m.Label)
	}
	v.Username, v.Password = cred.Username, cred.Password
	return s.d.Tenants.Audit(ctx, tid, nil, "delivery.render", "subscription", &sub.ID,
		map[string]any{"master_account_id": m.ID, "slot_id": sl.ID, "customer_id": sub.CustomerID})
}
