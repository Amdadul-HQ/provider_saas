package customer

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
)

// Service is the customer module API. All methods are tenant-scoped.
type Service interface {
	List(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) (items []Customer, hasNext bool, total int, err error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (Customer, error)
	GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Customer, error)
	Create(ctx context.Context, tenantID, actorID uuid.UUID, in CreateInput) (Customer, error)
	Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateInput) (Customer, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
	AddNote(ctx context.Context, tenantID, customerID, authorID uuid.UUID, body string) (Note, error)
	Notes(ctx context.Context, tenantID, customerID uuid.UUID) ([]Note, error)
	Merge(ctx context.Context, tenantID, keepID, mergeID uuid.UUID) (Customer, error)
	LinkIdentity(ctx context.Context, tenantID, customerID, identityID uuid.UUID) (Customer, error)
	AddIdentity(ctx context.Context, tenantID, customerID uuid.UUID, platform, externalID string, handle *string, socialAccountID *uuid.UUID) (Identity, error)
	IdentitiesByCustomers(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Identity, error)
	// FindIdentity looks up a channel identity by (platform, external id); NotFound when unknown.
	FindIdentity(ctx context.Context, tenantID uuid.UUID, platform, externalID string) (Identity, error)
	// FindByPhone returns the oldest live customer with this (normalised) phone; NotFound when none.
	FindByPhone(ctx context.Context, tenantID uuid.UUID, phone string) (Customer, error)
	// AdjustBalance changes the cached due balance / lifetime value (called by billing inside its tx).
	AdjustBalance(ctx context.Context, tenantID, customerID uuid.UUID, dueDelta, ltvDelta int64) error
	Touch(ctx context.Context, tenantID, customerID uuid.UUID, at time.Time) error
}

type service struct {
	pool *pgxpool.Pool
	repo Repository
}

func NewService(pool *pgxpool.Pool) Service { return &service{pool: pool} }

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func (s *service) List(ctx context.Context, tenantID uuid.UUID, f Filter, page pagination.Page) (items []Customer, hasNext bool, total int, err error) {
	if f.Status != nil {
		v := strings.ToLower(*f.Status)
		f.Status = &v
	}
	if f.Source != nil {
		v := strings.ToLower(*f.Source)
		f.Source = &v
	}
	f.Tags = NormalizeTags(f.Tags)
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.repo.List(ctx, tx, tenantID, f, page)
		if err != nil {
			return err
		}
		items, hasNext = pagination.Trim(rows, page.First)
		total, err = s.repo.Count(ctx, tx, tenantID, f)
		return err
	})
	return items, hasNext, total, err
}

func (s *service) Get(ctx context.Context, tenantID, id uuid.UUID) (c Customer, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		c, err = s.repo.Get(ctx, tx, tenantID, id, false)
		return err
	})
	return c, err
}

func (s *service) GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Customer, error) {
	out := make(map[uuid.UUID]Customer, len(ids))
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cs, err := s.repo.GetByIDs(ctx, tx, tenantID, ids)
		for _, c := range cs {
			out[c.ID] = c
		}
		return err
	})
	return out, err
}

func (s *service) Create(ctx context.Context, tenantID, actorID uuid.UUID, in CreateInput) (c Customer, err error) {
	if err := in.Validate(); err != nil {
		return c, err
	}
	var owner *uuid.UUID
	if actorID != uuid.Nil {
		owner = &actorID
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		c, err = s.repo.Insert(ctx, tx, tenantID, owner, in)
		return err
	})
	return c, err
}

func (s *service) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateInput) (c Customer, err error) {
	if err := in.Validate(); err != nil {
		return c, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.Get(ctx, tx, tenantID, id, true); err != nil { // visibility (reseller scope)
			return err
		}
		c, err = s.repo.Update(ctx, tx, tenantID, id, in)
		return err
	})
	return c, err
}

func (s *service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.SoftDelete(ctx, tx, tenantID, id)
		if err == nil && !ok {
			return apperr.NotFound("customer")
		}
		return err
	})
}

func (s *service) AddNote(ctx context.Context, tenantID, customerID, authorID uuid.UUID, body string) (n Note, err error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return n, apperr.ValidationField("body", "required")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.Get(ctx, tx, tenantID, customerID, false); err != nil {
			return err
		}
		n, err = s.repo.InsertNote(ctx, tx, tenantID, customerID, &authorID, body)
		return err
	})
	return n, err
}

