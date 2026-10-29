# Sprint 1 — Foundation (2026-10-29 →)
Goal: runnable skeleton end to end — DB, GraphQL, auth, tenant, customers, products, subscriptions, invoices, payments, inbox skeleton, storefront skeleton.

| # | Task | Owner | Status |
|---|------|-------|--------|
| 1 | AI memory + team + workflow | orchestrator | done |
| 2 | Database design + migrations | backend-expert | done |
| 3 | GraphQL contract (all modules) | team-lead | done |
| 4 | Go backend: platform + modules compile & serve | backend-expert | done |
| 5 | Next.js: shell, auth, module pages with shadcn | frontend-expert | done |
| 6 | Docker compose, Makefile, README | orchestrator | done |
| 7 | QA pass: build, smoke, tenant isolation | qa-tester | done — 53 PASS; 4 P1 + 11 P2 fixed, rest in Sprint 2 (see SPRINT-01-QA.md) |
| 8 | Security pass: auth, vault, webhooks, RLS | security-expert | done — 6 High + 9 Medium fixed; Lows deferred (see SPRINT-01-SECURITY.md) |

## Sprint close (2026-10-29)
- End-to-end verified by orchestrator on fixed code: login, wrong-password message, dashboard, inbox, storefront, invite → accept → staff login, role gates, SMS token rotate, lockout, alias limit.
- All gates green: go vet/build/test, integration tests as `psaas_app`, govulncheck clean on Go 1.27.3, pnpm typecheck/lint/test(29)/build, pnpm audit 0.
- Carry-over → [SPRINT-02](SPRINT-02.md).
