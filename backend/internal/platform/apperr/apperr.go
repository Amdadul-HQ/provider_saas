// Package apperr defines the domain error kinds shared by every module.
// The GraphQL error presenter maps them to `extensions.code`.
package apperr

import (
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrValidation      = errors.New("validation failed")
	ErrForbidden       = errors.New("forbidden")
	ErrConflict        = errors.New("conflict")
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrRateLimited     = errors.New("rate limited")
)

// Error is a domain error with a user-facing message. errors.Is(err, ErrX) works.
type Error struct {
	Kind  error
	Msg   string
	Field string
	// RetryAfter (rate-limited errors) is exposed as extensions.retryAfter in seconds.
	RetryAfter time.Duration
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Unwrap() error { return e.Kind }

func NotFound(what string) error  { return &Error{Kind: ErrNotFound, Msg: what + " not found"} }
func Validation(msg string) error { return &Error{Kind: ErrValidation, Msg: msg} }
func ValidationField(field, msg string) error {
	return &Error{Kind: ErrValidation, Msg: field + ": " + msg, Field: field}
}
func Conflict(msg string) error        { return &Error{Kind: ErrConflict, Msg: msg} }
func Forbidden(msg string) error       { return &Error{Kind: ErrForbidden, Msg: msg} }
func Unauthenticated(msg string) error { return &Error{Kind: ErrUnauthenticated, Msg: msg} }
func RateLimited(msg string) error     { return &Error{Kind: ErrRateLimited, Msg: msg} }

// RateLimitedFor is RateLimited with a retry hint (extensions.retryAfter, seconds).
func RateLimitedFor(msg string, after time.Duration) error {
	return &Error{Kind: ErrRateLimited, Msg: msg, RetryAfter: after}
}

// Kinds lists every domain error kind; each must map to a stable extensions.code
// (TestEveryKindHasACode, SECURITY L5).
var Kinds = []error{ErrNotFound, ErrValidation, ErrForbidden, ErrConflict, ErrUnauthenticated, ErrRateLimited}

// RetryAfterOf returns the retry hint of a rate-limited error (0 = none).
func RetryAfterOf(err error) time.Duration {
	var e *Error
	if errors.As(err, &e) {
		return e.RetryAfter
	}
	return 0
}

// Code returns the GraphQL extensions.code for err ("" when err is not a domain error).
func Code(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, ErrValidation):
		return "BAD_USER_INPUT"
	case errors.Is(err, ErrForbidden):
		return "FORBIDDEN"
	case errors.Is(err, ErrConflict):
		return "CONFLICT"
	case errors.Is(err, ErrUnauthenticated):
		return "UNAUTHENTICATED"
	case errors.Is(err, ErrRateLimited):
		return "RATE_LIMITED"
	}
	return ""
}

// FieldOf returns the offending input field of a validation error, if any.
func FieldOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Field
	}
	return ""
}
