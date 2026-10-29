-- +goose Up
-- SPRINT-01-SECURITY L3: GCM additional data v2 = "vault:v2|tenant|secret id|kind", so a
-- ciphertext copied onto another secret row (same tenant) no longer decrypts. Rows written
-- before this migration keep aad_version 1 (AAD = tenant id) until `api rotate-vault`
-- re-encrypts them with the active key and AAD v2.
ALTER TABLE vault_secrets ADD COLUMN aad_version smallint NOT NULL DEFAULT 1 CHECK (aad_version IN (1, 2));
CREATE INDEX vault_secrets_rotation_idx ON vault_secrets (tenant_id, key_version, aad_version);

-- +goose Down
DROP INDEX IF EXISTS vault_secrets_rotation_idx;
ALTER TABLE vault_secrets DROP COLUMN IF EXISTS aad_version;
