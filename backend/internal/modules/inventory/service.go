package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
	"provider_saas/internal/platform/vault"
)

// Products is what inventory needs from catalog.
type Products interface {
	Get(ctx context.Context, tenantID, id uuid.UUID) (catalog.Product, error)
}

// Vault is the secret store (platform/vault).
type Vault interface {
	Store(ctx context.Context, tenantID uuid.UUID, kind string, plaintext []byte) (uuid.UUID, error)
	Update(ctx context.Context, tenantID, secretID uuid.UUID, plaintext []byte) error
	Reveal(ctx context.Context, tenantID, secretID uuid.UUID) ([]byte, error)
}

// Service is the inventory module API.
type Service interface {
	Suppliers(ctx context.Context, tenantID uuid.UUID) ([]Supplier, error)
	SuppliersByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Supplier, error)
	CreateSupplier(ctx context.Context, tenantID uuid.UUID, in SupplierInput) (Supplier, error)
	UpdateSupplier(ctx context.Context, tenantID, id uuid.UUID, in SupplierInput) (Supplier, error)
	DeleteSupplier(ctx context.Context, tenantID, id uuid.UUID) error
	AccountCountsBySupplier(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error)

	MasterAccounts(ctx context.Context, tenantID uuid.UUID, f MasterFilter, page pagination.Page) ([]MasterAccount, error)
	MasterAccount(ctx context.Context, tenantID, id uuid.UUID) (MasterAccount, error)
	MastersByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]MasterAccount, error)
	CreateMasterAccount(ctx context.Context, tenantID uuid.UUID, in CreateMasterInput) (MasterAccount, error)
	UpdateMasterAccount(ctx context.Context, tenantID, id uuid.UUID, in UpdateMasterInput) (MasterAccount, error)
	SetCredential(ctx context.Context, tenantID, id uuid.UUID, cred Credential) error
	RevealCredential(ctx context.Context, tenantID, id uuid.UUID) (Credential, error)
	RefreshExpiryStatuses(ctx context.Context, tenantID uuid.UUID, now time.Time) (int64, error)

	SlotsByMasters(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Slot, error)
	SlotsByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Slot, error)
	FreeSlots(ctx context.Context, tenantID, productID uuid.UUID) ([]Slot, error)
	FreeCountsByMaster(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error)
	FreeCountsByProduct(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error)
	UpdateSlot(ctx context.Context, tenantID, id uuid.UUID, in UpdateSlotInput) (Slot, error)

	// Slot allocation used by subscriptions (must run inside the caller's tenant tx).
	PickFreeSlot(ctx context.Context, tenantID, productID uuid.UUID) (Slot, error)
	LockFreeSlot(ctx context.Context, tenantID, slotID, productID uuid.UUID) (Slot, error)
	AssignSlot(ctx context.Context, tenantID, slotID, subscriptionID uuid.UUID) error
	// ReleaseSlot frees the slot if it is still held by subscriptionID; reports whether it did.
	ReleaseSlot(ctx context.Context, tenantID, slotID, subscriptionID uuid.UUID) (bool, error)
	// TransferSlot moves a slot held by fromSub to toSub (renewals). Reports whether it moved.
	TransferSlot(ctx context.Context, tenantID, slotID, fromSub, toSub uuid.UUID) (bool, error)
}

type service struct {
	pool     *pgxpool.Pool
	repo     Repository
	products Products
	vault    Vault
	now      func() time.Time
}

func NewService(pool *pgxpool.Pool, products Products, v Vault) Service {
	return &service{pool: pool, products: products, vault: v, now: time.Now}
}

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

// ---- suppliers

func (s *service) Suppliers(ctx context.Context, tenantID uuid.UUID) (out []Supplier, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.Suppliers(ctx, tx, tenantID)
		return err
	})
	return out, err
}

func (s *service) SuppliersByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Supplier, error) {
	out := map[uuid.UUID]Supplier{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ss, err := s.repo.SuppliersByIDs(ctx, tx, tenantID, ids)
		for _, x := range ss {
			out[x.ID] = x
		}
		return err
	})
	return out, err
}

func (s *service) CreateSupplier(ctx context.Context, tenantID uuid.UUID, in SupplierInput) (out Supplier, err error) {
	if err := in.Validate(); err != nil {
		return out, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.InsertSupplier(ctx, tx, tenantID, in)
		return err
	})
	return out, err
}

func (s *service) UpdateSupplier(ctx context.Context, tenantID, id uuid.UUID, in SupplierInput) (out Supplier, err error) {
	if err := in.Validate(); err != nil {
		return out, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.UpdateSupplier(ctx, tx, tenantID, id, in)
		return err
	})
	return out, err
}

func (s *service) DeleteSupplier(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.DeleteSupplier(ctx, tx, tenantID, id)
		if err == nil && !ok {
			return apperr.NotFound("supplier")
		}
		return err
	})
}

