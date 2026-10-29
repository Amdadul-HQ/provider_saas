-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Tenant = a provider business.
CREATE TABLE tenants (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name          text NOT NULL,
  slug          citext NOT NULL UNIQUE,
  plan          text NOT NULL DEFAULT 'free' CHECK (plan IN ('free','starter','pro','enterprise')),
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deleted')),
  currency      char(3) NOT NULL DEFAULT 'BDT',
  timezone      text NOT NULL DEFAULT 'Asia/Dhaka',
  locale        text NOT NULL DEFAULT 'bn',
  settings      jsonb NOT NULL DEFAULT '{}'::jsonb,
  trial_ends_at timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER tenants_updated BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Users = humans who log in (owners, staff, resellers, platform admins).
CREATE TABLE users (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email          citext UNIQUE,
  phone          text UNIQUE,
  password_hash  text,
  name           text NOT NULL,
  avatar_url     text,
  is_platform_admin boolean NOT NULL DEFAULT false,
  last_login_at  timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (email IS NOT NULL OR phone IS NOT NULL)
);
CREATE TRIGGER users_updated BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE tenant_members (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role            text NOT NULL CHECK (role IN ('owner','admin','staff','reseller','viewer')),
  commission_bps  integer NOT NULL DEFAULT 0 CHECK (commission_bps BETWEEN 0 AND 10000),
  status          text NOT NULL DEFAULT 'active' CHECK (status IN ('invited','active','disabled')),
  invited_by      uuid REFERENCES users(id),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, user_id)
);
CREATE INDEX tenant_members_user_idx ON tenant_members(user_id);
CREATE TRIGGER tenant_members_updated BEFORE UPDATE ON tenant_members FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE refresh_tokens (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  text NOT NULL UNIQUE,
  user_agent  text,
  ip          inet,
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens(user_id);

-- Encrypted secrets: social tokens, master-account credentials. Never plaintext.
CREATE TABLE vault_secrets (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('social_token','account_credential','api_key','other')),
  ciphertext   bytea NOT NULL,
  nonce        bytea NOT NULL,
  key_version  integer NOT NULL DEFAULT 1,
  created_by   uuid REFERENCES users(id),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX vault_secrets_tenant_idx ON vault_secrets(tenant_id);
CREATE TRIGGER vault_secrets_updated BEFORE UPDATE ON vault_secrets FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE audit_logs (
  id          bigserial PRIMARY KEY,
  tenant_id   uuid REFERENCES tenants(id) ON DELETE CASCADE,
  user_id     uuid REFERENCES users(id) ON DELETE SET NULL,
  action      text NOT NULL,
  entity_type text NOT NULL,
  entity_id   uuid,
  meta        jsonb NOT NULL DEFAULT '{}'::jsonb,
  ip          inet,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_tenant_idx ON audit_logs(tenant_id, created_at DESC);
CREATE INDEX audit_logs_entity_idx ON audit_logs(entity_type, entity_id);

-- Postgres-backed job queue (ADR-006).
CREATE TABLE jobs (
  id          bigserial PRIMARY KEY,
  tenant_id   uuid REFERENCES tenants(id) ON DELETE CASCADE,
  kind        text NOT NULL,
  payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
  run_at      timestamptz NOT NULL DEFAULT now(),
  attempts    integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL DEFAULT 5,
  locked_at   timestamptz,
  locked_by   text,
  last_error  text,
  done_at     timestamptz,
  dedupe_key  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_ready_idx ON jobs(run_at) WHERE done_at IS NULL AND locked_at IS NULL;
CREATE UNIQUE INDEX jobs_dedupe_idx ON jobs(dedupe_key) WHERE dedupe_key IS NOT NULL AND done_at IS NULL;

CREATE TABLE notifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id    uuid REFERENCES users(id) ON DELETE CASCADE,
  kind       text NOT NULL,
  title      text NOT NULL,
  body       text,
  link       text,
  read_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications(tenant_id, user_id, read_at, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS notifications, jobs, audit_logs, vault_secrets, refresh_tokens, tenant_members, users, tenants CASCADE;
DROP FUNCTION IF EXISTS set_updated_at();
