# ADR-004 Money and secrets
**Money**: `amount bigint` in minor units (paisa) + `currency char(3)` default BDT. Never float.
**Secrets**: social access tokens and master-account credentials go in `vault_secrets(ciphertext, nonce, key_version)` encrypted with AES-256-GCM using `VAULT_KEY`. Reveal requires `owner`/`admin` role and writes an audit log.
