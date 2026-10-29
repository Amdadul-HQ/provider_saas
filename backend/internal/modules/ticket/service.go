package ticket

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/modules/delivery"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
)

// Service is the ticket module API.
type Service interface {
	Create(ctx context.Context, tenantID, actorID uuid.UUID, in CreateInput) (Ticket, error)
	Update(ctx context.Context, tenantID, actorID, id uuid.UUID, in UpdateInput) (Ticket, error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (Ticket, error)
	List(ctx context.Context, tenantID uuid.UUID, f Filter, after *uuid.UUID, limit int) ([]Ticket, error)
	Stats(ctx context.Context, tenantID uuid.UUID) (Stats, error)
	AddComment(ctx context.Context, tenantID, actorID, ticketID uuid.UUID, body string, internal bool) (Comment, error)
	CommentsByTickets(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Comment, error)
	// ResolveReplacement moves the ticket's subscription to newSlotID (or the next free slot
	// of the product), marks the old slot (banned|retired|free), optionally extends the
	// subscription, resolves the ticket and queues delivery of the new slot — one transaction.
	ResolveReplacement(ctx context.Context, tenantID, actorID, ticketID uuid.UUID, newSlotID *uuid.UUID, markOldSlot string, extendDays int) (Ticket, error)
}

type Deps struct {
	Subscriptions Subscriptions
	Slots         Slots
	Customers     Customers
	Catalog       Catalog
	Members       Members
	Messenger     Messenger
	Queue         Queue
}

type service struct {
	pool *pgxpool.Pool
	repo Repository
	d    Deps
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, d Deps) Service {
	return &service{pool: pool, d: d, now: time.Now}
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

func (s *service) Create(ctx context.Context, tenantID, actorID uuid.UUID, in CreateInput) (t Ticket, err error) {
	if err := in.Validate(); err != nil {
		return t, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.d.Customers.Get(ctx, tenantID, in.CustomerID); err != nil {
			return err
		}
		var oldSlot *uuid.UUID
		if in.SubscriptionID != nil {
			sub, err := s.d.Subscriptions.Get(ctx, tenantID, *in.SubscriptionID)
			if err != nil {
				return err
			}
			if sub.CustomerID != in.CustomerID {
				return apperr.ValidationField("subscriptionId", "subscription belongs to another customer")
			}
			oldSlot = sub.SlotID
		}
		t, err = s.repo.Insert(ctx, tx, tenantID, in, oldSlot, actor(actorID))
		return err
	})
	return t, err
}

func (s *service) Update(ctx context.Context, tenantID, actorID, id uuid.UUID, in UpdateInput) (t Ticket, err error) {
	if err := in.Validate(); err != nil {
		return t, err
	}
	if in.AssignedTo != nil {
		m, err := s.d.Members.Membership(ctx, *in.AssignedTo, tenantID)
		if err != nil || m.Status != "active" {
			return t, apperr.ValidationField("assignedTo", "not an active member of this business")
		}
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.Get(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		prev := cur.Status
		if in.Priority != nil {
			cur.Priority = *in.Priority
		}
		if in.Title != nil {
			cur.Title = *in.Title
		}
		if in.Description != nil {
			cur.Description = in.Description
		}
		if in.Reason != nil {
			cur.Reason = in.Reason
		}
		if in.AssignedTo != nil {
			cur.AssignedTo = in.AssignedTo
		}
		if in.Status != nil && *in.Status != prev {
			cur.Status = *in.Status
			switch {
			case cur.IsDone() && cur.ResolvedAt == nil:
				now := s.now()
				cur.ResolvedAt = &now
			case !cur.IsDone():
				cur.ResolvedAt = nil
			}
			if _, err := s.repo.InsertComment(ctx, tx, tenantID, id, actor(actorID), fmt.Sprintf("status: %s → %s", prev, cur.Status), true); err != nil {
				return err
			}
		}
		t, err = s.repo.Save(ctx, tx, tenantID, cur)
		return err
	})
	return t, err
}

func (s *service) Get(ctx context.Context, tenantID, id uuid.UUID) (t Ticket, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		t, err = s.repo.Get(ctx, tx, tenantID, id, false)
		return err
	})
	return t, err
}

func (s *service) List(ctx context.Context, tenantID uuid.UUID, f Filter, after *uuid.UUID, limit int) (out []Ticket, err error) {
	if limit <= 0 || limit > pagination.MaxFirst {
		limit = pagination.DefaultFirst
	}
	for _, p := range []**string{&f.Status, &f.Kind, &f.Priority} {
		if *p != nil {
			v := strings.ToLower(**p)
			*p = &v
		}
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.List(ctx, tx, tenantID, f, after, limit)
		return err
	})
	return out, err
}

func (s *service) Stats(ctx context.Context, tenantID uuid.UUID) (st Stats, err error) {
	now := s.now().In(time.FixedZone("Asia/Dhaka", 6*3600))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		st, err = s.repo.Stats(ctx, tx, tenantID, monthStart)
		return err
	})
	return st, err
}

