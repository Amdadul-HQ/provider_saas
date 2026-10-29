-- Dev/test only: the API's database role (ADR-002 addendum, SECURITY H1).
-- psaas_app is NOSUPERUSER NOBYPASSRLS and owns no table, so row-level security applies.
-- Run as the table owner (Homebrew: your OS user; docker: POSTGRES_USER) on the dev DB:
--   psql -d psaas_dev -f backend/scripts/dev-db-role.sql      (make db-setup does this)
-- Production: never use this password. Create a LOGIN role with a secret password and
-- `GRANT psaas_app TO <that role>` (migration 00009 creates psaas_app NOLOGIN + grants).
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'psaas_app') THEN
    CREATE ROLE psaas_app LOGIN PASSWORD 'psaas_app' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
  ELSE
    ALTER ROLE psaas_app LOGIN PASSWORD 'psaas_app' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
  END IF;
END $$;
GRANT USAGE ON SCHEMA public TO psaas_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO psaas_app;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO psaas_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO psaas_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO psaas_app;
