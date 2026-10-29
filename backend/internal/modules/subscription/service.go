package subscription

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
)

// Dependencies (service interfaces of other modules; wired in app.Container).
type (
	Customers interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (customer.Customer, error)
	}
	Catalog interface {
		Get(ctx context.Context, tenantID, id uuid.UUID) (catalog.Product, error)
		GetPlan(ctx context.Context, tenantID, id uuid.UUID) (catalog.Plan, error)
	}
	Slots interface {
		PickFreeSlot(ctx context.Context, tenantID, productID uuid.UUID) (inventory.Slot, error)
		LockFreeSlot(ctx context.Context, tenantID, slotID, productID uuid.UUID) (inventory.Slot, error)
		AssignSlot(ctx context.Context, tenantID, slotID, subscriptionID uuid.UUID) error
		ReleaseSlot(ctx context.Context, tenantID, slotID, subscriptionID uuid.UUID) (bool, error)
		TransferSlot(ctx context.Context, tenantID, slotID, fromSub, toSub uuid.UUID) (bool, error)
	}
	Billing interface {
		CreateInvoice(ctx context.Context, tenantID, actorID uuid.UUID, in billing.CreateInvoiceInput) (billing.Invoice, error)
		RecordPayment(ctx context.Context, tenantID, actorID uuid.UUID, invoiceID, customerID *uuid.UUID, in billing.PaymentInput, opts billing.RecordOptions) (billing.Payment, error)
	}
)

// Service is the subscription module API.
type Service interface {
	Create(ctx context.Context, tenantID, actorID uuid.UUID, in CreateInput) (Subscription, error)
	Renew(ctx context.Context, tenantID, actorID, id uuid.UUID, in RenewInput) (Subscription, error)
	MarkDelivered(ctx context.Context, tenantID, id uuid.UUID, note *string) (Subscription, error)
	Cancel(ctx context.Context, tenantID, id uuid.UUID, reason *string) (Subscription, error)
	ReassignSlot(ctx context.Context, tenantID, id, slotID uuid.UUID) (Subscription, error)
	Extend(ctx context.Context, tenantID, id uuid.UUID, days int, reason *string) (Subscription, error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (Subscription, error)
	GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Subscription, error)
	List(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) ([]Subscription, bool, int, error)
	Expiring(ctx context.Context, tenantID uuid.UUID, days int) ([]Subscription, error)
	ByCustomers(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Subscription, error)
	ActiveCountByCustomers(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error)
	ActiveCountByProducts(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error)
	// ExpireCheck applies time-based status changes for one tenant and frees slots of expired subscriptions.
	ExpireCheck(ctx context.Context, tenantID uuid.UUID, now time.Time) (ExpireCheckResult, error)
	TenantsWithLiveSubscriptions(ctx context.Context) ([]uuid.UUID, error)
}

type service struct {
	pool      *pgxpool.Pool
	repo      Repository
	customers Customers
	catalog   Catalog
	slots     Slots
	billing   Billing
	now       func() time.Time
}

