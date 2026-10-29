// Package vault encrypts secrets (social tokens, master-account credentials) with
// AES-256-GCM (ADR-004). Ciphertext is bound to its tenant, its own row id and its kind
// via GCM additional data (AAD v2, SECURITY L3), so a ciphertext copied to another row or
// tenant cannot be decrypted. Keys are versioned (VAULT_KEYS); `api rotate-vault`
// re-encrypts every secret with the active key. Reveal is audit-logged.
package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/db"
)

// Secret kinds (vault_secrets.kind).
const (
	KindSocialToken       = "social_token"
	KindAccountCredential = "account_credential"
	KindAPIKey            = "api_key"
	KindOther             = "other"
)

// Cipher is the pure crypto part (no database), keyed by version for rotation.
type Cipher struct {
	keys    map[int][]byte
	current int
}

// NewCipher builds a cipher with the current key. Older keys can be added with AddKey.
func NewCipher(key []byte, version int) (*Cipher, error) {
	if len(key) != 32 {
		return nil, errors.New("vault: key must be 32 bytes")
	}
	return &Cipher{keys: map[int][]byte{version: key}, current: version}, nil
}

// AddKey registers an older key version for decryption.
func (c *Cipher) AddKey(version int, key []byte) { c.keys[version] = key }

func (c *Cipher) gcm(version int) (cipher.AEAD, error) {
	key, ok := c.keys[version]
	if !ok {
		return nil, fmt.Errorf("vault: unknown key version %d", version)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Encrypt seals plaintext with the current key; aad binds it to a context (tenant id).
func (c *Cipher) Encrypt(plaintext, aad []byte) (ciphertext, nonce []byte, version int, err error) {
	g, err := c.gcm(c.current)
	if err != nil {
		return nil, nil, 0, err
	}
	nonce = make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, 0, err
	}
	return g.Seal(nil, nonce, plaintext, aad), nonce, c.current, nil
}

// Decrypt opens a ciphertext produced by Encrypt.
func (c *Cipher) Decrypt(ciphertext, nonce, aad []byte, version int) ([]byte, error) {
	g, err := c.gcm(version)
	if err != nil {
		return nil, err
	}
	if len(nonce) != g.NonceSize() {
		return nil, errors.New("vault: bad nonce")
	}
	pt, err := g.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errors.New("vault: decryption failed")
	}
	return pt, nil
}

// Versions lists the key versions the cipher can decrypt, ascending.
func (c *Cipher) Versions() []int {
	out := make([]int, 0, len(c.keys))
	for v := range c.keys {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// Active is the key version new ciphertexts are sealed with.
func (c *Cipher) Active() int { return c.current }

// AAD versions (vault_secrets.aad_version, migration 00016).
const (
	AADTenant = 1 // legacy: tenant id only
	AADSecret = 2 // "vault:v2|tenant|secret id|kind"
)

// AAD returns the GCM additional data of a stored secret.
func AAD(version int, tenantID, secretID uuid.UUID, kind string) ([]byte, error) {
	switch version {
	case AADTenant:
		return tenantID[:], nil
	case AADSecret:
		return []byte("vault:v2|" + tenantID.String() + "|" + secretID.String() + "|" + kind), nil
	}
	return nil, fmt.Errorf("vault: unknown aad version %d", version)
}

// Vault stores encrypted secrets in vault_secrets.
type Vault struct {
	pool   *pgxpool.Pool
	cipher *Cipher
}

// New builds a vault with a single key.
func New(pool *pgxpool.Pool, key []byte, version int) (*Vault, error) {
	return NewWithKeys(pool, map[int][]byte{version: key}, version)
}

// NewWithKeys builds a vault that seals with keys[active] and opens any listed version
// (VAULT_KEYS="v1:hex,v2:hex", VAULT_ACTIVE_KEY_VERSION=2).
func NewWithKeys(pool *pgxpool.Pool, keys map[int][]byte, active int) (*Vault, error) {
	k, ok := keys[active]
	if !ok {
		return nil, fmt.Errorf("vault: active key version %d is not configured", active)
	}
	c, err := NewCipher(k, active)
	if err != nil {
		return nil, err
	}
	for ver, key := range keys {
		if len(key) != 32 {
			return nil, fmt.Errorf("vault: key v%d must be 32 bytes", ver)
		}
		c.AddKey(ver, key)
	}
	return &Vault{pool: pool, cipher: c}, nil
}

// ActiveVersion is the key version new secrets are sealed with.
func (v *Vault) ActiveVersion() int { return v.cipher.current }

func (v *Vault) seal(tenantID, secretID uuid.UUID, kind string, plaintext []byte) (ct, nonce []byte, ver int, err error) {
	aad, _ := AAD(AADSecret, tenantID, secretID, kind)
	return v.cipher.Encrypt(plaintext, aad)
}

func (v *Vault) open(tenantID, secretID uuid.UUID, kind string, aadVersion int, ct, nonce []byte, keyVersion int) ([]byte, error) {
	aad, err := AAD(aadVersion, tenantID, secretID, kind)
	if err != nil {
		return nil, err
	}
	return v.cipher.Decrypt(ct, nonce, aad, keyVersion)
}

// Store encrypts plaintext and returns the new secret id. The id is chosen before sealing
// so it can be part of the AAD. Joins a tx carried by ctx.
func (v *Vault) Store(ctx context.Context, tenantID uuid.UUID, kind string, plaintext []byte) (uuid.UUID, error) {
	id := uuid.New()
	ct, nonce, ver, err := v.seal(tenantID, id, kind, plaintext)
	if err != nil {
		return uuid.Nil, err
	}
	var createdBy *uuid.UUID
	if p, ok := auth.FromContext(ctx); ok {
		createdBy = &p.UserID
	}
	err = db.WithTenantTx(ctx, v.pool, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO vault_secrets (id, tenant_id, kind, ciphertext, nonce, key_version, aad_version, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, id, tenantID, kind, ct, nonce, ver, AADSecret, createdBy)
		return err
	})
	if err != nil {
		return uuid.Nil, db.MapErr(err, "secret")
	}
	return id, nil
}

// Update re-encrypts an existing secret with new plaintext (active key, AAD v2).
func (v *Vault) Update(ctx context.Context, tenantID, secretID uuid.UUID, plaintext []byte) error {
	return db.WithTenantTx(ctx, v.pool, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		var kind string
		err := tx.QueryRow(ctx, `SELECT kind FROM vault_secrets WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, secretID).Scan(&kind)
		if err != nil {
			return db.MapErr(err, "secret")
		}
		ct, nonce, ver, err := v.seal(tenantID, secretID, kind, plaintext)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE vault_secrets SET ciphertext=$3, nonce=$4, key_version=$5, aad_version=$6
			WHERE tenant_id=$1 AND id=$2`, tenantID, secretID, ct, nonce, ver, AADSecret)
		return err
	})
}

