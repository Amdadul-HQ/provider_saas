package vault

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
)

// SealTransient encrypts a short-lived secret that must survive only until a job runs
// (SECURITY H3: the rendered delivery message with the master-account password travels in
// the inbox.send job payload, never in messages.body). The ciphertext is bound to the
// tenant and a purpose string (e.g. the message id) through GCM additional data.
// Format: "v<key version>.<nonce b64url>.<ciphertext b64url>".
func (v *Vault) SealTransient(tenantID uuid.UUID, purpose string, plaintext []byte) (string, error) {
	ct, nonce, ver, err := v.cipher.Encrypt(plaintext, transientAAD(tenantID, purpose))
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	return "v" + strconv.Itoa(ver) + "." + enc.EncodeToString(nonce) + "." + enc.EncodeToString(ct), nil
}

// OpenTransient decrypts a value produced by SealTransient for the same tenant and purpose.
func (v *Vault) OpenTransient(tenantID uuid.UUID, purpose, sealed string) ([]byte, error) {
	parts := strings.Split(sealed, ".")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "v") {
		return nil, errors.New("vault: malformed sealed value")
	}
	ver, err := strconv.Atoi(parts[0][1:])
	if err != nil {
		return nil, errors.New("vault: malformed sealed value")
	}
	enc := base64.RawURLEncoding
	nonce, err1 := enc.DecodeString(parts[1])
	ct, err2 := enc.DecodeString(parts[2])
	if err1 != nil || err2 != nil {
		return nil, errors.New("vault: malformed sealed value")
	}
	return v.cipher.Decrypt(ct, nonce, transientAAD(tenantID, purpose), ver)
}

func transientAAD(tenantID uuid.UUID, purpose string) []byte {
	return append(append([]byte("transient:"), tenantID[:]...), []byte(":"+purpose)...)
}

// Fingerprint identifies a key without revealing it: hex(sha256(key))[:16].
func Fingerprint(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:])[:16]
}

// Fingerprint of the vault's current key.
func (v *Vault) Fingerprint() string { return Fingerprint(v.cipher.keys[v.cipher.current]) }

// SettingKeyFingerprint is the tenants.settings key written by seed (QA BUG-01).
const SettingKeyFingerprint = "vault_key_fingerprint"

// SelfCheck detects a VAULT_KEY that differs from the one existing secrets were written
// with (QA BUG-01): tenants carrying a recorded key fingerprint must match, and one
// sampled secret of a few tenants must decrypt. The API logs a WARN when it fails.
func (v *Vault) SelfCheck(ctx context.Context) error {
	fp := v.Fingerprint()
	rows, err := v.pool.Query(ctx, `SELECT id, COALESCE(settings->>$1, '') FROM tenants WHERE status = 'active'
		ORDER BY (settings ? $1) DESC, created_at LIMIT 5`, SettingKeyFingerprint)
	if err != nil {
		return err
	}
	type t struct {
		id uuid.UUID
		fp string
	}
	ts, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (t, error) {
		var x t
		return x, r.Scan(&x.id, &x.fp)
	})
	if err != nil {
		return err
	}
	for _, x := range ts {
		if x.fp != "" && x.fp != fp {
			return fmt.Errorf("vault: VAULT_KEY fingerprint %s does not match the key tenant %s was seeded with (%s)", fp, x.id, x.fp)
		}
		var ct, nonce []byte
		var ver, aadVer int
		var sid uuid.UUID
		var kind string
		err := db.WithTenantTx(ctx, v.pool, x.id, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT id, kind, ciphertext, nonce, key_version, aad_version FROM vault_secrets WHERE tenant_id = $1
				ORDER BY created_at DESC LIMIT 1`, x.id).Scan(&sid, &kind, &ct, &nonce, &ver, &aadVer)
		})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := v.open(x.id, sid, kind, aadVer, ct, nonce, ver); err != nil {
			return fmt.Errorf("vault: a stored secret of tenant %s cannot be decrypted with the current VAULT_KEY (fingerprint %s)", x.id, fp)
		}
	}
	return nil
}