func NewService(pool *pgxpool.Pool, customers Customers, cat Catalog, slots Slots, bill Billing) Service {
	return &service{pool: pool, customers: customers, catalog: cat, slots: slots, billing: bill, now: time.Now}
}

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func actor(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func appendNote(notes *string, line string) *string {
	if line == "" {
		return notes
	}
	if notes == nil || *notes == "" {
		return &line
	}
	v := *notes + "\n" + line
	return &v
}

// terms resolves product, plan and commercial terms.
func (s *service) terms(ctx context.Context, tenantID, productID uuid.UUID, planID *uuid.UUID, dur *int, price, cost *int64) (catalog.Product, *catalog.Plan, Terms, error) {
	p, err := s.catalog.Get(ctx, tenantID, productID)
	if err != nil {
		return p, nil, Terms{}, err
	}
	var plan *catalog.Plan
	var pd *int
	var pp, pc *int64
	var cur string
	if planID != nil {
		pl, err := s.catalog.GetPlan(ctx, tenantID, *planID)
		if err != nil {
			return p, nil, Terms{}, err
		}
		if pl.ProductID != productID {
			return p, nil, Terms{}, apperr.ValidationField("planId", "plan does not belong to this product")
		}
		plan, pd, pp, pc, cur = &pl, &pl.DurationDays, &pl.Price, &pl.Cost, pl.Currency
	}
	t, err := ResolveTerms(pd, pp, pc, cur, dur, price, cost)
	return p, plan, t, err
}

// allocateSlot picks (or locks the requested) slot for shared-slot products.
func (s *service) allocateSlot(ctx context.Context, tenantID uuid.UUID, product catalog.Product, slotID *uuid.UUID) (*uuid.UUID, error) {
	if slotID != nil {
		sl, err := s.slots.LockFreeSlot(ctx, tenantID, *slotID, product.ID)
		if err != nil {
			return nil, err
		}
		return &sl.ID, nil
	}
	if product.DeliveryType != catalog.DeliverySharedSlot {
		return nil, nil
	}
	sl, err := s.slots.PickFreeSlot(ctx, tenantID, product.ID)
	if err != nil {
		return nil, err // apperr.Conflict("no free slot")
	}
	return &sl.ID, nil
}

// bill creates the optional invoice and payment for a sale (same tx).
func (s *service) bill(ctx context.Context, tenantID, actorID uuid.UUID, sub Subscription, product catalog.Product, plan *catalog.Plan, createInvoice bool, pay *billing.PaymentInput) error {
	var invoiceID *uuid.UUID
	if createInvoice {
		desc := product.Name
		if plan != nil {
			desc += " — " + plan.Name
		} else {
			desc += fmt.Sprintf(" — %d days", int(sub.ExpiresAt.Sub(sub.StartsAt).Hours()/24))
		}
		due := sub.StartsAt
		inv, err := s.billing.CreateInvoice(ctx, tenantID, actorID, billing.CreateInvoiceInput{
			CustomerID: sub.CustomerID, DueAt: &due,
			Items: []billing.ItemInput{{SubscriptionID: &sub.ID, Description: desc, Quantity: 1, UnitPrice: sub.Price}},
		})
		if err != nil {
			return err
		}
		invoiceID = &inv.ID
	}
	if pay != nil {
		_, err := s.billing.RecordPayment(ctx, tenantID, actorID, invoiceID, &sub.CustomerID, *pay, billing.RecordOptions{ForceVerified: true})
		return err
	}
	return nil
}

func (s *service) Create(ctx context.Context, tenantID, actorID uuid.UUID, in CreateInput) (out Subscription, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.customers.Get(ctx, tenantID, in.CustomerID); err != nil {
			return err
		}
		product, plan, t, err := s.terms(ctx, tenantID, in.ProductID, in.PlanID, in.DurationDays, in.Price, in.Cost)
		if err != nil {
			return err
		}
		if !product.IsActive {
			return apperr.Validation("product is not active")
		}
		now := s.now()
		starts := now
		if in.StartsAt != nil {
			starts = *in.StartsAt
		}
		expires := ExpiryFor(starts, t.DurationDays)
		status := StatusAt(starts, expires, now)
		// A back-dated sale that has already ended (data import) holds no slot.
		var slotID *uuid.UUID
		if IsLive(status) {
			if slotID, err = s.allocateSlot(ctx, tenantID, product, in.SlotID); err != nil {
				return err
			}
		}
		out, err = s.repo.Insert(ctx, tx, tenantID, Subscription{
			CustomerID: in.CustomerID, ProductID: in.ProductID, PlanID: in.PlanID, SlotID: slotID,
			StartsAt: starts, ExpiresAt: expires, Price: t.Price, Cost: t.Cost, Currency: t.Currency,
			Status: status, AutoRenew: in.AutoRenew, SoldBy: actor(actorID), Notes: in.Notes,
		})
		if err != nil {
			return err
		}
		if slotID != nil {
			if err := s.slots.AssignSlot(ctx, tenantID, *slotID, out.ID); err != nil {
				return err
			}
		}
		return s.bill(ctx, tenantID, actorID, out, product, plan, in.CreateInvoice, in.MarkPaid)
	})
	return out, err
}

