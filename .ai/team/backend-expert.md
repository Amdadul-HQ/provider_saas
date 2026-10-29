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
- Domain errors: return apperr.NotFound/Validation/Conflict/Forbidden from services; db.MapErr maps pgx.ErrNoRows/23505/23503; the GraphQL presenter turns them into extensions.code and masks unknown errors outside dev.
- Money ledger: every due change writes a ledger_entries row AND adjusts customers.due_balance in the same tx; integration tests assert sum(ledger) == cached due.
