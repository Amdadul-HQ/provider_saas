# ADR-007 Role matrix (tenant roles and what they may do)
**Status**: accepted · 2026-10-29 · fixes SPRINT-01-SECURITY H2 (L-023)

**Context**: a bare `@auth` accepted any role, so VIEWER and RESELLER could verify payments, mark
subscriptions delivered and trigger automatic credential delivery. There were no service-level role checks.

**Decision**: every GraphQL `Mutation` field declares `@auth(roles: [...])` from this matrix. A schema
test (`backend/graph/schema_auth_test.go`) fails on a Mutation field that has no `@auth` or a bare `@auth`,
except the public and account-level fields listed below. Platform admins pass every role check.

| Area | OWNER | ADMIN | STAFF | RESELLER | VIEWER |
|---|---|---|---|---|---|
| Billing plan, ownership transfer, `updateMemberRole`, `removeMember` | yes | — | — | — | — |
| Team (`inviteMember`, `resendInvite`), `updateTenant`, `rotateSmsWebhookToken`, `auditLogs` | yes | yes | — | — | — |
| Catalog, inventory (suppliers, master accounts, slots), credential set/reveal | yes | yes | — | — | read (no reveal) |
| Social connect/disconnect/sync, storefront settings, templates, reminder rules | yes | yes | — | — | read |
| `voidInvoice`, `addLedgerAdjustment`, `extendSubscription`, `deleteCustomer`, `mergeCustomers` | yes | yes | — | — | — |
| Reverse a verified payment (`rejectPayment` on VERIFIED) | yes | yes | — | — | — |
| Customers, subscriptions (create/renew/deliver/cancel/reassign slot) | yes | yes | yes | own rows | read |
| Invoices, `recordPayment`, payment inbox, inbox/messages, tickets, orders, content, portal links | yes | yes | yes | — | read |
| Verify money (`verifyPayment`, `matchPaymentInbox`, `confirmOrder`, `markPaid` on create/renew) | yes | yes | if `settings.staff_can_verify_payments` (default true) | — | — |
| Personal (`markNotificationRead`, `markAllNotificationsRead`, `ping`) | yes | yes | yes | yes | yes |

- **Public** (no `@auth`): `register`, `login`, `refreshToken`, `logout`, `acceptInvite`, `placeOrder`,
  `portalRequestRenewal`; queries `health`, `invite`, `publicStorefront`, `publicOrder`, `portal`.
- **Account-level** (bare `@auth`, work without a tenant role): `switchTenant`, `changePassword`.
- **RESELLER rows**: repositories add `customers.owner_user_id = me`, `subscriptions.sold_by = me OR the
  customer is mine`, `orders.created_by = me` whenever the principal is a reseller (`auth.ResellerScope`).
  Resellers cannot read invoices, payments, inbox, tickets, inventory (cost/credentials), analytics,
  audit logs or storefront settings (queries carry `roles: [OWNER, ADMIN, STAFF, VIEWER]`).
- **VIEWER** is read-only; slot PINs are masked (`••••`) for everyone except OWNER/ADMIN.
- Cost fields (`purchaseCost`, plan `cost`) stay visible to VIEWER/STAFF (accepted; revisit with pricing tiers).

**Consequences**: the frontend must hide actions per role (FORBIDDEN otherwise); a reseller dashboard
needs its own queries (Sprint 2). New mutations cannot be merged without a matrix entry (the test fails).