// Renew creates a new subscription row (renewed_from_id = id) starting when the old one
// ends (or now if already expired). The old slot is transferred when still held, otherwise
// a new free slot is picked for shared-slot products.
func (s *service) Renew(ctx context.Context, tenantID, actorID, id uuid.UUID, in RenewInput) (out Subscription, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		old, err := s.repo.Get(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if old.Status == StatusCancelled || old.Status == StatusReplaced {
			return apperr.Validation("cannot renew a " + old.Status + " subscription")
		}
		if succ, err := s.repo.LiveSuccessor(ctx, tx, tenantID, id); err != nil {
			return err
		} else if succ != nil {
			return apperr.Conflict("subscription already renewed")
		}
		planID := in.PlanID
		if planID == nil {
			planID = old.PlanID
		}
		dur := in.DurationDays
		price := in.Price
		if planID == nil {
			if dur == nil {
				d := int(old.ExpiresAt.Sub(old.StartsAt).Hours() / 24)
				dur = &d
			}
			if price == nil {
				price = &old.Price
			}
		}
		product, plan, t, err := s.terms(ctx, tenantID, old.ProductID, planID, dur, price, nil)
		if err != nil {
			return err
		}
		if plan == nil {
			t.Cost = old.Cost
		}
		now := s.now()
		starts := now
		if old.ExpiresAt.After(now) {
			starts = old.ExpiresAt
		}
		expires := ExpiryFor(starts, t.DurationDays)
		out, err = s.repo.Insert(ctx, tx, tenantID, Subscription{
			CustomerID: old.CustomerID, ProductID: old.ProductID, PlanID: planID, StartsAt: starts, ExpiresAt: expires,
			Price: t.Price, Cost: t.Cost, Currency: t.Currency, Status: StatusAt(starts, expires, now), AutoRenew: old.AutoRenew,
			RenewedFromID: &old.ID, SoldBy: actor(actorID), DeliveredAt: nil,
		})
		if err != nil {
			return err
		}
		var slotID *uuid.UUID
		if old.SlotID != nil {
			moved, err := s.slots.TransferSlot(ctx, tenantID, *old.SlotID, old.ID, out.ID)
			if err != nil {
				return err
			}
			if moved {
				slotID = old.SlotID
			}
		}
		if slotID == nil && product.DeliveryType == catalog.DeliverySharedSlot {
			if slotID, err = s.allocateSlot(ctx, tenantID, product, nil); err != nil {
				return err
			}
			if err := s.slots.AssignSlot(ctx, tenantID, *slotID, out.ID); err != nil {
				return err
			}
		}
		if slotID != nil {
			out.SlotID = slotID
			// A renewal on the same slot needs no new delivery.
			if old.SlotID != nil && *old.SlotID == *slotID && old.DeliveredAt != nil {
				out.DeliveredAt, out.DeliveryNote = &now, old.DeliveryNote
			}
			if out, err = s.repo.Update(ctx, tx, tenantID, out); err != nil {
				return err
			}
		}
		return s.bill(ctx, tenantID, actorID, out, product, plan, in.CreateInvoice, in.MarkPaid)
	})
	return out, err
}

