# ADR-008 RLS that actually isolates: app role, FORCE, fail-closed, lookup policies
**Status**: accepted · 2026-10-29 · amends ADR-002 · fixes SPRINT-01-SECURITY H1 (L-022)

**Decision**
- The API and worker connect as `psaas_app` (NOSUPERUSER, NOBYPASSRLS, owns nothing). Migrations run as the
  table owner via `MIGRATE_DATABASE_URL` (superuser in dev; a dedicated owner role with BYPASSRLS in prod).
  Migration 00009 creates `psaas_app` NOLOGIN with grants + default privileges; prod creates its own LOGIN
  role and `GRANT psaas_app TO <login role>`. Dev: `make db-setup` / docker init script make it LOGIN.
- Every tenant table: `ENABLE` + `FORCE ROW LEVEL SECURITY`, policy
  `tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid` for USING and WITH CHECK.
  No setting => no rows (fail-closed). Exempt platform tables: `tenants`, `users`, `refresh_tokens`, `jobs`.
- `db.WithTenantTx` sets `app.tenant_id`; `db.WithPlatformTx` (was `WithTx`) is for platform tables only;
  `db.PlatformConn` (was `Conn`) is only used for `jobs`. Worker handlers open their tenant tx from the
  payload and the worker checks that payload tenant == job tenant.
- Unauthenticated paths that must find a row before the tenant is known use **key-scoped lookup policies**
  through `db.WithLookup(key, value)`: `app.lookup_user_id` (own memberships), `app.lookup_social_id` /
  `app.lookup_social_key` (webhook routing), `app.lookup_slug` (public storefront), `app.lookup_token_hash`
  (portal), `app.lookup_invite_hash` (invites). They expose only the row whose key the caller already holds.
- Platform audit rows (`tenant_id IS NULL`, failed logins) are insert-only for the API role.
- Integration tests run as `psaas_app` and assert 0 rows without `app.tenant_id`.

**Rejected**: a BYPASSRLS "platform" role used by webhooks/jobs (one leaked code path reads everything);
SECURITY DEFINER functions (owner is subject to FORCE RLS unless superuser; harder to review).
