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
