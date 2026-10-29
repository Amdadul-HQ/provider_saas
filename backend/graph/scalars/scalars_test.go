package scalars

import (
	"errors"
	"testing"

	"provider_saas/internal/platform/apperr"
)

func TestUnmarshalErrorsAreValidation(t *testing.T) {
	if _, err := UnmarshalUUID("nope"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("uuid: %v", err)
	}
	if _, err := UnmarshalTime("yesterday"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("time: %v", err)
	}
	if _, err := UnmarshalMoney("12.5"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("money: %v", err)
	}
	if m, err := UnmarshalMoney(35000); err != nil || m != 35000 {
		t.Fatalf("money ok: %d %v", m, err)
	}
}
