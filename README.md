# provider_saas

Multi-tenant SaaS ERP for digital-subscription sellers (Facebook page / WhatsApp / Telegram based providers in Bangladesh). One system to track every social-media customer, sell slots on shared accounts, verify bKash/Nagad payments, invoice, remind, renew, and reply from a unified inbox.

- Backend: Go, GraphQL (gqlgen), PostgreSQL 17 — `backend/`
- Frontend: Next.js 15, shadcn/ui, urql — `frontend/`
- AI engineering team memory — `.ai/` (read `CLAUDE.md` first)

## Quick start

```bash
cp .env.example .env            # edit secrets
make up                         # postgres + redis (or use a local Postgres 17)
make migrate && make seed       # schema + demo tenant (demo@provider.test / Demo1234!)
make backend-dev                # http://localhost:8080/graphql  (playground: /playground)
make worker-dev                 # reminders, expiry checks, post publishing
make frontend-dev               # http://localhost:3000
```

## Docs
- [Architecture](docs/architecture.md)
- [Database design](docs/database.md)
- [Integrations & webhooks](docs/integrations.md)
- [Product goal](.ai/goal.md) · [Requirements](.ai/requirements/README.md) · [Decisions](.ai/decisions/ADR-INDEX.md)
