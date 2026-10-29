package auth

import (
	"crypto/hkdf"
	"crypto/sha256"
)

// Keys derives independent purpose keys from JWT_SECRET with HKDF-SHA256 (SECURITY L9):
// a key used for one purpose (access JWTs) is never used for another (OAuth state,
// Telegram webhook secrets, SSE events tokens), so a token of one kind can never verify
// as another and one purpose can be moved to its own secret later without touching the rest.
type Keys struct{ root []byte }

// hkdfSalt is fixed and public (RFC 5869 §3.1: a salt adds strength even when not secret).
var hkdfSalt = []byte("provider_saas/hkdf/v1")

// NewKeys builds the key set from the root secret (JWT_SECRET, ≥32 bytes in config).
func NewKeys(secret string) Keys { return Keys{root: []byte(secret)} }

func (k Keys) derive(label string) []byte {
	key, err := hkdf.Key(sha256.New, k.root, hkdfSalt, label, 32)
	if err != nil { // only possible for keyLength > 255*32
		panic("hkdf: " + err.Error())
	}
	return key
}

// JWTKey signs access tokens (HS256).
func (k Keys) JWTKey() []byte { return k.derive("jwt-access/v1") }

// OAuthStateKey signs Meta OAuth state parameters.
func (k Keys) OAuthStateKey() []byte { return k.derive("oauth-state/v1") }

// TelegramSecretKey derives per-account Telegram webhook secret tokens.
func (k Keys) TelegramSecretKey() []byte { return k.derive("telegram-webhook/v1") }

// EventsTokenKey signs the short-lived SSE events tokens (GET /events?token=...).
func (k Keys) EventsTokenKey() []byte { return k.derive("events-token/v1") }
