package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestCode(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{NotFound("customer"), "NOT_FOUND"},
		{fmt.Errorf("wrap: %w", Conflict("no free slot")), "CONFLICT"},
		{Validation("bad"), "BAD_USER_INPUT"},
		{Forbidden("no"), "FORBIDDEN"},
		{Unauthenticated("login"), "UNAUTHENTICATED"},
		{errors.New("boom"), ""},
		{nil, ""},
	}
	for _, tt := range tests {
		if got := Code(tt.err); got != tt.want {
			t.Errorf("Code(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
	if FieldOf(ValidationField("email", "invalid")) != "email" {
		t.Fatal("FieldOf")
	}
}

// SECURITY L5 follow-up: every kind maps to a stable, non-empty extensions.code, and the
// codes are distinct (the frontend switches on them).
func TestEveryKindHasACode(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Kinds {
		code := Code(&Error{Kind: k, Msg: "x"})
		if code == "" || seen[code] {
			t.Fatalf("kind %v: code %q empty or duplicate", k, code)
		}
		seen[code] = true
	}
	if Code(RateLimitedFor("slow down", 3e9)) != "RATE_LIMITED" || RetryAfterOf(RateLimitedFor("x", 3e9)) != 3e9 {
		t.Fatal("RateLimitedFor")
	}
}
