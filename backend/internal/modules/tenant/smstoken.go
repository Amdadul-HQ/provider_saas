package tenant

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
)

// Reserved tenants.settings keys managed by the server (never overwritten by updateTenant).
const (
	SettingSMSSecretID  = "sms_webhook_secret_id"
	SettingSMSTokenHint = "sms_webhook_token_hint"
)

// Secrets is the vault subset used for the SMS-forwarder token.
type Secrets interface {
	Store(ctx context.Context, tenantID uuid.UUID, kind string, plaintext []byte) (uuid.UUID, error)
	Open(ctx context.Context, tenantID, secretID uuid.UUID) ([]byte, error)
}

const smsTokenPrefix = "sms_"

// NewSMSToken builds "sms_<tenant hex>_<32 random bytes base64url>". The tenant part lets
// the webhook find the secret without a lookup table; the random part is the secret.
func NewSMSToken(tenantID uuid.UUID) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return smsTokenPrefix + hex.EncodeToString(tenantID[:]) + "_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// ParseSMSToken extracts the tenant id from a token.
func ParseSMSToken(tok string) (uuid.UUID, bool) {
	rest, ok := strings.CutPrefix(tok, smsTokenPrefix)
	if !ok {
		return uuid.Nil, false
	}
	h, _, ok := strings.Cut(rest, "_")
	if !ok || len(h) != 32 {
		return uuid.Nil, false
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.FromBytes(b)
	return id, err == nil
}

// SMSTokenHint is the masked form shown in settings ("sms_…AbCd").
func SMSTokenHint(tok string) string {
	if len(tok) < 8 {
		return "…"
	}
	return smsTokenPrefix + "…" + tok[len(tok)-4:]
}

// EnsureSMSWebhookToken creates the tenant's SMS-forwarder bearer token on first use.
// The plaintext is returned only when created (created=true); afterwards only the hint
// in tenants.settings is visible. The token itself lives only in the vault.
func (s *service) EnsureSMSWebhookToken(ctx context.Context, tenantID uuid.UUID) (token, hint string, created bool, err error) {
	if s.secrets == nil {
		return "", "", false, apperr.Validation("vault not configured")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		var cur map[string]any
		if err := tx.QueryRow(ctx, `SELECT settings FROM tenants WHERE id = $1 FOR UPDATE`, tenantID).Scan(&cur); err != nil {
			return db.MapErr(err, "tenant")
		}
		if id, _ := cur[SettingSMSSecretID].(string); id != "" {
			hint, _ = cur[SettingSMSTokenHint].(string)
			return nil
		}
		tok, err := NewSMSToken(tenantID)
		if err != nil {
			return err
		}
		sid, err := s.secrets.Store(ctx, tenantID, "api_key", []byte(tok))
		if err != nil {
			return err
		}
		token, hint, created = tok, SMSTokenHint(tok), true
		_, err = tx.Exec(ctx, `UPDATE tenants SET settings = settings || jsonb_build_object($2::text, $3::text, $4::text, $5::text) WHERE id = $1`,
			tenantID, SettingSMSSecretID, sid.String(), SettingSMSTokenHint, hint)
		if err != nil {
			return err
		}
		return s.repo.InsertAudit(ctx, tx, tenantID, nil, "sms_token.create", "tenant", &tenantID, nil)
	})
	return token, hint, created, err
}

// VerifySMSWebhookToken resolves a bearer token to its tenant (constant-time compare).
func (s *service) VerifySMSWebhookToken(ctx context.Context, token string) (uuid.UUID, error) {
	bad := apperr.Unauthenticated("invalid sms token")
	tid, ok := ParseSMSToken(token)
	if !ok || s.secrets == nil {
		return uuid.Nil, bad
	}
	var sidStr string
	err := s.tx(ctx, tid, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COALESCE(settings->>$2, '') FROM tenants WHERE id = $1 AND status = 'active'`,
			tid, SettingSMSSecretID).Scan(&sidStr)
	})
	sid, perr := uuid.Parse(sidStr)
	if err != nil || perr != nil {
		return uuid.Nil, bad
	}
	want, err := s.secrets.Open(ctx, tid, sid)
	if err != nil || subtle.ConstantTimeCompare(want, []byte(token)) != 1 {
		return uuid.Nil, bad
	}
	return tid, nil
}

// PublicSettings strips server-managed secret references before settings leave the API.
func PublicSettings(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if k == SettingSMSSecretID {
			continue
		}
		out[k] = v
	}
	return out
}
