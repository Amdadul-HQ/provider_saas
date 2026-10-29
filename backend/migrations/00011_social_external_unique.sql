-- +goose Up
-- SPRINT-01-SECURITY H5: a page / Instagram account / WhatsApp number / bot can be connected
-- by at most one business at a time ("most recent connection wins" routing is gone).
-- Disconnected rows are kept (conversations reference them) and do not count.
-- Existing duplicates: keep the most recently updated live connection, disconnect the rest.
UPDATE social_accounts s SET status = 'disconnected', token_secret_id = NULL, webhook_verified = false,
  last_error = 'disconnected: connected by another business'
WHERE s.status <> 'disconnected' AND EXISTS (
  SELECT 1 FROM social_accounts o
  WHERE o.platform = s.platform AND o.external_id = s.external_id AND o.status <> 'disconnected'
    AND (o.updated_at, o.id) > (s.updated_at, s.id));
CREATE UNIQUE INDEX social_accounts_external_active_key ON social_accounts (platform, external_id)
  WHERE status <> 'disconnected';

-- +goose Down
DROP INDEX IF EXISTS social_accounts_external_active_key;