func (s *service) AddComment(ctx context.Context, tenantID, actorID, ticketID uuid.UUID, body string, internal bool) (c Comment, err error) {
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 4000 {
		return c, apperr.ValidationField("body", "1..4000 characters")
	}
	var customerID uuid.UUID
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		t, err := s.repo.Get(ctx, tx, tenantID, ticketID, false)
		if err != nil {
			return err
		}
		customerID = t.CustomerID
		c, err = s.repo.InsertComment(ctx, tx, tenantID, ticketID, actor(actorID), body, internal)
		return err
	})
	if err != nil || internal {
		return c, err
	}
	// Customer-visible comments are also messaged to the customer (best effort when the
	// customer has no reachable channel yet).
	if err := s.d.Messenger.SendToCustomer(ctx, tenantID, customerID, "", body); err != nil && !errors.Is(err, apperr.ErrValidation) {
		return c, err
	}
	return c, nil
}

func (s *service) CommentsByTickets(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Comment, error) {
	out := map[uuid.UUID][]Comment{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cs, err := s.repo.CommentsByTickets(ctx, tx, tenantID, ids)
		for _, c := range cs {
			out[c.TicketID] = append(out[c.TicketID], c)
		}
		return err
	})
	return out, err
}

func slotName(sl inventory.Slot) string {
	if sl.Label != nil && *sl.Label != "" {
		return *sl.Label
	}
	return fmt.Sprintf("slot %d", sl.SlotNo)
}

func (s *service) ResolveReplacement(ctx context.Context, tenantID, actorID, ticketID uuid.UUID, newSlotID *uuid.UUID, markOldSlot string, extendDays int) (t Ticket, err error) {
	mark, err := ValidOldSlotStatus(markOldSlot)
	if err != nil {
		return t, err
	}
	if extendDays < 0 || extendDays > 365 {
		return t, apperr.ValidationField("extendDays", "must be 0..365")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.Get(ctx, tx, tenantID, ticketID, true)
		if err != nil {
			return err
		}
		switch {
		case cur.IsDone():
			return apperr.Validation("ticket is already " + cur.Status)
		case cur.SubscriptionID == nil:
			return apperr.Validation("ticket has no subscription to replace")
		}
		sub, err := s.d.Subscriptions.Get(ctx, tenantID, *cur.SubscriptionID)
		if err != nil {
			return err
		}
		if !subscription.IsLive(sub.Status) {
			return apperr.Validation("cannot replace a " + sub.Status + " subscription; renew it first")
		}
		if sub.SlotID == nil {
			return apperr.Validation("subscription has no slot to replace")
		}
		oldSlot := *sub.SlotID
		target := newSlotID
		if target == nil {
			sl, err := s.d.Slots.PickFreeSlot(ctx, tenantID, sub.ProductID)
			if err != nil {
				return err
			}
			target = &sl.ID
		}
		if *target == oldSlot {
			return apperr.ValidationField("newSlotId", "pick a different slot")
		}
		if _, err := s.d.Subscriptions.ReassignSlot(ctx, tenantID, sub.ID, *target); err != nil {
			return err
		}
		if mark != inventory.SlotFree {
			if _, err := s.d.Slots.UpdateSlot(ctx, tenantID, oldSlot, inventory.UpdateSlotInput{Status: &mark}); err != nil {
				return err
			}
		}
		reason := fmt.Sprintf("replacement ticket #%d", cur.Number)
		if extendDays > 0 {
			if _, err := s.d.Subscriptions.Extend(ctx, tenantID, sub.ID, extendDays, &reason); err != nil {
				return err
			}
		}
		slots, err := s.d.Slots.SlotsByIDs(ctx, tenantID, []uuid.UUID{oldSlot, *target})
		if err != nil {
			return err
		}
		now := s.now()
		cur.Status, cur.ResolvedAt, cur.OldSlotID, cur.NewSlotID = StatusResolved, &now, &oldSlot, target
		if t, err = s.repo.Save(ctx, tx, tenantID, cur); err != nil {
			return err
		}
		note := fmt.Sprintf("Replacement: %s → %s; old slot marked %s", slotName(slots[oldSlot]), slotName(slots[*target]), mark)
		if extendDays > 0 {
			note += fmt.Sprintf("; subscription extended %d days", extendDays)
		}
		if _, err := s.repo.InsertComment(ctx, tx, tenantID, ticketID, actor(actorID), note, true); err != nil {
			return err
		}
		product, err := s.d.Catalog.Get(ctx, tenantID, sub.ProductID)
		if err != nil {
			return err
		}
		if product.DeliveryTemplate != nil && strings.TrimSpace(*product.DeliveryTemplate) != "" {
			return delivery.Enqueue(ctx, s.d.Queue, tenantID, sub.ID, reason)
		}
		return nil
	})
	return t, err
}
