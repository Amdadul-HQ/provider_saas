package ticket

import (
	"context"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/tenant"
)

// Dependencies (service interfaces of other modules; wired in app.Container).
type (
	Subscriptions interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (subscription.Subscription, error)
		ReassignSlot(ctx context.Context, tenantID, id, slotID uuid.UUID) (subscription.Subscription, error)
		Extend(ctx context.Context, tenantID, id uuid.UUID, days int, reason *string) (subscription.Subscription, error)
	}
	// Slots is the inventory slot picker (no SQL is copied here).
	Slots interface {
		PickFreeSlot(ctx context.Context, tenantID, productID uuid.UUID) (inventory.Slot, error)
		UpdateSlot(ctx context.Context, tenantID, id uuid.UUID, in inventory.UpdateSlotInput) (inventory.Slot, error)
		SlotsByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]inventory.Slot, error)
	}
	Customers interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (customer.Customer, error)
	}
	Catalog interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (catalog.Product, error)
	}
	Members interface {
		Membership(ctx context.Context, userID, tenantID uuid.UUID) (tenant.Membership, error)
	}
	// Messenger is inbox.Sender (customer-visible comments are messaged to the customer).
	Messenger interface {
		SendToCustomer(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) error
	}
	Queue interface {
		Enqueue(ctx context.Context, kind string, payload any, runAt time.Time, dedupeKey string) (int64, error)
	}
)