func (s *service) AccountCountsBySupplier(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (out map[uuid.UUID]int, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.AccountCountsBySupplier(ctx, tx, tenantID, ids)
		return err
	})
	return out, err
}

// ---- master accounts

func (s *service) MasterAccounts(ctx context.Context, tenantID uuid.UUID, f MasterFilter, page pagination.Page) (out []MasterAccount, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ListMasters(ctx, tx, tenantID, f, page)
		return err
	})
	return out, err
}

func (s *service) MasterAccount(ctx context.Context, tenantID, id uuid.UUID) (m MasterAccount, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		m, err = s.repo.GetMaster(ctx, tx, tenantID, id, false)
		return err
	})
	return m, err
}

func (s *service) MastersByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]MasterAccount, error) {
	out := map[uuid.UUID]MasterAccount{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ms, err := s.repo.MastersByIDs(ctx, tx, tenantID, ids)
		for _, m := range ms {
			out[m.ID] = m
		}
		return err
	})
	return out, err
}

func (s *service) CreateMasterAccount(ctx context.Context, tenantID uuid.UUID, in CreateMasterInput) (m MasterAccount, err error) {
	if err := in.Validate(); err != nil {
		return m, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.products.Get(ctx, tenantID, in.ProductID); err != nil {
			return err
		}
		var secretID *uuid.UUID
		if in.Username != nil {
			id, err := s.storeCredential(ctx, tenantID, Credential{Username: *in.Username, Password: *in.Password, Extra: in.Extra})
			if err != nil {
				return err
			}
			secretID = &id
		}
		cur, err := s.repo.TenantCurrency(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		status := ExpiryStatus(MasterActive, in.ExpiresAt, s.now())
		if m, err = s.repo.InsertMaster(ctx, tx, tenantID, in, secretID, cur, status); err != nil {
			return err
		}
		return s.repo.InsertSlots(ctx, tx, tenantID, m.ID, 0, in.TotalSlots)
	})
	return m, err
}

func (s *service) storeCredential(ctx context.Context, tenantID uuid.UUID, c Credential) (uuid.UUID, error) {
	if s.vault == nil {
		return uuid.Nil, errors.New("inventory: vault not configured")
	}
	b, err := json.Marshal(c)
	if err != nil {
		return uuid.Nil, err
	}
	return s.vault.Store(ctx, tenantID, vault.KindAccountCredential, b)
}

func (s *service) UpdateMasterAccount(ctx context.Context, tenantID, id uuid.UUID, in UpdateMasterInput) (m MasterAccount, err error) {
	if err := in.Validate(); err != nil {
		return m, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.GetMaster(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if in.TotalSlots != nil && *in.TotalSlots != cur.TotalSlots {
			maxNo, err := s.repo.MaxSlotNo(ctx, tx, tenantID, id)
			if err != nil {
				return err
			}
			if *in.TotalSlots > maxNo {
				if err := s.repo.InsertSlots(ctx, tx, tenantID, id, maxNo, *in.TotalSlots); err != nil {
					return err
				}
			} else {
				blocked, err := s.repo.DeleteFreeSlotsAbove(ctx, tx, tenantID, id, *in.TotalSlots)
				if err != nil {
					return err
				}
				if blocked > 0 {
					return apperr.Conflict("cannot reduce slots: some of the removed slots are assigned")
				}
			}
		}
		// A new expiry re-derives active/expiring/expired unless the caller set a status.
		if in.ExpiresAt != nil && in.Status == nil {
			st := ExpiryStatus(cur.Status, in.ExpiresAt, s.now())
			in.Status = &st
		}
		m, err = s.repo.UpdateMaster(ctx, tx, tenantID, id, in)
		return err
	})
	return m, err
}

func (s *service) SetCredential(ctx context.Context, tenantID, id uuid.UUID, c Credential) error {
	if c.Username == "" || c.Password == "" {
		return apperr.Validation("username and password are required")
	}
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		m, err := s.repo.GetMaster(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		hint := MaskLogin(c.Username)
		if m.CredentialSecretID != nil {
			b, err := json.Marshal(c)
			if err != nil {
				return err
			}
			if err := s.vault.Update(ctx, tenantID, *m.CredentialSecretID, b); err != nil {
				return err
			}
			return s.repo.SetMasterSecret(ctx, tx, tenantID, id, *m.CredentialSecretID, &hint)
		}
		secretID, err := s.storeCredential(ctx, tenantID, c)
		if err != nil {
			return err
		}
		return s.repo.SetMasterSecret(ctx, tx, tenantID, id, secretID, &hint)
	})
}