// Reveal decrypts a secret and writes an audit_logs row (actor from ctx Principal).
func (v *Vault) Reveal(ctx context.Context, tenantID, secretID uuid.UUID) ([]byte, error) {
	var out []byte
	err := db.WithTenantTx(ctx, v.pool, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		var ct, nonce []byte
		var ver, aadVer int
		var kind string
		err := tx.QueryRow(ctx, `SELECT ciphertext, nonce, key_version, aad_version, kind FROM vault_secrets
			WHERE tenant_id=$1 AND id=$2`, tenantID, secretID).Scan(&ct, &nonce, &ver, &aadVer, &kind)
		if err != nil {
			return db.MapErr(err, "secret")
		}
		pt, err := v.open(tenantID, secretID, kind, aadVer, ct, nonce, ver)
		if err != nil {
			return err
		}
		var userID *uuid.UUID
		if p, ok := auth.FromContext(ctx); ok {
			userID = &p.UserID
		}
		meta, _ := json.Marshal(map[string]any{"kind": kind})
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (tenant_id, user_id, action, entity_type, entity_id, meta)
			VALUES ($1,$2,'vault.reveal','vault_secret',$3,$4)`, tenantID, userID, secretID, meta); err != nil {
			return err
		}
		out = pt
		return nil
	})
	return out, err
}

// Delete removes a secret.
func (v *Vault) Delete(ctx context.Context, tenantID, secretID uuid.UUID) error {
	return db.WithTenantTx(ctx, v.pool, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM vault_secrets WHERE tenant_id=$1 AND id=$2`, tenantID, secretID)
		return err
	})
}

// Open decrypts a secret for server-side machine use (calling Meta/Telegram APIs with a
// stored token, verifying a webhook bearer token). Unlike Reveal it writes no audit row:
// the plaintext never leaves the process. Anything shown to a human must use Reveal,
// and callers that render a secret into an outbound message must audit explicitly.
func (v *Vault) Open(ctx context.Context, tenantID, secretID uuid.UUID) ([]byte, error) {
	var ct, nonce []byte
	var ver, aadVer int
	var kind string
	err := db.WithTenantTx(ctx, v.pool, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT ciphertext, nonce, key_version, aad_version, kind FROM vault_secrets
			WHERE tenant_id=$1 AND id=$2`, tenantID, secretID).Scan(&ct, &nonce, &ver, &aadVer, &kind)
	})
	if err != nil {
		return nil, db.MapErr(err, "secret")
	}
	return v.open(tenantID, secretID, kind, aadVer, ct, nonce, ver)
}
