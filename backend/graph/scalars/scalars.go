// Package scalars wraps gqlgen's UUID/Time/Money scalars so that malformed client
// input is reported as BAD_USER_INPUT (apperr.ErrValidation) instead of an internal error.
package scalars

import (
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

func MarshalUUID(id uuid.UUID) graphql.Marshaler { return graphql.MarshalUUID(id) }

func UnmarshalUUID(v any) (uuid.UUID, error) {
	id, err := graphql.UnmarshalUUID(v)
	if err != nil {
		return id, apperr.Validation("invalid UUID")
	}
	return id, nil
}

func MarshalTime(t time.Time) graphql.Marshaler { return graphql.MarshalTime(t) }

func UnmarshalTime(v any) (time.Time, error) {
	t, err := graphql.UnmarshalTime(v)
	if err != nil {
		return t, apperr.Validation("invalid time: use RFC3339, e.g. 2026-10-29T10:15:00+06:00")
	}
	return t, nil
}

// Money is int64 minor units (paisa).
func MarshalMoney(m int64) graphql.Marshaler { return graphql.MarshalInt64(m) }

func UnmarshalMoney(v any) (int64, error) {
	m, err := graphql.UnmarshalInt64(v)
	if err != nil {
		return m, apperr.Validation("invalid money amount: send integer minor units (paisa)")
	}
	return m, nil
}
