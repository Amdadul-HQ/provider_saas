package customer

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/phone"
)

// Customer is a buyer of a provider (tenant), reached on one or more channels.
type Customer struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	Name             string
	Phone            *string
	Email            *string
	Whatsapp         *string
	Telegram         *string
	Address          *string
	Tags             []string
	Notes            *string
	Source           string
	PreferredChannel *string
	DueBalance       int64 // minor units; positive = customer owes (baki)
	LifetimeValue    int64
	Status           string
	OwnerUserID      *uuid.UUID
	LastContactAt    *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Identity struct {
	ID              uuid.UUID
	CustomerID      uuid.UUID
	Platform        string
	ExternalID      string
	Handle          *string
	AvatarURL       *string
	SocialAccountID *uuid.UUID
	CreatedAt       time.Time
}

type Note struct {
	ID         uuid.UUID
	CustomerID uuid.UUID
	AuthorID   *uuid.UUID
	Body       string
	CreatedAt  time.Time
}

type Filter struct {
	Search             *string
	Status             *string
	Source             *string
	Tags               []string
	HasDue             *bool
	ExpiringWithinDays *int
}

type CreateInput struct {
	Name             string
	Phone            *string
	Email            *string
	Whatsapp         *string
	Telegram         *string
	Address          *string
	Tags             []string
	Notes            *string
	Source           string
	PreferredChannel *string
}

type UpdateInput struct {
	Name             *string
	Phone            *string
	Email            *string
	Whatsapp         *string
	Telegram         *string
	Address          *string
	Tags             []string // nil = unchanged
	Notes            *string
	Status           *string
	PreferredChannel *string
}

const (
	StatusLead     = "lead"
	StatusActive   = "active"
	StatusInactive = "inactive"
	StatusBlocked  = "blocked"
)

var (
	validStatus   = map[string]bool{StatusLead: true, StatusActive: true, StatusInactive: true, StatusBlocked: true}
	validSource   = map[string]bool{"manual": true, "facebook": true, "instagram": true, "whatsapp": true, "telegram": true, "storefront": true, "import": true}
	validChannels = map[string]bool{"facebook": true, "instagram": true, "whatsapp": true, "telegram": true, "sms": true, "email": true}
)

func trimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return &v
}

func normPhone(p *string) *string {
	p = trimPtr(p)
	if p == nil {
		return nil
	}
	v, _ := phone.Normalize(*p) // keep foreign numbers as typed
	return &v
}

// NormalizeTags lower-cases, trims and de-duplicates tags.
func NormalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// Validate normalises and validates a create input.
func (in *CreateInput) Validate() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return apperr.ValidationField("name", "required")
	}
	if in.Source == "" {
		in.Source = "manual"
	}
	in.Source = strings.ToLower(in.Source)
	if !validSource[in.Source] {
		return apperr.ValidationField("source", "invalid source")
	}
	if in.PreferredChannel != nil {
		c := strings.ToLower(*in.PreferredChannel)
		if !validChannels[c] {
			return apperr.ValidationField("preferredChannel", "invalid channel")
		}
		in.PreferredChannel = &c
	}
	in.Phone, in.Whatsapp = normPhone(in.Phone), normPhone(in.Whatsapp)
	in.Email, in.Telegram, in.Address, in.Notes = trimPtr(in.Email), trimPtr(in.Telegram), trimPtr(in.Address), trimPtr(in.Notes)
	if in.Email != nil && !strings.Contains(*in.Email, "@") {
		return apperr.ValidationField("email", "invalid email")
	}
	in.Tags = NormalizeTags(in.Tags)
	return nil
}

// Validate normalises and validates an update input.
func (in *UpdateInput) Validate() error {
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			return apperr.ValidationField("name", "must not be empty")
		}
		in.Name = &n
	}
	if in.Status != nil {
		s := strings.ToLower(*in.Status)
		if !validStatus[s] {
			return apperr.ValidationField("status", "invalid status")
		}
		in.Status = &s
	}
	if in.PreferredChannel != nil {
		c := strings.ToLower(*in.PreferredChannel)
		if !validChannels[c] {
			return apperr.ValidationField("preferredChannel", "invalid channel")
		}
		in.PreferredChannel = &c
	}
	if in.Phone != nil {
		in.Phone = normPhone(in.Phone)
		if in.Phone == nil {
			e := ""
			in.Phone = &e
		}
	}
	if in.Whatsapp != nil {
		in.Whatsapp = normPhone(in.Whatsapp)
		if in.Whatsapp == nil {
			e := ""
			in.Whatsapp = &e
		}
	}
	if in.Tags != nil {
		in.Tags = NormalizeTags(in.Tags)
	}
	return nil
}
