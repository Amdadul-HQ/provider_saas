# Backend Expert (Go)

## Mission
Write idiomatic, fast, secure Go: small interfaces, explicit errors, context everywhere, no globals.

## Responsibilities
- One module = domain.go, repository.go (pgx, parameterised SQL, tenant_id always), service.go (rules), resolver.go (thin).
- Migrations via goose, reversible, one concern per file.
- Table-driven tests for services; integration tests against docker postgres with `-tags integration`.
- Run `go vet ./... && go build ./... && go test ./...` before done.

## Before starting any task
1. Read `.ai/MEMORY.md`, `.ai/goal.md`, `.ai/workflow.md`.
2. `grep -i "<your area>" .ai/errors/LESSONS.md` and obey every prevention rule.
3. Read the sprint task; if unclear, write your assumption in the task row instead of guessing silently.

## Definition of done
- Build/vet/test green; new lessons logged; module has schema, resolvers wired in container.

## Learned patterns (append, ≤3 lines each)
- Tenant tx in ctx: `db.WithTenantTx(ctx, pool, tid, fn(ctx, tx))` stores the tx in ctx; nested service calls (subscription -> billing -> customer) join it, so a cross-module sale is atomic. `SET LOCAL` can't take $1 — use `set_config('app.tenant_id', $1, true)`.
- Slot allocation: `SELECT ... FOR UPDATE OF s SKIP LOCKED LIMIT 1` + assign in the same tx; release only `WHERE slot.subscription_id = this` so renewals can transfer slots safely. Proven by a concurrent integration test (8 buyers, 3 slots).
- gqlgen: bind types that need FK ids to hand-written structs in graph/model/bound.go (gqlgen.yml models:), keep resolvers thin, batch every forceResolver field through per-request dataloaders; wrap scalar Unmarshal errors as apperr.Validation.
- List queries (ADR-010): `filter(ctx, tid, f) *sqlb.Query` shared by List and Count; `sqlb.Eq` for optional fields, `q.Search(f.Search, sqlb.Like(..), sqlb.Related(..), sqlb.Exact(..))`, then `q.Page(page, keyset)` (fetches First+1) and `pagination.Trim` in the service. Resolvers only ever call `pageOf`.
- Domain errors: return apperr.NotFound/Validation/Conflict/Forbidden from services; db.MapErr maps pgx.ErrNoRows/23505/23503; the GraphQL presenter turns them into extensions.code and masks unknown errors outside dev.
- Money ledger: every due change writes a ledger_entries row AND adjusts customers.due_balance in the same tx; integration tests assert sum(ledger) == cached due.
- Webhooks: verify first (X-Hub-Signature-256 HMAC over the RAW body, constant-time compare, fail closed when the secret is empty), then parse with pure functions (`social.ParseMetaPayload`, `ParseTelegramUpdate`) that are table-tested with real payloads; answer 200 for unknown accounts/garbage, 500 only for transient ingest errors (platforms retry; ingest is idempotent on message external id).
- Outbound I/O never inside a DB tx: queue an `inbox.send` job in the caller's tx; when the caller owned the tx, run it right after commit with `jobs.Worker.RunByID` (atomic claim, so the poller can't double-send). Jobs decide retry vs final via `permanent(err)` + `j.Attempts >= j.MaxAttempts`.
- gqlgen JSON that may be an array: list two models for the scalar (`graphql.Map` first, `graphql.Any` second) and type bound-struct fields `any`; generated structs keep `map[string]any`. Keep helpers out of `*.resolvers.go` (gqlgen comments out unknown code on regenerate).
- Plain-list paging (tickets/posts/orders/reminders): `page.after` = last row id, keyset `(created_at,id) < (SELECT created_at,id FROM t WHERE id=$after)`. Rows created in one tx share `now()`, so never pick "the newest" by created_at — select by FK.
- Secrets for machine use (`vault.Open`, no audit) vs humans (`vault.Reveal`, audited); when a secret is rendered into an outbound message write an explicit audit row (`delivery.render`) and store only a masked note.
- RLS (ADR-008): the API connects as `psaas_app` (no superuser/BYPASSRLS), migrations via `MIGRATE_DATABASE_URL`. Tenant tables are FORCE + fail-closed: outside `WithTenantTx` they return 0 rows. Pre-tenant lookups use `db.WithLookup(db.LookupSlug|LookupSocialKey|..., key, fn)` (key-scoped policy), never a bypass role. Every new tenant table: ENABLE + FORCE + `tenant_isolation` policy in its migration.
- Roles (ADR-007): every new Mutation needs `@auth(roles: [...])` from the matrix or `graph/schema_auth_test.go` fails; reseller row scoping lives in repositories via `pauth.ResellerScope(ctx)`; money verification also goes through `requirePaymentVerifier` (staff_can_verify_payments).
- Secrets in outbound messages: store the redacted text, put the real text in the job payload with `vault.SealTransient(tenant, messageID, ...)` (`jobs.SealedPayloadKey` is scrubbed on finish). Upstream HTTP errors go through `social.SafeError` before being returned/stored/logged.
- Human numbers: `db.NextNumber(ctx, tx, tenant, "invoice"|"ticket"|"order")` (row-locked counter, gap-free). Tests that mutate shared dev DBs use the same VAULT_KEY as seed (`config.DevVaultKey`).
