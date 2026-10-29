package ticket

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

const (
	KindReplacement = "replacement"

	StatusOpen            = "open"
	StatusInProgress      = "in_progress"
	StatusWaitingCustomer = "waiting_customer"
	StatusResolved        = "resolved"
	StatusClosed          = "closed"
)

var (
	validKind     = map[string]bool{"replacement": true, "complaint": true, "question": true, "refund": true, "other": true}
	validPriority = map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}
	validStatus   = map[string]bool{StatusOpen: true, StatusInProgress: true, StatusWaitingCustomer: true, StatusResolved: true, StatusClosed: true}
	// What the old slot of a replacement may become.
	validOldSlot = map[string]bool{"banned": true, "retired": true, "free": true}
)

type Ticket struct {
	ID             uuid.UUID
	Number         int
	CustomerID     uuid.UUID
	SubscriptionID *uuid.UUID
	ConversationID *uuid.UUID
	Kind           string
	Priority       string
	Status         string
	Title          string
	Description    *string
	Reason         *string
	OldSlotID      *uuid.UUID
	NewSlotID      *uuid.UUID
	AssignedTo     *uuid.UUID
	ResolvedAt     *time.Time
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsDone reports whether the ticket is resolved or closed.
func (t Ticket) IsDone() bool { return t.Status == StatusResolved || t.Status == StatusClosed }

type Comment struct {
	ID         uuid.UUID
	TicketID   uuid.UUID
	AuthorID   *uuid.UUID
	Body       string
	IsInternal bool
	CreatedAt  time.Time
}

type Stats struct {
	Open                  int
	InProgress            int
	ResolvedThisMonth     int
	ReplacementsThisMonth int
}

type Filter struct {
	Status     *string
	Kind       *string
	Priority   *string
	CustomerID *uuid.UUID
	AssignedTo *uuid.UUID
	Search     *string
}

type CreateInput struct {
	CustomerID     uuid.UUID
	SubscriptionID *uuid.UUID
	ConversationID *uuid.UUID
	Kind           string
	Priority       string
	Title          string
	Description    *string
	Reason         *string
}

func (in *CreateInput) Validate() error {
	in.Kind, in.Priority, in.Title = strings.ToLower(in.Kind), strings.ToLower(in.Priority), strings.TrimSpace(in.Title)
	if in.Priority == "" {
		in.Priority = "normal"
	}
	switch {
	case !validKind[in.Kind]:
		return apperr.ValidationField("kind", "invalid kind")
	case !validPriority[in.Priority]:
		return apperr.ValidationField("priority", "invalid priority")
	case in.Title == "" || len([]rune(in.Title)) > 200:
		return apperr.ValidationField("title", "1..200 characters")
	case in.Kind == KindReplacement && in.SubscriptionID == nil:
		return apperr.ValidationField("subscriptionId", "a replacement ticket needs the subscription")
	}
	return nil
}

type UpdateInput struct {
	Priority    *string
	Status      *string
	Title       *string
	Description *string
	Reason      *string
	AssignedTo  *uuid.UUID
}

func (in *UpdateInput) Validate() error {
	if in.Priority != nil {
		v := strings.ToLower(*in.Priority)
		if !validPriority[v] {
			return apperr.ValidationField("priority", "invalid priority")
		}
		in.Priority = &v
	}
	if in.Status != nil {
		v := strings.ToLower(*in.Status)
		if !validStatus[v] {
			return apperr.ValidationField("status", "invalid status")
		}
		in.Status = &v
	}
	if in.Title != nil {
		v := strings.TrimSpace(*in.Title)
		if v == "" || len([]rune(v)) > 200 {
			return apperr.ValidationField("title", "1..200 characters")
		}
		in.Title = &v
	}
	return nil
}

// ValidOldSlotStatus normalises the markOldSlot argument of resolveReplacement.
func ValidOldSlotStatus(s string) (string, error) {
	s = strings.ToLower(s)
	if s == "" {
		s = "banned"
	}
	if !validOldSlot[s] {
		return "", apperr.ValidationField("markOldSlot", "must be BANNED, RETIRED or FREE")
	}
	return s, nil
}
