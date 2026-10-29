-- +goose Up
-- SPRINT-01-SECURITY L2: access JWTs carry tv = users.token_version. Bumping it (logoutAll,
-- password change, member removal, role change) makes every older access token invalid
-- at once instead of after its 15-minute expiry.
ALTER TABLE users ADD COLUMN token_version integer NOT NULL DEFAULT 0 CHECK (token_version >= 0);

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS token_version;