func (s *service) MarkDelivered(ctx context.Context, tenantID, id uuid.UUID, note *string) (out Subscription, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.Get(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if !IsLive(cur.Status) {
			return apperr.Validation("cannot deliver a " + cur.Status + " subscription")
		}
		now := s.now()
		cur.DeliveredAt = &now
		if note != nil {
			cur.DeliveryNote = note
		}
		cur.Status = StatusAt(cur.StartsAt, cur.ExpiresAt, now)
		out, err = s.repo.Update(ctx, tx, tenantID, cur)
		return err
	})
	return out, err
}

func (s *service) Cancel(ctx context.Context, tenantID, id uuid.UUID, reason *string) (out Subscription, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.Get(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if !CanTransition(cur.Status, StatusCancelled) || cur.Status == StatusCancelled {
			return apperr.Validation("cannot cancel a " + cur.Status + " subscription")
		}
		if cur.SlotID != nil {
			if _, err := s.slots.ReleaseSlot(ctx, tenantID, *cur.SlotID, cur.ID); err != nil {
				return err
			}
		}
		cur.Status = StatusCancelled
		line := "cancelled"
		if reason != nil && strings.TrimSpace(*reason) != "" {
			line += ": " + strings.TrimSpace(*reason)
		}
		cur.Notes = appendNote(cur.Notes, line)
		out, err = s.repo.Update(ctx, tx, tenantID, cur)
		return err
	})
	return out, err
}

func (s *service) ReassignSlot(ctx context.Context, tenantID, id, slotID uuid.UUID) (out Subscription, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.Get(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if !IsLive(cur.Status) {
			return apperr.Validation("cannot reassign a " + cur.Status + " subscription")
		}
		if cur.SlotID != nil && *cur.SlotID == slotID {
			out = cur
			return nil
		}
		if _, err := s.slots.LockFreeSlot(ctx, tenantID, slotID, cur.ProductID); err != nil {
			return err
		}
		if cur.SlotID != nil {
			if _, err := s.slots.ReleaseSlot(ctx, tenantID, *cur.SlotID, cur.ID); err != nil {
				return err
			}
		}
		if err := s.slots.AssignSlot(ctx, tenantID, slotID, cur.ID); err != nil {
			return err
		}
		cur.SlotID = &slotID
		cur.DeliveredAt = nil // new credentials must be delivered again
		out, err = s.repo.Update(ctx, tx, tenantID, cur)
		return err
	})
	return out, err
}

func (s *service) Extend(ctx context.Context, tenantID, id uuid.UUID, days int, reason *string) (out Subscription, err error) {
	if days == 0 || days > 3650 || days < -3650 {
		return out, apperr.ValidationField("days", "must be non-zero and within ±3650")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.Get(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if cur.Status == StatusCancelled || cur.Status == StatusReplaced {
			return apperr.Validation("cannot extend a " + cur.Status + " subscription")
		}
		newExp := cur.ExpiresAt.AddDate(0, 0, days)
		if !newExp.After(cur.StartsAt) {
			return apperr.ValidationField("days", "would end before the subscription starts")
		}
		now := s.now()
		wasLive := IsLive(cur.Status)
		next := StatusAt(cur.StartsAt, newExp, now)
		if !CanTransition(cur.Status, next) {
			return apperr.Validation(fmt.Sprintf("cannot move from %s to %s", cur.Status, next))
		}
		// Reviving an expired shared-slot subscription needs its slot back (or a new one).
		if !wasLive && IsLive(next) {
			product, err := s.catalog.Get(ctx, tenantID, cur.ProductID)
			if err != nil {
				return err
			}
			if product.DeliveryType == catalog.DeliverySharedSlot {
				var slotID *uuid.UUID
				if cur.SlotID != nil {
					if sl, err := s.slots.LockFreeSlot(ctx, tenantID, *cur.SlotID, cur.ProductID); err == nil {
						slotID = &sl.ID
					}
				}
				if slotID == nil {
					if slotID, err = s.allocateSlot(ctx, tenantID, product, nil); err != nil {
						return err
					}
					cur.DeliveredAt = nil
				}
				if err := s.slots.AssignSlot(ctx, tenantID, *slotID, cur.ID); err != nil {
					return err
				}
				cur.SlotID = slotID
			}
		}
		if wasLive && !IsLive(next) && cur.SlotID != nil {
			if _, err := s.slots.ReleaseSlot(ctx, tenantID, *cur.SlotID, cur.ID); err != nil {
				return err
			}
		}
		line := fmt.Sprintf("extended %+d days", days)
		if reason != nil && strings.TrimSpace(*reason) != "" {
			line += ": " + strings.TrimSpace(*reason)
		}
		cur.ExpiresAt, cur.Status, cur.Notes = newExp, next, appendNote(cur.Notes, line)
		out, err = s.repo.Update(ctx, tx, tenantID, cur)
		return err
	})
	return out, err
}

