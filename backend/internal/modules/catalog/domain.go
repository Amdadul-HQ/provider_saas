package catalog

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

// Delivery types (products.delivery_type).
const (
	DeliverySharedSlot     = "shared_slot"
	DeliveryPrivateAccount = "private_account"
	DeliveryLicenseKey     = "license_key"
	DeliveryInviteLink     = "invite_link"
	DeliveryManual         = "manual"
)

var validDelivery = map[string]bool{DeliverySharedSlot: true, DeliveryPrivateAccount: true, DeliveryLicenseKey: true, DeliveryInviteLink: true, DeliveryManual: true}

type Product struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	Name             string
	Category         *string
	Description      *string
	ImageURL         *string
	DeliveryType     string
	WarrantyDays     int
	DeliveryTemplate *string
	IsActive         bool
	SortOrder        int
	CreatedAt        time.Time
}

type Plan struct {
	ID           uuid.UUID
	ProductID    uuid.UUID
	Name         string
	DurationDays int
	Price        int64
	Cost         int64
	Currency     string
	IsActive     bool
	SortOrder    int
}

type PlanInput struct {
	Name         string
	DurationDays int
	Price        int64
	Cost         int64
	IsActive     bool
	SortOrder    int
}

type CreateInput struct {
	Name             string
	Category         *string
	Description      *string
	ImageURL         *string
	DeliveryType     string
	WarrantyDays     int
	DeliveryTemplate *string
	Plans            []PlanInput
}

type UpdateInput struct {
	Name             *string
	Category         *string
	Description      *string
	ImageURL         *string
	DeliveryType     *string
	WarrantyDays     *int
	DeliveryTemplate *string
	IsActive         *bool
	SortOrder        *int
}

type Filter struct {
	Search   *string
	Category *string
	IsActive *bool
}

func (p *PlanInput) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "":
		return apperr.ValidationField("plan.name", "required")
	case p.DurationDays <= 0 || p.DurationDays > 3650:
		return apperr.ValidationField("plan.durationDays", "must be 1..3650")
	case p.Price < 0:
		return apperr.ValidationField("plan.price", "must be >= 0")
	case p.Cost < 0:
		return apperr.ValidationField("plan.cost", "must be >= 0")
	}
	return nil
}

func (in *CreateInput) Validate() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return apperr.ValidationField("name", "required")
	}
	if in.DeliveryType == "" {
		in.DeliveryType = DeliverySharedSlot
	}
	in.DeliveryType = strings.ToLower(in.DeliveryType)
	if !validDelivery[in.DeliveryType] {
		return apperr.ValidationField("deliveryType", "invalid delivery type")
	}
	if in.WarrantyDays < 0 {
		return apperr.ValidationField("warrantyDays", "must be >= 0")
	}
	if in.Category != nil {
		c := strings.ToLower(strings.TrimSpace(*in.Category))
		in.Category = &c
	}
	for i := range in.Plans {
		if err := in.Plans[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (in *UpdateInput) Validate() error {
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			return apperr.ValidationField("name", "must not be empty")
		}
		in.Name = &n
	}
	if in.DeliveryType != nil {
		d := strings.ToLower(*in.DeliveryType)
		if !validDelivery[d] {
			return apperr.ValidationField("deliveryType", "invalid delivery type")
		}
		in.DeliveryType = &d
	}
	if in.WarrantyDays != nil && *in.WarrantyDays < 0 {
		return apperr.ValidationField("warrantyDays", "must be >= 0")
	}
	return nil
}
