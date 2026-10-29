package auth

import (
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/phone"
)

// User is a human who logs in.
type User struct {
	ID              uuid.UUID
	Name            string
	Email           *string
	Phone           *string
	AvatarURL       *string
	IsPlatformAdmin bool
	PasswordHash    *string
	CreatedAt       time.Time
}

// Session is what register/login/refresh/switchTenant return.
type Session struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	User         User
	Membership   *tenant.Membership // nil when the user has no business yet
}

type RegisterInput struct {
	Name         string
	Email        *string
	Phone        *string
	Password     string
	BusinessName string
}

type LoginInput struct {
	Identifier string // email or phone
	Password   string
}

// ClientMeta is stored with refresh tokens.
type ClientMeta struct {
	UserAgent string
	IP        string
}

const MinPasswordLen = 8

// ValidatePassword enforces length >= 8 with at least one letter and one digit.
func ValidatePassword(pw string) error {
	if len(pw) < MinPasswordLen {
		return apperr.ValidationField("password", "must be at least 8 characters")
	}
	var letter, digit bool
	for _, r := range pw {
		letter = letter || unicode.IsLetter(r)
		digit = digit || unicode.IsDigit(r)
	}
	if !letter || !digit {
		return apperr.ValidationField("password", "must contain a letter and a digit")
	}
	return nil
}

// NormalizeEmail lower-cases and validates an email; empty -> nil.
func NormalizeEmail(e *string) (*string, error) {
	if e == nil || strings.TrimSpace(*e) == "" {
		return nil, nil
	}
	v := strings.ToLower(strings.TrimSpace(*e))
	if a, err := mail.ParseAddress(v); err != nil || a.Address != v {
		return nil, apperr.ValidationField("email", "invalid email")
	}
	return &v, nil
}

// NormalizePhone validates a BD mobile number; empty -> nil.
func NormalizePhone(p *string) (*string, error) {
	if p == nil || strings.TrimSpace(*p) == "" {
		return nil, nil
	}
	v, ok := phone.Normalize(*p)
	if !ok {
		return nil, apperr.ValidationField("phone", "invalid Bangladeshi mobile number")
	}
	return &v, nil
}

// Refresh tokens are "<random>.<tenantID>" so the tenant survives a refresh; the
// whole string is hashed, so tampering with the tenant part invalidates the token.
func splitRefresh(tok string) (uuid.UUID, bool) {
	i := strings.LastIndexByte(tok, '.')
	if i < 0 {
		return uuid.Nil, true
	}
	id, err := uuid.Parse(tok[i+1:])
	return id, err == nil
}