func (s *service) Get(ctx context.Context, tenantID, id uuid.UUID) (out Subscription, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.Get(ctx, tx, tenantID, id, false)
		return err
	})
	return out, err
}

func (s *service) GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Subscription, error) {
	out := map[uuid.UUID]Subscription{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ss, err := s.repo.GetByIDs(ctx, tx, tenantID, ids)
		for _, x := range ss {
			out[x.ID] = x
		}
		return err
	})
	return out, err
}

func (s *service) List(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) (items []Subscription, more bool, total int, err error) {
	if f.Status != nil {
		v := strings.ToLower(*f.Status)
		f.Status = &v
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.repo.List(ctx, tx, tenantID, f, page)
		if err != nil {
			return err
		}
		items, more = pagination.Trim(rows, page.First)
		total, err = s.repo.Count(ctx, tx, tenantID, f)
		return err
	})
	return items, more, total, err
}

func (s *service) Expiring(ctx context.Context, tenantID uuid.UUID, days int) (out []Subscription, err error) {
	if days <= 0 {
		days = 7
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.Expiring(ctx, tx, tenantID, days)
		return err
	})
	return out, err
}

func (s *service) ByCustomers(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Subscription, error) {
	out := map[uuid.UUID][]Subscription{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ss, err := s.repo.ByCustomers(ctx, tx, tenantID, ids)
		for _, x := range ss {
			out[x.CustomerID] = append(out[x.CustomerID], x)
		}
		return err
	})
	return out, err
}

func (s *service) ActiveCountByCustomers(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (out map[uuid.UUID]int, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ActiveCountByCustomers(ctx, tx, tenantID, ids)
		return err
	})
	return out, err
}

func (s *service) ActiveCountByProducts(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (out map[uuid.UUID]int, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ActiveCountByProducts(ctx, tx, tenantID, ids)
		return err
	})
	return out, err
}

func (s *service) ExpireCheck(ctx context.Context, tenantID uuid.UUID, now time.Time) (res ExpireCheckResult, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		expired, err := s.repo.ExpireDue(ctx, tx, tenantID, now)
		if err != nil {
			return err
		}
		res.Expired = int64(len(expired))
		for _, e := range expired {
			if e.SlotID == nil {
				continue
			}
			freed, err := s.slots.ReleaseSlot(ctx, tenantID, *e.SlotID, e.ID)
			if err != nil {
				return err
			}
			if freed {
				res.SlotsFreed++
			}
		}
		if res.Activated, err = s.repo.Activate(ctx, tx, tenantID, now); err != nil {
			return err
		}
		res.Expiring, err = s.repo.MarkExpiring(ctx, tx, tenantID, now, ExpiringWindow)
		return err
	})
	return res, err
}

// TenantsWithLiveSubscriptions is a platform-level query used by the expire_check job
// to fan out per tenant (each tenant is then processed in its own WithTenantTx).
func (s *service) TenantsWithLiveSubscriptions(ctx context.Context) (out []uuid.UUID, err error) {
	err = db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.TenantsWithLiveSubscriptions(ctx, tx)
		return err
	})
	return out, err
}
