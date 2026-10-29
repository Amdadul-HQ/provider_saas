package social

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

// StateTTL is how long a startMetaConnect state stays valid.
const StateTTL = 10 * time.Minute

// State is the payload of the signed OAuth state parameter.
type State struct {
	TenantID uuid.UUID `json:"t"`
	UserID   uuid.UUID `json:"u"`
	Platform string    `json:"p"`
	Exp      int64     `json:"e"`
	Nonce    string    `json:"n"`
}

// SignState encodes st as base64url(json) + "." + base64url(HMAC-SHA256).
func SignState(secret []byte, st State) string {
	if st.Nonce == "" {
		b := make([]byte, 9)
		_, _ = rand.Read(b)
		st.Nonce = base64.RawURLEncoding.EncodeToString(b)
	}
	body, _ := json.Marshal(st)
	enc := base64.RawURLEncoding.EncodeToString(body)
	return enc + "." + base64.RawURLEncoding.EncodeToString(mac(secret, "oauth-state:"+enc))
}

// VerifyState checks signature and expiry.
func VerifyState(secret []byte, s string, now time.Time) (State, error) {
	var st State
	enc, sig, ok := strings.Cut(s, ".")
	if !ok {
		return st, apperr.ValidationField("state", "malformed")
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, mac(secret, "oauth-state:"+enc)) {
		return st, apperr.ValidationField("state", "invalid signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil || json.Unmarshal(body, &st) != nil {
		return st, apperr.ValidationField("state", "malformed")
	}
	if now.Unix() > st.Exp {
		return st, apperr.ValidationField("state", "expired; start the connection again")
	}
	return st, nil
}

// WebhookSecret derives the per-account Telegram secret token (sent back by Telegram in
// X-Telegram-Bot-Api-Secret-Token). Derived, so nothing extra is stored.
func WebhookSecret(secret []byte, accountID uuid.UUID) string {
	return hex.EncodeToString(mac(secret, "telegram-webhook:"+accountID.String()))
}

// VerifyMetaSignature checks an X-Hub-Signature-256 header ("sha256=<hex>") against body.
func VerifyMetaSignature(appSecret string, body []byte, header string) bool {
	if appSecret == "" {
		return false
	}
	hexSig, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, []byte(appSecret))
	m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}

// SignMetaBody returns the X-Hub-Signature-256 header value for body (tests, tooling).
func SignMetaBody(appSecret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(appSecret))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func mac(secret []byte, msg string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(msg))
	return m.Sum(nil)
}
