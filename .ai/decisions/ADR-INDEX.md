# ADR index
| # | Decision | Status |
|---|----------|--------|
| [ADR-001](ADR-001-stack.md) | Go + gqlgen backend, Next.js + shadcn frontend, PostgreSQL | accepted |
| [ADR-002](ADR-002-multitenancy.md) | Shared DB, tenant_id on every row, RLS + repo guard | accepted |
| [ADR-003](ADR-003-graphql-first.md) | GraphQL is the only API; schema drives UI | accepted |
| [ADR-004](ADR-004-money-and-secrets.md) | Money as bigint minor units; secrets in AES-GCM vault | accepted |
| [ADR-005](ADR-005-module-layout.md) | Module-per-domain in both apps; cross-module via service interfaces | accepted |
| [ADR-006](ADR-006-jobs.md) | Postgres-backed job queue polled by worker; no external broker in v1 | accepted |
| [ADR-007](ADR-007-role-matrix.md) | Role matrix: roles on every Mutation, reseller row scoping, viewer read-only | accepted |
| [ADR-008](ADR-008-rls-app-role.md) | RLS with non-owner app role, FORCE, fail-closed policy, key-scoped lookup policies | accepted |
| [ADR-009](ADR-009-design-language.md) | UI design language v2 "Nodi": tokens, Manrope + Hind Siliguri, Today hero, floating dock, motion rules | accepted |
