# Architecture

```
Facebook / Instagram / WhatsApp / Telegram / SMS-forwarder
            │ webhooks (REST, signed)
            ▼
┌─────────────────────────── backend (Go) ───────────────────────────┐
│ cmd/api  ── chi ── /graphql (gqlgen) ── @auth directive ── modules  │
│ cmd/worker ── jobs table (SKIP LOCKED) ── handlers (reminders, …)   │
│ internal/platform: config · logger · db(pgx+RLS) · auth(JWT) ·      │
│                    vault(AES-GCM) · gql · httpx · jobs · pagination │
│ internal/modules/<m>: domain · repository · service · <m>.graphql   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ GraphQL only
┌─────────────────────────── frontend (Next.js) ─────────────────────┐
│ app/ routes compose src/modules/<m>/{components,hooks,graphql}      │
│ shadcn/ui · urql + codegen · i18n (bn/en) · PWA mobile-first        │
└─────────────────────────────────────────────────────────────────────┘
                 PostgreSQL 17 (tenant_id + RLS) · Redis (cache/ratelimit)
```

## Request flow
1. Frontend sends GraphQL with `Authorization: Bearer <access JWT>`.
2. `auth` middleware → `Principal{userID, tenantID, role}` in context.
3. `@auth(roles)` directive rejects unauthenticated / wrong role.
4. Resolver (thin) → module `Service` → `db.WithTenantTx` (sets `app.tenant_id` → RLS) → `Repository`.
5. Side effects (send message, reminder, publish post) are enqueued as jobs; the worker executes with retries.

## Module dependency rules (ADR-005)
- Modules depend on other modules' `Service` interfaces only, injected in `internal/app/container.go`.
- No cycles: `auth → tenant`; `subscription → catalog, inventory, billing`; `billing → customer`; `ticket → subscription, inventory`; `reminder → subscription, billing, inbox`; `storefront → customer, catalog, billing, subscription`; `inbox → social, customer`; `content → social`; `analytics` reads everything via SQL.

## Security
- Tenant isolation: repository `WHERE tenant_id` + Postgres RLS. Tested in `backend/internal/modules/customer/repository_integration_test.go`.
- Secrets only in `vault_secrets` (AES-256-GCM). Reveal is role-gated and audit-logged.
- Webhooks verify signatures (`X-Hub-Signature-256` for Meta, Telegram secret token header, HMAC for SMS forwarder) and are idempotent on external message id.
- Passwords argon2id; JWT 15 min access + rotating refresh tokens (hashed at rest).
