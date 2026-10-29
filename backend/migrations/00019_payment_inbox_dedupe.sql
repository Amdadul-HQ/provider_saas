-- +goose Up
-- SPRINT-01-SECURITY I2: a forwarded payment SMS is ingested once. IngestSMS skips an SMS
-- whose (method, trx_id, amount) or sha256(raw text) was already ingested for the tenant in
-- the last 24 hours (serialised per tenant with a transaction advisory lock).
ALTER TABLE payment_inbox ADD COLUMN raw_sha256 text;
UPDATE payment_inbox SET raw_sha256 = encode(digest(raw_text, 'sha256'), 'hex') WHERE raw_sha256 IS NULL;
CREATE INDEX payment_inbox_dedupe_trx_idx ON payment_inbox (tenant_id, method, trx_id, amount, created_at)
  WHERE trx_id IS NOT NULL;
CREATE INDEX payment_inbox_dedupe_raw_idx ON payment_inbox (tenant_id, raw_sha256, created_at);

-- +goose Down
DROP INDEX IF EXISTS payment_inbox_dedupe_raw_idx;
DROP INDEX IF EXISTS payment_inbox_dedupe_trx_idx;
ALTER TABLE payment_inbox DROP COLUMN IF EXISTS raw_sha256;
