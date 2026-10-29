# Sprint 2 — Hardening + growth (planned)
Goal: close deferred security Lows, finish integrations, add the "hook" features from the goal doc.

| # | Task | Owner | Source | Status |
|---|------|-------|--------|--------|
| 1 | httpOnly refresh-token cookie (backend + frontend) | backend+frontend | SEC M6 | built 2026-10-29 (resolvers + middleware wired, TestRefreshCookieFlow); extended by ADR-011: access token also an httpOnly cookie (psaas_at), web app holds no token; needs QA + security sign-off |
| 2 | Register enumeration + separate derived keys (HKDF) + token version revocation | backend | SEC L1, L2, L9 | built (bf678f0); VersionCache wired into container 2026-10-29; needs QA + security sign-off |
| 3 | Vault AAD per secret + key rotation command | backend | SEC L3 | todo |
| 4 | Composite (tenant_id,id) FKs; invoice item ownership check | backend | SEC L4 | todo |
| 5 | Random public order reference; portal rate limit + revoke | backend | SEC L6 | todo |
| 6 | Server-side URL/theme validation for storefront | backend | SEC L7 | todo |
| 7 | SMS dedupe; disconnected-account ingest rejection for Meta | backend | SEC I2 | todo |
| 8 | WhatsApp template messages outside 24h window | backend | REQ-002 | todo |
| 9 | Inbox auto-reply rules (FAQ bot) | backend+frontend | goal §2 | todo |
| 10 | `POST /uploads` + media in posts/products | backend+frontend | REQ-005 | todo |
| 11 | Bengali font in price-list PNG | backend | QA | todo |
| 12 | WhatsApp/Messenger chat-export import (AI parse) | backend+frontend | goal §1 | todo |
| 13 | Reseller sub-agent panel + commission report | all | goal §3 | todo |
| 14 | Win-back / upsell campaigns | all | goal §3 | todo |
| 15 | Real-time inbox (SSE) instead of polling | backend+frontend | FE note | todo |
| 16 | Playwright e2e suite in CI; GitHub Actions (vet/test/govulncheck/pnpm audit) | qa | process | todo |
