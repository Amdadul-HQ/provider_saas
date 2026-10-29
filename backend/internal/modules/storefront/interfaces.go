package storefront

import (
	"context"

	"github.com/google/uuid"

	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/subscription"
)

// Dependencies the storefront implementation is expected to use (all already
// implemented and wired in app.Container).
type (
	Billing interface {
		RecordPayment(ctx context.Context, tenantID, actorID uuid.UUID, invoiceID, customerID *uuid.UUID, in billing.PaymentInput, opts billing.RecordOptions) (billing.Payment, error)
	}
	Subscriptions interface {
		Create(ctx context.Context, tenantID, actorID uuid.UUID, in subscription.CreateInput) (subscription.Subscription, error)
	}
)
