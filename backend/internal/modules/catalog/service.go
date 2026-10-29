package catalog

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
)

// Service is the catalog module API.
type Service interface {
	List(ctx context.Context, tenantID uuid.UUID, f Filter, limit int) ([]Product, error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (Product, error)
	GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Product, error)
	Create(ctx context.Context, tenantID uuid.UUID, in CreateInput) (Product, error)
	Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateInput) (Product, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
	UpsertPlan(ctx context.Context, tenantID, productID uuid.UUID, id *uuid.UUID, in PlanInput) (Plan, error)
	DeletePlan(ctx context.Context, tenantID, id uuid.UUID) error
	GetPlan(ctx context.Context, tenantID, id uuid.UUID) (Plan, error)
	PlansByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Plan, error)
	PlansByProducts(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Plan, error)
}

type service struct {
	pool *pgxpool.Pool
	repo Repository
}

func NewService(pool *pgxpool.Pool) Service { return &service{pool: pool} }

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func (s *service) List(ctx context.Context, tenantID uuid.UUID, f Filter, limit int) (out []Product, err error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.List(ctx, tx, tenantID, f, limit)
		return err
	})
	return out, err
}

func (s *service) Get(ctx context.Context, tenantID, id uuid.UUID) (p Product, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		p, err = s.repo.Get(ctx, tx, tenantID, id)
		return err
	})
	return p, err
}

func (s *service) GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Product, error) {
	out := make(map[uuid.UUID]Product, len(ids))
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ps, err := s.repo.GetByIDs(ctx, tx, tenantID, ids)
		for _, p := range ps {
			out[p.ID] = p
		}
		return err
	})
	return out, err
}

func (s *service) Create(ctx context.Context, tenantID uuid.UUID, in CreateInput) (p Product, err error) {
	if err := in.Validate(); err != nil {
		return p, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if p, err = s.repo.Insert(ctx, tx, tenantID, in); err != nil {
			return err
		}
		cur, err := s.repo.TenantCurrency(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		for _, pl := range in.Plans {
			if _, err := s.repo.InsertPlan(ctx, tx, tenantID, p.ID, cur, pl); err != nil {
				return err
			}
		}
		return nil
	})
	return p, err
}

func (s *service) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateInput) (p Product, err error) {
	if err := in.Validate(); err != nil {
		return p, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		p, err = s.repo.Update(ctx, tx, tenantID, id, in)
		return err
	})
	return p, err
}

func (s *service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.SoftDelete(ctx, tx, tenantID, id)
		if err == nil && !ok {
			return apperr.NotFound("product")
		}
		return err
	})
}

func (s *service) UpsertPlan(ctx context.Context, tenantID, productID uuid.UUID, id *uuid.UUID, in PlanInput) (p Plan, err error) {
	if err := in.Validate(); err != nil {
		return p, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.Get(ctx, tx, tenantID, productID); err != nil {
			return err
		}
		if id != nil {
			p, err = s.repo.UpdatePlan(ctx, tx, tenantID, productID, *id, in)
			return err
		}
		cur, err := s.repo.TenantCurrency(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		p, err = s.repo.InsertPlan(ctx, tx, tenantID, productID, cur, in)
		return err
	})
	return p, err
}

func (s *service) DeletePlan(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.DeletePlan(ctx, tx, tenantID, id)
		if err == nil && !ok {
			return apperr.NotFound("plan")
		}
		return err
	})
}

func (s *service) GetPlan(ctx context.Context, tenantID, id uuid.UUID) (p Plan, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		p, err = s.repo.GetPlan(ctx, tx, tenantID, id)
		return err
	})
	return p, err
}

func (s *service) PlansByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Plan, error) {
	out := make(map[uuid.UUID]Plan, len(ids))
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ps, err := s.repo.PlansByIDs(ctx, tx, tenantID, ids)
		for _, p := range ps {
			out[p.ID] = p
		}
		return err
	})
	return out, err
}

func (s *service) PlansByProducts(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Plan, error) {
	out := map[uuid.UUID][]Plan{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ps, err := s.repo.PlansByProducts(ctx, tx, tenantID, ids)
		for _, p := range ps {
			out[p.ProductID] = append(out[p.ProductID], p)
		}
		return err
	})
	return out, err
}
