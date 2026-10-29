# provider_saas — Claude entry point

Multi-tenant SaaS ERP for digital subscription providers (Netflix/Spotify/Canva/ChatGPT resellers who sell via Facebook pages, WhatsApp and Telegram in Bangladesh). Goal: automate their whole workflow — every social media customer tracked, every subscription/invoice/reminder handled, every channel managed from one inbox.

## AI memory system (READ FIRST, every session)

The project is run by a virtual engineering team whose shared brain lives in `.ai/`. Before doing ANY work:

1. Read `.ai/MEMORY.md` (index) → `.ai/goal.md` → `.ai/workflow.md`.
2. Read `.ai/errors/LESSONS.md`. Never repeat a recorded mistake.
3. Read `.ai/decisions/ADR-INDEX.md` so you do not re-decide settled questions.
4. Pick your role from `.ai/team/` (or be the Orchestrator if you are the top-level session).
5. For any UI work read `.ai/design-quality.md`: compact, eye-catching, no "AI slop" (no ALL-CAPS eyebrows, gradient text, card-kit sameness, stock copy, fake social proof).

Rules:
- **Every error you hit** (build failure, failing test, wrong assumption, security miss) is logged in `.ai/errors/LESSONS.md` with root cause + fix + prevention rule. Use `scripts/ai/log-lesson.sh`.
- **Every architectural choice** gets an ADR in `.ai/decisions/`.
- **Every requirement** flows: Business Analyst → `.ai/requirements/` → Project Manager breaks it down in `.ai/sprints/` → Team Lead assigns → engineers build → QA + Security sign off → Orchestrator closes the loop.
- Skill growth: when an engineer learns a better pattern, add it to `.ai/team/<role>.md` under "Learned patterns".

## Stack (settled, see ADRs)

- Backend: Go 1.25+, chi, gqlgen (GraphQL), pgx v5, goose migrations, JWT auth, AES-GCM vault. Module-per-domain under `backend/internal/modules/<module>/` (domain, repository, service, resolvers, schema.graphql).
- Frontend: Next.js 15 App Router, TypeScript, Tailwind v4, shadcn/ui, urql + graphql-codegen. Module-per-domain under `frontend/src/modules/<module>/`.
- Database: PostgreSQL 17. Every business table carries `tenant_id`; row-level isolation enforced in repositories and by RLS policies.
- Dev: `docker compose up` (postgres + redis), `make dev` in each app.

## Commands

```bash
make up            # docker compose: postgres + redis
make backend-dev   # go run ./cmd/api (auto-migrates)
make frontend-dev  # pnpm dev
make gen           # gqlgen + graphql-codegen
make test          # go test ./... && pnpm test
```

## Conventions

- GraphQL is the only API between frontend and backend. UI is driven by schema; never hardcode what the schema can express.
- Each module is self-contained and reusable; cross-module calls go through service interfaces, never through repositories of another module.
- Money is stored as `bigint` minor units (paisa) with `currency` text. Never float.
- Secrets (social tokens, account credentials) only exist encrypted in `vault_secrets`; the API never returns plaintext except to an authorised reveal mutation that is audit-logged.
- Bengali-first UX: all user-facing strings go through `frontend/src/i18n/` (bn default, en fallback).
