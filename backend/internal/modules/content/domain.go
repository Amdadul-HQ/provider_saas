package content

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"provider_saas/internal/modules/social"
	"provider_saas/internal/platform/apperr"
)

// Post statuses.
const (
	StatusDraft      = "draft"
	StatusScheduled  = "scheduled"
	StatusPublishing = "publishing"
	StatusPublished  = "published"
	StatusPartial    = "partial"
	StatusFailed     = "failed"

	TargetPending   = "pending"
	TargetPublished = "published"
	TargetFailed    = "failed"
)

// JobPublish publishes a post to all its targets (dedupe key post.publish:<id>).
const JobPublish = "post.publish"

type Post struct {
	ID          uuid.UUID
	Body        string
	Media       []social.Attachment
	ScheduledAt *time.Time
	Status      string
	CreatedBy   *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Editable reports whether the post may still be changed / deleted.
func (p Post) Editable() bool { return p.Status != StatusPublishing && p.Status != StatusPublished }

type Target struct {
	ID              uuid.UUID
	PostID          uuid.UUID
	SocialAccountID uuid.UUID
	ExternalPostID  *string
	Status          string
	Error           *string
	PublishedAt     *time.Time
}

type PostInput struct {
	Body             string
	Media            []social.Attachment
	ScheduledAt      *time.Time
	SocialAccountIDs []uuid.UUID
}

func (in *PostInput) Validate(now time.Time) error {
	in.Body = strings.TrimSpace(in.Body)
	switch {
	case in.Body == "":
		return apperr.ValidationField("body", "required")
	case utf8.RuneCountInString(in.Body) > 5000:
		return apperr.ValidationField("body", "max 5000 characters")
	case len(in.SocialAccountIDs) == 0:
		return apperr.ValidationField("socialAccountIds", "pick at least one account")
	case len(in.Media) > 10:
		return apperr.ValidationField("media", "max 10 items")
	case in.ScheduledAt != nil && in.ScheduledAt.Before(now.Add(-time.Minute)):
		return apperr.ValidationField("scheduledAt", "must be in the future")
	}
	seen := map[uuid.UUID]bool{}
	ids := in.SocialAccountIDs[:0]
	for _, id := range in.SocialAccountIDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	in.SocialAccountIDs = ids
	for i, m := range in.Media {
		if !strings.HasPrefix(m.URL, "http://") && !strings.HasPrefix(m.URL, "https://") {
			return apperr.ValidationField("media", "every item needs an http(s) url")
		}
		if m.Type == "" {
			in.Media[i].Type = "image"
		}
	}
	return nil
}

// OverallStatus derives the post status from its targets' results.
func OverallStatus(targets []Target) string {
	ok, failed := 0, 0
	for _, t := range targets {
		switch t.Status {
		case TargetPublished:
			ok++
		case TargetFailed:
			failed++
		}
	}
	switch {
	case ok == len(targets) && ok > 0:
		return StatusPublished
	case ok == 0:
		return StatusFailed
	default:
		return StatusPartial
	}
}

type Filter struct {
	Status *string
}
