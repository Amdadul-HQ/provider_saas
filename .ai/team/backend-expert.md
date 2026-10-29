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
- (none yet)