func (s *service) Notes(ctx context.Context, tenantID, customerID uuid.UUID) (out []Note, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.Get(ctx, tx, tenantID, customerID, false); err != nil {
			if errors.Is(err, apperr.ErrNotFound) {
				out = []Note{}
				return nil
			}
			return err
		}
		out, err = s.repo.Notes(ctx, tx, tenantID, customerID)
		return err
	})
	return out, err
}

func (s *service) Merge(ctx context.Context, tenantID, keepID, mergeID uuid.UUID) (c Customer, err error) {
	if keepID == mergeID {
		return c, apperr.Validation("cannot merge a customer into itself")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		// Lock both rows in a stable order to avoid deadlocks.
		first, second := keepID, mergeID
		if second.String() < first.String() {
			first, second = second, first
		}
		if _, err := s.repo.Get(ctx, tx, tenantID, first, true); err != nil {
			return err
		}
		if _, err := s.repo.Get(ctx, tx, tenantID, second, true); err != nil {
			return err
		}
		if err := s.repo.Merge(ctx, tx, tenantID, keepID, mergeID); err != nil {
			return err
		}
		c, err = s.repo.Get(ctx, tx, tenantID, keepID, false)
		return err
	})
	return c, err
}

func (s *service) LinkIdentity(ctx context.Context, tenantID, customerID, identityID uuid.UUID) (c Customer, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.Get(ctx, tx, tenantID, customerID, true); err != nil {
			return err
		}
		if _, err := s.repo.GetIdentity(ctx, tx, tenantID, identityID); err != nil {
			return err
		}
		if err := s.repo.MoveIdentity(ctx, tx, tenantID, identityID, customerID); err != nil {
			return err
		}
		c, err = s.repo.Get(ctx, tx, tenantID, customerID, false)
		return err
	})
	return c, err
}

var validPlatforms = map[string]bool{"facebook": true, "instagram": true, "whatsapp": true, "telegram": true, "sms": true, "email": true}

func (s *service) AddIdentity(ctx context.Context, tenantID, customerID uuid.UUID, platform, externalID string, handle *string, socialAccountID *uuid.UUID) (i Identity, err error) {
	platform = strings.ToLower(platform)
	if !validPlatforms[platform] {
		return i, apperr.ValidationField("platform", "invalid platform")
	}
	if strings.TrimSpace(externalID) == "" {
		return i, apperr.ValidationField("externalId", "required")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		i, err = s.repo.InsertIdentity(ctx, tx, tenantID, customerID, platform, externalID, handle, socialAccountID)
		return err
	})
	return i, err
}

func (s *service) IdentitiesByCustomers(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Identity, error) {
	out := map[uuid.UUID][]Identity{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		is, err := s.repo.IdentitiesByCustomers(ctx, tx, tenantID, ids)
		for _, i := range is {
			out[i.CustomerID] = append(out[i.CustomerID], i)
		}
		return err
	})
	return out, err
}

func (s *service) AdjustBalance(ctx context.Context, tenantID, customerID uuid.UUID, dueDelta, ltvDelta int64) error {
	if dueDelta == 0 && ltvDelta == 0 {
		return nil
	}
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.AdjustBalance(ctx, tx, tenantID, customerID, dueDelta, ltvDelta)
	})
}

func (s *service) Touch(ctx context.Context, tenantID, customerID uuid.UUID, at time.Time) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.Touch(ctx, tx, tenantID, customerID, at)
	})
}

func (s *service) FindIdentity(ctx context.Context, tenantID uuid.UUID, platform, externalID string) (i Identity, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		i, err = s.repo.FindIdentity(ctx, tx, tenantID, strings.ToLower(platform), externalID)
		return err
	})
	return i, err
}

func (s *service) FindByPhone(ctx context.Context, tenantID uuid.UUID, phone string) (c Customer, err error) {
	p := normPhone(&phone)
	if p == nil {
		return c, apperr.ValidationField("phone", "required")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		c, err = s.repo.FindByPhone(ctx, tx, tenantID, *p)
		return err
	})
	return c, err
}
