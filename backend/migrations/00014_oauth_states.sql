-- +goose Up
-- SPRINT-01-SECURITY M2: a Meta OAuth state is single-use and bound to the user and tenant
-- that started the flow. completeMetaConnect (authenticated) deletes the row it consumes.
CREATE TABLE oauth_states (
  nonce      text PRIMARY KEY,
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  platform   text NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX oauth_states_expiry_idx ON oauth_states (expires_at);
ALTER TABLE oauth_states ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_states FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON oauth_states
  USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE IF EXISTS oauth_states;
