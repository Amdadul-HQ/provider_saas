package inventory

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

// Master account statuses.
const (
	MasterActive   = "active"
	MasterExpiring = "expiring"
	MasterExpired  = "expired"
	MasterBanned   = "banned"
	MasterReplaced = "replaced"
	MasterRetired  = "retired"
)

// Slot statuses.
const (
	SlotFree     = "free"
	SlotReserved = "reserved"
	SlotAssigned = "assigned"
	SlotBanned   = "banned"
	SlotRetired  = "retired"
)

// MasterExpiringWindow: a master account within this window of expiry is "expiring".
const MasterExpiringWindow = 7 * 24 * time.Hour

var (
	validMasterStatus = map[string]bool{MasterActive: true, MasterExpiring: true, MasterExpired: true, MasterBanned: true, MasterReplaced: true, MasterRetired: true}
	validSlotStatus   = map[string]bool{SlotFree: true, SlotReserved: true, SlotAssigned: true, SlotBanned: true, SlotRetired: true}
)

type Supplier struct {
	ID        uuid.UUID
	Name      string
	Contact   *string
	Notes     *string
	Rating    *int
	CreatedAt time.Time
}

type MasterAccount struct {
	ID                 uuid.UUID
	ProductID          uuid.UUID
	SupplierID         *uuid.UUID
	Label              string
	LoginHint          *string
	CredentialSecretID *uuid.UUID
	TotalSlots         int
	PurchaseCost       int64
	Currency           string
	PurchasedAt        *time.Time
	ExpiresAt          *time.Time
	Status             string
	Notes              *string
	CreatedAt          time.Time
}

type Slot struct {
	ID              uuid.UUID
	MasterAccountID uuid.UUID
	SlotNo          int
	Label           *string
	Pin             *string
	Status          string
	SubscriptionID  *uuid.UUID
}

// Credential is the plaintext stored (encrypted) in the vault for a master account.
type Credential struct {
	Username string         `json:"username"`
	Password string         `json:"password"`
	Extra    map[string]any `json:"extra,omitempty"`
}

type SupplierInput struct {
	Name    string
	Contact *string
	Notes   *string
	Rating  *int
}

type MasterFilter struct {
	ProductID          *uuid.UUID
	SupplierID         *uuid.UUID
	Status             *string
	ExpiringWithinDays *int
}

type CreateMasterInput struct {
	ProductID    uuid.UUID
	SupplierID   *uuid.UUID
	Label        string
	LoginHint    *string
	Username     *string
	Password     *string
	Extra        map[string]any
	TotalSlots   int
	PurchaseCost int64
	PurchasedAt  *time.Time
	ExpiresAt    *time.Time
	Notes        *string
}

type UpdateMasterInput struct {
	SupplierID   *uuid.UUID
	Label        *string
	LoginHint    *string
	TotalSlots   *int
	PurchaseCost *int64
	PurchasedAt  *time.Time
	ExpiresAt    *time.Time
	Status       *string
	Notes        *string
}

type UpdateSlotInput struct {
	Label  *string
	Pin    *string
	Status *string
}

const MaxSlots = 50

func (in *SupplierInput) Validate() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return apperr.ValidationField("name", "required")
	}
	if in.Rating != nil && (*in.Rating < 1 || *in.Rating > 5) {
		return apperr.ValidationField("rating", "must be 1..5")
	}
	return nil
}

func (in *CreateMasterInput) Validate() error {
	in.Label = strings.TrimSpace(in.Label)
	switch {
	case in.Label == "":
		return apperr.ValidationField("label", "required")
	case in.TotalSlots < 1 || in.TotalSlots > MaxSlots:
		return apperr.ValidationField("totalSlots", "must be 1..50")
	case in.PurchaseCost < 0:
		return apperr.ValidationField("purchaseCost", "must be >= 0")
	case (in.Username == nil) != (in.Password == nil):
		return apperr.Validation("username and password must be given together")
	}
	if in.LoginHint == nil && in.Username != nil {
		h := MaskLogin(*in.Username)
		in.LoginHint = &h
	}
	return nil
}

func (in *UpdateMasterInput) Validate() error {
	if in.Label != nil && strings.TrimSpace(*in.Label) == "" {
		return apperr.ValidationField("label", "must not be empty")
	}
	if in.TotalSlots != nil && (*in.TotalSlots < 1 || *in.TotalSlots > MaxSlots) {
		return apperr.ValidationField("totalSlots", "must be 1..50")
	}
	if in.PurchaseCost != nil && *in.PurchaseCost < 0 {
		return apperr.ValidationField("purchaseCost", "must be >= 0")
	}
	if in.Status != nil {
		s := strings.ToLower(*in.Status)
		if !validMasterStatus[s] {
			return apperr.ValidationField("status", "invalid status")
		}
		in.Status = &s
	}
	return nil
}

// MaskLogin turns "rahim.netflix@gmail.com" into "ra***@gmail.com" (never store plaintext).
func MaskLogin(login string) string {
	login = strings.TrimSpace(login)
	local, domain, ok := strings.Cut(login, "@")
	keep := func(s string) string {
		r := []rune(s)
		if len(r) <= 2 {
			return string(r[:min(1, len(r))]) + "***"
		}
		return string(r[:2]) + "***"
	}
	if !ok {
		r := []rune(login)
		if len(r) > 4 {
			return keep(login) + string(r[len(r)-2:])
		}
		return keep(login)
	}
	return keep(local) + "@" + domain
}

// ExpiryStatus derives the status of an active/expiring master account from its expiry.
// Terminal statuses (banned/replaced/retired) are kept.
func ExpiryStatus(current string, expiresAt *time.Time, now time.Time) string {
	if current != MasterActive && current != MasterExpiring && current != MasterExpired {
		return current
	}
	if expiresAt == nil {
		return MasterActive
	}
	switch {
	case !expiresAt.After(now):
		return MasterExpired
	case expiresAt.Sub(now) <= MasterExpiringWindow:
		return MasterExpiring
	default:
		return MasterActive
	}
}

// ValidateSlotStatusChange enforces that "assigned" is only set/cleared by subscriptions.
func ValidateSlotStatusChange(from, to string) error {
	if !validSlotStatus[to] {
		return apperr.ValidationField("status", "invalid slot status")
	}
	if from == to {
		return nil
	}
	if to == SlotAssigned {
		return apperr.Validation("slots are assigned by creating a subscription")
	}
	if from == SlotAssigned && to != SlotBanned && to != SlotRetired {
		return apperr.Validation("an assigned slot can only be marked banned or retired; cancel or reassign the subscription instead")
	}
	return nil
}