func (s *service) RevealCredential(ctx context.Context, tenantID, id uuid.UUID) (c Credential, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		m, err := s.repo.GetMaster(ctx, tx, tenantID, id, false)
		if err != nil {
			return err
		}
		if m.CredentialSecretID == nil {
			return apperr.NotFound("credential")
		}
		b, err := s.vault.Reveal(ctx, tenantID, *m.CredentialSecretID) // writes audit_logs
		if err != nil {
			return err
		}
		return json.Unmarshal(b, &c)
	})
	return c, err
}

func (s *service) RefreshExpiryStatuses(ctx context.Context, tenantID uuid.UUID, now time.Time) (n int64, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		n, err = s.repo.RefreshMasterStatuses(ctx, tx, tenantID, now, MasterExpiringWindow)
		return err
	})
	return n, err
}

// ---- slots

func (s *service) SlotsByMasters(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Slot, error) {
	out := map[uuid.UUID][]Slot{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ss, err := s.repo.SlotsByMasters(ctx, tx, tenantID, ids)
		for _, x := range ss {
			out[x.MasterAccountID] = append(out[x.MasterAccountID], x)
		}
		return err
	})
	return out, err
}

func (s *service) SlotsByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Slot, error) {
	out := map[uuid.UUID]Slot{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ss, err := s.repo.SlotsByIDs(ctx, tx, tenantID, ids)
		for _, x := range ss {
			out[x.ID] = x
		}
		return err
	})
	return out, err
}

func (s *service) FreeSlots(ctx context.Context, tenantID, productID uuid.UUID) (out []Slot, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.FreeSlots(ctx, tx, tenantID, productID)
		return err
	})
	return out, err
}

func (s *service) FreeCountsByMaster(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (out map[uuid.UUID]int, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.FreeCountsByMaster(ctx, tx, tenantID, ids)
		return err
	})
	return out, err
}

func (s *service) FreeCountsByProduct(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (out map[uuid.UUID]int, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.FreeCountsByProduct(ctx, tx, tenantID, ids)
		return err
	})
	return out, err
}

func (s *service) UpdateSlot(ctx context.Context, tenantID, id uuid.UUID, in UpdateSlotInput) (out Slot, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, _, _, err := s.repo.GetSlot(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if in.Status != nil {
			if err := ValidateSlotStatusChange(cur.Status, *in.Status); err != nil {
				return err
			}
		}
		out, err = s.repo.UpdateSlot(ctx, tx, tenantID, id, in)
		return err
	})
	return out, err
}

func (s *service) PickFreeSlot(ctx context.Context, tenantID, productID uuid.UUID) (out Slot, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.PickFreeSlot(ctx, tx, tenantID, productID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Conflict("no free slot")
		}
		return err
	})
	return out, err
}

func (s *service) LockFreeSlot(ctx context.Context, tenantID, slotID, productID uuid.UUID) (out Slot, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		sl, pid, masterStatus, err := s.repo.GetSlot(ctx, tx, tenantID, slotID, true)
		if err != nil {
			return err
		}
		switch {
		case pid != productID:
			return apperr.Validation("slot belongs to another product")
		case sl.Status != SlotFree:
			return apperr.Conflict("slot is not free")
		case masterStatus != MasterActive && masterStatus != MasterExpiring:
			return apperr.Conflict("master account is " + masterStatus)
		}
		out = sl
		return nil
	})
	return out, err
}

func (s *service) AssignSlot(ctx context.Context, tenantID, slotID, subscriptionID uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.SetSlotAssignment(ctx, tx, tenantID, slotID, &subscriptionID, SlotAssigned)
	})
}

func (s *service) ReleaseSlot(ctx context.Context, tenantID, slotID, subscriptionID uuid.UUID) (released bool, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		sl, _, _, err := s.repo.GetSlot(ctx, tx, tenantID, slotID, true)
		if err != nil {
			return err
		}
		if sl.SubscriptionID == nil || *sl.SubscriptionID != subscriptionID {
			return nil
		}
		status := sl.Status
		if status == SlotAssigned || status == SlotReserved {
			status = SlotFree
		}
		released = true
		return s.repo.SetSlotAssignment(ctx, tx, tenantID, slotID, nil, status)
	})
	return released, err
}

func (s *service) TransferSlot(ctx context.Context, tenantID, slotID, fromSub, toSub uuid.UUID) (moved bool, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		sl, _, masterStatus, err := s.repo.GetSlot(ctx, tx, tenantID, slotID, true)
		if err != nil {
			return err
		}
		if sl.SubscriptionID == nil || *sl.SubscriptionID != fromSub || sl.Status != SlotAssigned ||
			(masterStatus != MasterActive && masterStatus != MasterExpiring) {
			return nil
		}
		moved = true
		return s.repo.SetSlotAssignment(ctx, tx, tenantID, slotID, &toSub, SlotAssigned)
	})
	return moved, err
}
