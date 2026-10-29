package model

// Hand-written models for Sprint 2 growth features (bound in gqlgen.yml). Like bound.go
// they carry the foreign keys their forceResolver fields load through.

import (
	"time"

	"github.com/google/uuid"
)

type AutoReplyRule struct {
	ID               uuid.UUID        `json:"id"`
	Name             string           `json:"name"`
	Keywords         []string         `json:"keywords"`
	MatchType        AutoReplyMatch   `json:"matchType"`
	Body             *string          `json:"body,omitempty"`
	TemplateID       *uuid.UUID       `json:"-"`
	Platforms        []SocialPlatform `json:"platforms"`
	OnlyOutsideHours bool             `json:"onlyOutsideHours"`
	CooldownMinutes  int              `json:"cooldownMinutes"`
	Priority         int              `json:"priority"`
	IsActive         bool             `json:"isActive"`
	Hits             int              `json:"hits"`
	CreatedAt        time.Time        `json:"createdAt"`
}

type Campaign struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Body          string          `json:"body"`
	TemplateID    *uuid.UUID      `json:"-"`
	Channel       ReminderChannel `json:"channel"`
	Segment       map[string]any  `json:"segment"`
	Offer         *CampaignOffer  `json:"offer,omitempty"`
	ScheduledAt   *time.Time      `json:"scheduledAt,omitempty"`
	Status        CampaignStatus  `json:"status"`
	AudienceCount int             `json:"audienceCount"`
	SentCount     int             `json:"sentCount"`
	FailedCount   int             `json:"failedCount"`
	CreatedByID   *uuid.UUID      `json:"-"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

type Import struct {
	ID          uuid.UUID      `json:"id"`
	Kind        ImportKind     `json:"kind"`
	FileURL     string         `json:"fileUrl"`
	Status      ImportStatus   `json:"status"`
	Stats       map[string]any `json:"stats"`
	Error       *string        `json:"error,omitempty"`
	CreatedByID *uuid.UUID     `json:"-"`
	CreatedAt   time.Time      `json:"createdAt"`
	FinishedAt  *time.Time     `json:"finishedAt,omitempty"`
}
