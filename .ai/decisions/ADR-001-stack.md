# ADR-001 Stack
**Context**: need fast, typed, cheap-to-host backend and a mobile-first UI with reusable components.
**Decision**: Go 1.25+ (chi, gqlgen, pgx v5, goose, golang-jwt, argon2id), PostgreSQL 17, Redis (cache/rate limit), Next.js 15 App Router + TypeScript + Tailwind v4 + shadcn/ui + urql + graphql-codegen.
**Consequences**: one GraphQL contract; generated types on both sides; `make gen` must run after any schema change.
