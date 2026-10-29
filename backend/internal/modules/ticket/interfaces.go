package ticket

import (
	"context"

	"github.com/google/uuid"

	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/subscription"
)

// Dependencies the ticket implementation is expected to use for replacements.
type (
	Subscriptions interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (subscription.Subscription, error)
		ReassignSlot(ctx context.Context, tenantID, id, slotID uuid.UUID) (subscription.Subscription, error)
		Extend(ctx context.Context, tenantID, id uuid.UUID, days int, reason *string) (subscription.Subscription, error)
	}
	Slots interface {
		PickFreeSlot(ctx context.Context, tenantID, productID uuid.UUID) (inventory.Slot, error)
		UpdateSlot(ctx context.Context, tenantID, id uuid.UUID, in inventory.UpdateSlotInput) (inventory.Slot, error)
	}
)
