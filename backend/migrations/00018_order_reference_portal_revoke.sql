-- +goose Up
-- SPRINT-01-SECURITY L6: publicOrder is looked up by an unguessable public reference
-- (10 chars Crockford base32 = 50 random bits) instead of the per-tenant order number.
-- +goose StatementBegin
CREATE FUNCTION psaas_order_reference() RETURNS text LANGUAGE sql VOLATILE AS $$
  SELECT string_agg(substr('0123456789ABCDEFGHJKMNPQRSTVWXYZ', (get_byte(b, i) % 32) + 1, 1), '' ORDER BY i)
  FROM (SELECT gen_random_bytes(10) AS b) r, generate_series(0, 9) AS i
$$;
-- +goose StatementEnd
ALTER TABLE orders ADD COLUMN reference text;
UPDATE orders SET reference = psaas_order_reference() WHERE reference IS NULL;
ALTER TABLE orders ALTER COLUMN reference SET DEFAULT psaas_order_reference(),
  ALTER COLUMN reference SET NOT NULL,
  ADD CONSTRAINT orders_reference_key UNIQUE (reference),
  ADD CONSTRAINT orders_reference_format CHECK (reference ~ '^[0-9A-HJKMNP-TV-Z]{10}$');

-- Portal magic links can be revoked (revokePortalLinks) and only one link per customer is
-- active: creating a new link revokes the previous ones.
ALTER TABLE portal_tokens ADD COLUMN revoked_at timestamptz;
CREATE INDEX portal_tokens_active_idx ON portal_tokens (tenant_id, customer_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS portal_tokens_active_idx;
ALTER TABLE portal_tokens DROP COLUMN IF EXISTS revoked_at;
ALTER TABLE orders DROP COLUMN IF EXISTS reference;
DROP FUNCTION IF EXISTS psaas_order_reference();
