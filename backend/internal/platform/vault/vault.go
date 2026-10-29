// Package vault encrypts secrets (social tokens, master-account credentials) with
// AES-256-GCM (ADR-004). Ciphertext is bound to its tenant via GCM additional data,
// so a row copied to another tenant cannot be decrypted. Reveal is audit-logged.
package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/apperr"
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

// Vault stores encrypted secrets in vault_secrets.
type Vault struct {
	pool   *pgxpool.Pool
	cipher *Cipher
}

func New(pool *pgxpool.Pool, key []byte, version int) (*Vault, error) {
	c, err := NewCipher(key, version)
	if err != nil {
		return nil, err
	}
	return &Vault{pool: pool, cipher: c}, nil
}

// Store encrypts plaintext and returns the new secret id. Joins a tx carried by ctx.
func (v *Vault) Store(ctx context.Context, tenantID uuid.UUID, kind string, plaintext []byte) (uuid.UUID, error) {
	ct, nonce, ver, err := v.cipher.Encrypt(plaintext, tenantID[:])
	if err != nil {
		return uuid.Nil, err
	}
	var createdBy *uuid.UUID
	if p, ok := auth.FromContext(ctx); ok {
		createdBy = &p.UserID
	}
	var id uuid.UUID
	err = db.WithTenantTx(ctx, v.pool, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `INSERT INTO vault_secrets (tenant_id, kind, ciphertext, nonce, key_version, created_by)
			VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, tenantID, kind, ct, nonce, ver, createdBy).Scan(&id)
	})
	return id, db.MapErr(err, "secret")
}

// Update re-encrypts an existing secret (also used for key rotation).
func (v *Vault) Update(ctx context.Context, tenantID, secretID uuid.UUID, plaintext []byte) error {
	ct, nonce, ver, err := v.cipher.Encrypt(plaintext, tenantID[:])
	if err != nil {
		return err
	}
	return db.WithTenantTx(ctx, v.pool, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE vault_secrets SET ciphertext=$3, nonce=$4, key_version=$5
			WHERE tenant_id=$1 AND id=$2`, tenantID, secretID, ct, nonce, ver)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound("secret")
		}
		return nil
	})
}

// Reveal decrypts a secret and writes an audit_logs row (actor from ctx Principal).
func (v *Vault) Reveal(ctx context.Context, tenantID, secretID uuid.UUID) ([]byte, error) {
	var out []byte
	err := db.WithTenantTx(ctx, v.pool, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		var ct, nonce []byte
		var ver int
		var kind string
		err := tx.QueryRow(ctx, `SELECT ciphertext, nonce, key_version, kind FROM vault_secrets
			WHERE tenant_id=$1 AND id=$2`, tenantID, secretID).Scan(&ct, &nonce, &ver, &kind)
		if err != nil {
			return db.MapErr(err, "secret")
		}
		pt, err := v.cipher.Decrypt(ct, nonce, tenantID[:], ver)
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
