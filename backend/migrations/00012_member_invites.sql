-- +goose Up
-- SPRINT-01-QA BUG-02/BUG-03, SECURITY M4: invitations are accepted explicitly through a
-- one-time link. Only the SHA-256 of the token is stored; invite_name is what the inviter
-- typed (shown instead of an existing user's real profile until accepted).
ALTER TABLE tenant_members
  ADD COLUMN invite_token_hash text,
  ADD COLUMN invite_expires_at timestamptz,
  ADD COLUMN invite_name text;
CREATE UNIQUE INDEX tenant_members_invite_token_key ON tenant_members (invite_token_hash) WHERE invite_token_hash IS NOT NULL;

-- Public invite preview / accept resolve a token hash before any tenant is known.
CREATE POLICY member_lookup_by_invite ON tenant_members FOR SELECT
  USING (invite_token_hash = nullif(current_setting('app.lookup_invite_hash', true), ''));

-- +goose Down
DROP POLICY IF EXISTS member_lookup_by_invite ON tenant_members;
DROP INDEX IF EXISTS tenant_members_invite_token_key;
ALTER TABLE tenant_members DROP COLUMN IF EXISTS invite_token_hash, DROP COLUMN IF EXISTS invite_expires_at,
  DROP COLUMN IF EXISTS invite_name;
