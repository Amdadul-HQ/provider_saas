package subscription

import (
	"math"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/modules/billing"
	"provider_saas/internal/platform/apperr"
)

// Subscription statuses and lifecycle.
//
//	pending  -> active | expiring | expired | cancelled        (starts in the future, e.g. early renewal)
//	active   -> expiring | expired | cancelled | replaced
//	expiring -> active (extended) | expired | cancelled | replaced
//	expired  -> active | expiring (extended)                    (renewal creates a NEW row instead)
//	cancelled, replaced: terminal
//
// Time-based moves (pending->active, active->expiring <=3 days, ->expired) are done by the
// subscription.expire_check job; expiring frees the slot only at expiry.
const (
	StatusPending   = "pending"
	StatusActive    = "active"
	StatusExpiring  = "expiring"
	StatusExpired   = "expired"
	StatusCancelled = "cancelled"
	StatusReplaced  = "replaced"
)

// ExpiringWindow: a subscription this close to expiry is "expiring".
const ExpiringWindow = 3 * 24 * time.Hour

var transitions = map[string]map[string]bool{
	StatusPending:  {StatusActive: true, StatusExpiring: true, StatusExpired: true, StatusCancelled: true},
	StatusActive:   {StatusExpiring: true, StatusExpired: true, StatusCancelled: true, StatusReplaced: true},
	StatusExpiring: {StatusActive: true, StatusExpired: true, StatusCancelled: true, StatusReplaced: true},
	StatusExpired:  {StatusActive: true, StatusExpiring: true},
}

// CanTransition reports whether from -> to is allowed.
func CanTransition(from, to string) bool {
	if from == to {
		return true
	}
	return transitions[from][to]
}

// IsLive reports whether the subscription currently holds an entitlement (and its slot).
func IsLive(status string) bool {
	return status == StatusPending || status == StatusActive || status == StatusExpiring
}

// StatusAt derives the time-based status for a live subscription.
func StatusAt(startsAt, expiresAt, now time.Time) string {
	switch {
	case !expiresAt.After(now):
		return StatusExpired
	case startsAt.After(now):
		return StatusPending
	case expiresAt.Sub(now) <= ExpiringWindow:
		return StatusExpiring
	default:
		return StatusActive
	}
}

// DaysLeft is the number of (partial) days until expiry; negative once expired.
func DaysLeft(expiresAt, now time.Time) int {
	d := expiresAt.Sub(now).Hours() / 24
	if d >= 0 {
		return int(math.Ceil(d))
	}
	return int(math.Floor(d))
}

// ExpiryFor computes expires_at = starts + duration days.
func ExpiryFor(startsAt time.Time, durationDays int) time.Time {
	return startsAt.AddDate(0, 0, durationDays)
}

type Subscription struct {
	ID            uuid.UUID
	CustomerID    uuid.UUID
	ProductID     uuid.UUID
	PlanID        *uuid.UUID
	SlotID        *uuid.UUID
	StartsAt      time.Time
	ExpiresAt     time.Time
	Price         int64
	Cost          int64
	Currency      string
	Status        string
	AutoRenew     bool
	DeliveredAt   *time.Time
	DeliveryNote  *string
	RenewedFromID *uuid.UUID
	SoldBy        *uuid.UUID
	Notes         *string
	CreatedAt     time.Time
}

type CreateInput struct {
	CustomerID    uuid.UUID
	ProductID     uuid.UUID
	PlanID        *uuid.UUID
	SlotID        *uuid.UUID
	StartsAt      *time.Time
	DurationDays  *int
	Price         *int64
	Cost          *int64
	AutoRenew     bool
	Notes         *string
	CreateInvoice bool
	MarkPaid      *billing.PaymentInput
}

type RenewInput struct {
	PlanID        *uuid.UUID
	DurationDays  *int
	Price         *int64
	CreateInvoice bool
	MarkPaid      *billing.PaymentInput
}

type Filter struct {
	Search             *string
	Status             *string
	ProductID          *uuid.UUID
	CustomerID         *uuid.UUID
	ExpiringWithinDays *int
	ExpiredWithinDays  *int
}

// Terms are the resolved commercial terms of a sale.
type Terms struct {
	DurationDays int
	Price        int64
	Cost         int64
	Currency     string
}

// ResolveTerms applies overrides on top of plan defaults.
func ResolveTerms(planDuration *int, planPrice, planCost *int64, planCurrency string, durationDays *int, price, cost *int64) (Terms, error) {
	t := Terms{Currency: planCurrency}
	if t.Currency == "" {
		t.Currency = "BDT"
	}
	switch {
	case durationDays != nil:
		t.DurationDays = *durationDays
	case planDuration != nil:
		t.DurationDays = *planDuration
	default:
		return t, apperr.Validation("durationDays is required when no plan is given")
	}
	if t.DurationDays <= 0 || t.DurationDays > 3650 {
		return t, apperr.ValidationField("durationDays", "must be 1..3650")
	}
	switch {
	case price != nil:
		t.Price = *price
	case planPrice != nil:
		t.Price = *planPrice
	default:
		return t, apperr.Validation("price is required when no plan is given")
	}
	if cost != nil {
		t.Cost = *cost
	} else if planCost != nil {
		t.Cost = *planCost
	}
	if t.Price < 0 || t.Cost < 0 {
		return t, apperr.Validation("price and cost must be >= 0")
	}
	return t, nil
}

// ExpireCheckResult summarises one expire_check run for a tenant.
type ExpireCheckResult struct {
	Activated  int64
	Expiring   int64
	Expired    int64
	SlotsFreed int64
}
