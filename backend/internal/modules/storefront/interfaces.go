package storefront

import (
	"context"

	"github.com/google/uuid"

	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/platform/pagination"
)

// Dependencies (service interfaces of other modules; wired in app.Container).
type (
	Customers interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (customer.Customer, error)
		Create(ctx context.Context, tenantID, actorID uuid.UUID, in customer.CreateInput) (customer.Customer, error)
		FindByPhone(ctx context.Context, tenantID uuid.UUID, phone string) (customer.Customer, error)
	}
	Catalog interface {
		List(ctx context.Context, tenantID uuid.UUID, f catalog.Filter, limit int) ([]catalog.Product, error)
		Get(ctx context.Context, tenantID, id uuid.UUID) (catalog.Product, error)
		GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]catalog.Product, error)
		GetPlan(ctx context.Context, tenantID, id uuid.UUID) (catalog.Plan, error)
		PlansByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]catalog.Plan, error)
		PlansByProducts(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]catalog.Plan, error)
	}
	Inventory interface {
		FreeCountsByProduct(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error)
	}
	Billing interface {
		CreateInvoice(ctx context.Context, tenantID, actorID uuid.UUID, in billing.CreateInvoiceInput) (billing.Invoice, error)
		RecordPayment(ctx context.Context, tenantID, actorID uuid.UUID, invoiceID, customerID *uuid.UUID, in billing.PaymentInput, opts billing.RecordOptions) (billing.Payment, error)
		Payment(ctx context.Context, tenantID, id uuid.UUID) (billing.Payment, error)
		VerifyPayment(ctx context.Context, tenantID, actorID, id uuid.UUID) (billing.Payment, error)
		RejectPayment(ctx context.Context, tenantID, actorID, id uuid.UUID, reason *string) (billing.Payment, error)
		AttachPayment(ctx context.Context, tenantID, paymentID, invoiceID uuid.UUID) (billing.Payment, error)
		Invoices(ctx context.Context, tenantID uuid.UUID, f billing.InvoiceFilter, page pagination.Page) ([]billing.Invoice, bool, int, error)
	}
	Subscriptions interface {
		Create(ctx context.Context, tenantID, actorID uuid.UUID, in subscription.CreateInput) (subscription.Subscription, error)
		Get(ctx context.Context, tenantID, id uuid.UUID) (subscription.Subscription, error)
		MarkDelivered(ctx context.Context, tenantID, id uuid.UUID, note *string) (subscription.Subscription, error)
		ByCustomers(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]subscription.Subscription, error)
	}
	// Messenger is inbox.Sender.
	Messenger interface {
		SendToCustomer(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) error
	}
	Notifier interface {
		Notify(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, kind, title string, body, link *string) error
	}
)
