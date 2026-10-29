package auth

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		pw string
		ok bool
	}{{"Demo1234!", true}, {"short1", false}, {"onlyletters", false}, {"12345678", false}, {"পাসওয়ার্ড1234", true}}
	for _, tt := range tests {
		err := ValidatePassword(tt.pw)
		if (err == nil) != tt.ok {
			t.Errorf("ValidatePassword(%q) = %v", tt.pw, err)
		}
		if err != nil && !errors.Is(err, apperr.ErrValidation) {
			t.Errorf("want validation error, got %v", err)
		}
	}
}

func TestNormalizeIdentity(t *testing.T) {
	s := func(v string) *string { return &v }
	e, err := NormalizeEmail(s(" Demo@Provider.TEST "))
	if err != nil || *e != "demo@provider.test" {
		t.Fatalf("email = %v %v", e, err)
	}
	if _, err := NormalizeEmail(s("nope")); err == nil {
		t.Fatal("invalid email accepted")
	}
	if e, _ := NormalizeEmail(s("")); e != nil {
		t.Fatal("empty email should be nil")
	}
	p, err := NormalizePhone(s("+8801712345678"))
	if err != nil || *p != "01712345678" {
		t.Fatalf("phone = %v %v", p, err)
	}
	if _, err := NormalizePhone(s("12345")); err == nil {
		t.Fatal("invalid phone accepted")
	}
}

func TestSplitRefresh(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		tok  string
		want uuid.UUID
		ok   bool
	}{{"abc." + id.String(), id, true}, {"abc", uuid.Nil, true}, {"abc.not-a-uuid", uuid.Nil, false}}
	for _, tt := range tests {
		got, ok := splitRefresh(tt.tok)
		if got != tt.want || ok != tt.ok {
			t.Errorf("splitRefresh(%q) = %v,%v", tt.tok, got, ok)
		}
	}
}
