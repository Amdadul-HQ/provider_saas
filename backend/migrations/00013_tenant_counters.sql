-- +goose Up
-- SPRINT-01-QA BUG-11 / SECURITY L6: human-facing numbers (invoice, ticket, order) are
-- contiguous per tenant. db.NextNumber does INSERT .. ON CONFLICT DO UPDATE .. RETURNING,
-- which row-locks (tenant, kind) until the caller's transaction ends (gap-free on rollback).
-- The old global sequences stay so the down migrations of 00004/00006/00007 keep working.
CREATE TABLE tenant_counters (
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  kind       text NOT NULL CHECK (kind IN ('invoice','ticket','order')),
  value      bigint NOT NULL DEFAULT 0 CHECK (value >= 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, kind)
);

-- Backfill from existing rows so new numbers never collide with old ones in the same tenant
-- (invoice numbers are INV-YYYYMM-NNNN; the counter continues after the largest NNNN).
INSERT INTO tenant_counters (tenant_id, kind, value)
SELECT tenant_id, 'invoice', max(coalesce(substring(number from '(\d+)$'), '0')::bigint) FROM invoices GROUP BY tenant_id;
INSERT INTO tenant_counters (tenant_id, kind, value)
SELECT tenant_id, 'ticket', max(number) FROM tickets GROUP BY tenant_id;
INSERT INTO tenant_counters (tenant_id, kind, value)
SELECT tenant_id, 'order', max(number) FROM orders GROUP BY tenant_id;

ALTER TABLE tenant_counters ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_counters FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_counters
  USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE IF EXISTS tenant_counters;
