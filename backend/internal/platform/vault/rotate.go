package vault

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
)

// RotateStats summarises a rotation run.
type RotateStats struct {
	Tenants   int
	Rotated   int // re-encrypted with the active key and AAD v2
	Failed    int // could not be decrypted with any configured key (left untouched)
	Remaining int // rows still not on (active key, AAD v2) after the run
}

// Rotate re-encrypts every secret that is not yet sealed with the active key and AAD v2
// (SECURITY L3). Each batch of at most batch rows runs in its own tenant transaction with
// the rows locked, so the API can keep serving while it runs and a crash loses nothing:
// re-running continues where it stopped. Rows that fail to decrypt are reported, skipped
// (keyset paging by id) and never modified.
func (v *Vault) Rotate(ctx context.Context, batch int) (RotateStats, error) {
	if batch <= 0 {
		batch = 500
	}
	var st RotateStats
	var tenants []uuid.UUID
	err := db.WithPlatformTx(ctx, v.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM tenants ORDER BY id`)
		if err != nil {
			return err
		}
		tenants, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		return err
	})
	if err != nil {
		return st, err
	}
	active := v.cipher.current
	for _, tid := range tenants {
		after := uuid.Nil
		touched := false
		for {
			n, failed, last, err := v.rotateBatch(ctx, tid, active, after, batch)
			if err != nil {
				return st, fmt.Errorf("rotate tenant %s: %w", tid, err)
			}
			st.Rotated += n
			st.Failed += failed
			if n+failed > 0 {
				touched = true
			}
			if n+failed < batch {
				break
			}
			after = last
		}
		if touched {
			st.Tenants++
		}
		var rem int
		if err := db.WithTenantTx(ctx, v.pool, tid, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM vault_secrets WHERE tenant_id = $1 AND (key_version <> $2 OR aad_version <> $3)`,
				tid, active, AADSecret).Scan(&rem)
		}); err != nil {
			return st, err
		}
		st.Remaining += rem
	}
	return st, nil
}

func (v *Vault) rotateBatch(ctx context.Context, tid uuid.UUID, active int, after uuid.UUID, batch int) (rotated, failed int, last uuid.UUID, err error) {
	err = db.WithTenantTx(ctx, v.pool, tid, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, kind, ciphertext, nonce, key_version, aad_version FROM vault_secrets
			WHERE tenant_id = $1 AND (key_version <> $2 OR aad_version <> $3) AND id > $4
			ORDER BY id LIMIT $5 FOR UPDATE`, tid, active, AADSecret, after, batch)
		if err != nil {
			return err
		}
		type row struct {
			id          uuid.UUID
			kind        string
			ct, nonce   []byte
			ver, aadVer int
		}
		rs, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
			var x row
			return x, r.Scan(&x.id, &x.kind, &x.ct, &x.nonce, &x.ver, &x.aadVer)
		})
		if err != nil {
			return err
		}
		for _, x := range rs {
			last = x.id
			pt, err := v.open(tid, x.id, x.kind, x.aadVer, x.ct, x.nonce, x.ver)
			if err != nil {
				failed++
				continue
			}
			ct, nonce, ver, err := v.seal(tid, x.id, x.kind, pt)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE vault_secrets SET ciphertext = $3, nonce = $4, key_version = $5, aad_version = $6
				WHERE tenant_id = $1 AND id = $2`, tid, x.id, ct, nonce, ver, AADSecret); err != nil {
				return err
			}
			rotated++
		}
		return nil
	})
	return rotated, failed, last, err
}
