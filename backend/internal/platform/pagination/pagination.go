// Package pagination is the one page type for every list query (ADR-010): opaque cursors
// (base64 of created_at + id) for the GraphQL *Connection types, or the last row id for
// plain array lists. platform/sqlb turns a Page into SQL.
package pagination

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultFirst = 25
	MaxFirst     = 100
)

var ErrBadCursor = errors.New("invalid cursor")

// Cursor identifies a row position in a (created_at DESC, id DESC) ordering.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func Encode(createdAt time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
}

func Decode(s string) (Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, ErrBadCursor
	}
	ts, idStr, ok := strings.Cut(string(b), "|")
	if !ok {
		return Cursor{}, ErrBadCursor
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return Cursor{}, ErrBadCursor
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return Cursor{}, ErrBadCursor
	}
	return Cursor{CreatedAt: t, ID: id}, nil
}

// Page is a normalised page request. At most one of After and AfterID is set.
type Page struct {
	First   int
	After   *Cursor    // endCursor of a Connection
	AfterID *uuid.UUID // id of the last row of an array list (frontend arrayPageInfo)
}

// New normalises first (default 25, max 100) and accepts either cursor form for after.
func New(first *int, after *string) (Page, error) {
	p := Page{First: DefaultFirst}
	if first != nil {
		p.First = *first
	}
	if p.First <= 0 {
		p.First = DefaultFirst
	}
	if p.First > MaxFirst {
		p.First = MaxFirst
	}
	if after != nil && *after != "" {
		if id, err := uuid.Parse(*after); err == nil {
			p.AfterID = &id
			return p, nil
		}
		c, err := Decode(*after)
		if err != nil {
			return p, err
		}
		p.After = &c
	}
	return p, nil
}

// Trim cuts a slice fetched with LIMIT First+1 and reports whether there is a next page.
func Trim[T any](rows []T, first int) ([]T, bool) {
	if len(rows) > first {
		return rows[:first], true
	}
	return rows, false
}
