-- +goose Up
-- SPRINT-01-SECURITY H1 / ADR-002 addendum: make row-level security actually protect data.
--
-- * The API connects as `psaas_app` (NOSUPERUSER, NOBYPASSRLS, does not own any table).
--   Migrations run as the owner role (superuser in dev; in production a dedicated owner
--   role with BYPASSRLS). This migration creates `psaas_app` as NOLOGIN; production then
--   creates a LOGIN role (e.g. `psaas_api`) and runs `GRANT psaas_app TO psaas_api`.
--   Dev/test: `make db-setup` (Homebrew) or docker/postgres-init.sql make it LOGIN.
-- * Every tenant table has FORCE ROW LEVEL SECURITY, so even the table owner is filtered.
-- * The policy is fail-closed: no `app.tenant_id` (NULL or '') => no rows.
-- * Platform tables are exempt (no tenant_id or not tenant-owned): tenants, users,
--   refresh_tokens, jobs, goose_db_version.
-- * Narrow lookup policies let unauthenticated platform paths read exactly the row whose
--   key they already hold (webhook routing, public storefront slug, portal token, a user's
--   own memberships). The key is passed with set_config('app.lookup_*', <key>, true) by
--   db.WithLookup; without it those policies match nothing.

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'psaas_app') THEN
    CREATE ROLE psaas_app NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
  END IF;
EXCEPTION WHEN insufficient_privilege THEN
  RAISE NOTICE 'psaas_app role could not be created by %; create it manually (see docs/database.md)', current_user;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'psaas_app') THEN
    BEGIN
      -- A pre-existing role must never be able to bypass the policies below.
      ALTER ROLE psaas_app NOSUPERUSER NOBYPASSRLS;
    EXCEPTION WHEN insufficient_privilege THEN
      RAISE NOTICE 'could not ALTER ROLE psaas_app; verify it is NOSUPERUSER NOBYPASSRLS';
    END;
    BEGIN
      EXECUTE format('GRANT CONNECT ON DATABASE %I TO psaas_app', current_database());
    EXCEPTION WHEN insufficient_privilege THEN
      RAISE NOTICE 'could not GRANT CONNECT to psaas_app (PUBLIC usually has it)';
    END;
    GRANT USAGE ON SCHEMA public TO psaas_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO psaas_app;
    GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO psaas_app;
    -- goose bookkeeping is read-only for the API.
    REVOKE INSERT, UPDATE, DELETE ON goose_db_version FROM psaas_app;
    -- Tables/sequences created by later migrations (run by this same owner role).
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO psaas_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO psaas_app;
  END IF;
END $$;
-- +goose StatementEnd

-- jobs is a platform table (the worker claims across tenants; handlers then open a
-- tenant transaction from the job's tenant). It leaves RLS.
DROP POLICY IF EXISTS tenant_isolation ON jobs;
ALTER TABLE jobs DISABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'tenant_members','vault_secrets','audit_logs','notifications',
    'customers','customer_identities','customer_notes',
    'products','product_plans','suppliers','master_accounts','slots',
    'subscriptions','invoices','invoice_items','payments','payment_inbox','ledger_entries',
    'social_accounts','conversations','messages','message_templates','posts','post_targets',
    'tickets','ticket_comments','reminder_rules','reminders',
    'storefronts','orders','portal_tokens','imports'
  ] LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$
      CREATE POLICY tenant_isolation ON %I
      USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
      WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
    $p$, t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- Platform audit rows (failed logins, lockouts) have no tenant: insert-only, never readable.
CREATE POLICY audit_platform_insert ON audit_logs FOR INSERT WITH CHECK (tenant_id IS NULL);

-- Login / switchTenant / refresh list the caller's own memberships across tenants.
CREATE POLICY member_lookup_by_user ON tenant_members FOR SELECT
  USING (user_id = nullif(current_setting('app.lookup_user_id', true), '')::uuid);

-- Webhook routing: Telegram routes carry the account id; Meta payloads carry platform + external id.
CREATE POLICY social_lookup ON social_accounts FOR SELECT
  USING (id = nullif(current_setting('app.lookup_social_id', true), '')::uuid
     OR (platform || ':' || external_id) = nullif(current_setting('app.lookup_social_key', true), ''));

-- Public storefront pages resolve a slug to its tenant.
CREATE POLICY storefront_lookup_by_slug ON storefronts FOR SELECT
  USING (slug = nullif(current_setting('app.lookup_slug', true), '')::citext);

-- Customer portal magic links resolve a token hash (and stamp used_at).
CREATE POLICY portal_lookup_select ON portal_tokens FOR SELECT
  USING (token_hash = nullif(current_setting('app.lookup_token_hash', true), ''));
CREATE POLICY portal_lookup_update ON portal_tokens FOR UPDATE
  USING (token_hash = nullif(current_setting('app.lookup_token_hash', true), ''))
  WITH CHECK (token_hash = nullif(current_setting('app.lookup_token_hash', true), ''));

-- +goose Down
DROP POLICY IF EXISTS portal_lookup_update ON portal_tokens;
DROP POLICY IF EXISTS portal_lookup_select ON portal_tokens;
DROP POLICY IF EXISTS storefront_lookup_by_slug ON storefronts;
DROP POLICY IF EXISTS social_lookup ON social_accounts;
DROP POLICY IF EXISTS member_lookup_by_user ON tenant_members;
DROP POLICY IF EXISTS audit_platform_insert ON audit_logs;
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'tenant_members','vault_secrets','audit_logs','jobs','notifications',
    'customers','customer_identities','customer_notes',
    'products','product_plans','suppliers','master_accounts','slots',
    'subscriptions','invoices','invoice_items','payments','payment_inbox','ledger_entries',
    'social_accounts','conversations','messages','message_templates','posts','post_targets',
    'tickets','ticket_comments','reminder_rules','reminders',
    'storefronts','orders','portal_tokens','imports'
  ] LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format('ALTER TABLE %I NO FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format($p$
      CREATE POLICY tenant_isolation ON %I
      USING (tenant_id IS NULL OR current_setting('app.tenant_id', true) IS NULL OR current_setting('app.tenant_id', true) = '' OR tenant_id = current_setting('app.tenant_id', true)::uuid)
      WITH CHECK (tenant_id IS NULL OR current_setting('app.tenant_id', true) IS NULL OR current_setting('app.tenant_id', true) = '' OR tenant_id = current_setting('app.tenant_id', true)::uuid)
    $p$, t);
  END LOOP;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'psaas_app') THEN
    ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM psaas_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE USAGE, SELECT, UPDATE ON SEQUENCES FROM psaas_app;
    REVOKE ALL ON ALL TABLES IN SCHEMA public FROM psaas_app;
    REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM psaas_app;
  END IF;
END $$;
-- +goose StatementEnd
