# Lessons (append-only; newest at bottom)

Format: `## L-<n> · <date> · <area>` then What broke / Root cause / Fix / Prevention rule.
Grep this file for your area before starting a task.

## L-001 · 2026-10-29 · process
- What broke: nothing yet — seed entry.
- Root cause: —
- Fix: —
- Prevention rule: every agent logs errors here with `scripts/ai/log-lesson.sh`; never fix silently.

## L-002 · 2026-10-29 · infra
- What broke: docker run failed: daemon not running on this Mac
- Root cause: Docker Desktop is not always started; Homebrew postgresql@17 is running on :5432
- Fix: Validated migrations against local Homebrew Postgres (psql -U $USER)
- Prevention rule: Before using docker, run 'docker info' ; fall back to local Postgres (pg_isready) for DB checks; docker compose is optional in dev

## L-003 · 2026-10-29 · backend/graphql
- What broke: gqlgen generated an empty model.Subscription struct and stream resolvers (<-chan) for every Subscription field
- Root cause: GraphQL treats an object type named Subscription as the subscription ROOT operation type when there is no explicit schema{} definition
- Fix: Added 'schema { query: Query mutation: Mutation }' to graph/schema/base.graphql (no fields changed) and regenerated
- Prevention rule: Never name a business type Query/Mutation/Subscription without an explicit schema{} block; after gqlgen generate grep resolvers for '<-chan' to catch accidental root types

## L-004 · 2026-10-29 · frontend
- What broke: shadcn init -d scaffolded the base-nova style (Base UI, not Radix) and no 'form' component exists in the nova registries
- Root cause: shadcn CLI v4 defaults to --preset base-nova; the legacy RHF 'form' component was replaced by 'field'
- Fix: Re-ran 'shadcn init -b radix -p nova -f'; built RHF Controller wrappers on top of the 'field' component in components/shared/form-fields.tsx
- Prevention rule: Init shadcn with '-b radix' explicitly; for forms use 'field' + react-hook-form Controller, never expect 'form'

## L-005 · 2026-10-29 · frontend
- What broke: Bengali sentence-ending dandas (।) vanished from bn.ts after a cleanup regex
- Root cause: Stripped the whole Devanagari block U+0900-097F to remove a typo, but the danda U+0964/U+0965 lives in that block and is shared by Bengali
- Fix: Restored the affected lines from the pre-edit output
- Prevention rule: Never strip U+0900-097F from Bengali text; exclude U+0964-0965 when hunting stray Devanagari

## L-006 · 2026-10-29 · frontend
- What broke: tsc failed: cannot find module @graphql-typed-document-node/core in src/gql
- Root cause: graphql-codegen client-preset output imports it but it is not a transitive dep under pnpm strict node_modules
- Fix: pnpm add -D @graphql-typed-document-node/core
- Prevention rule: With client-preset always install @graphql-typed-document-node/core explicitly

## L-007 · 2026-10-29 · frontend
- What broke: tsc: zodResolver Resolver type not assignable to useForm<Values> in create-invoice-dialog
- Root cause: zod v4 .refine() with a type-predicate callback ((c) => c !== null) narrows the OUTPUT type, so schema input != output and RHF generics diverge
- Fix: Moved the null check to an object-level .refine(..., { path: ['customer'] }) that returns boolean without narrowing
- Prevention rule: In RHF+zod forms keep input==output: no type-predicate refines, transforms or defaults on fields; validate cross-field/nullness with object-level refine

## L-008 · 2026-10-29 · backend/seed
- What broke: api seed failed half-way ('cannot deliver a expired subscription') leaving a registered demo user, so re-running seed skipped as 'already seeded'
- Root cause: Seed called MarkDelivered on subscriptions created already expired, and seed was not transactional
- Fix: Only deliver live subscriptions; wrap the whole seed in db.WithTx (services join the ctx tx) so it is all-or-nothing
- Prevention rule: Seeds/bulk operations run in one transaction via db.WithTx; respect domain state rules (IsLive) in fixtures

## L-009 · 2026-10-29 · frontend
- What broke: tsc: Property 'payload' does not exist on TooltipProps in recharts custom tooltip
- Root cause: recharts v3 split TooltipProps (component props) from TooltipContentProps (what a custom content renderer receives)
- Fix: Typed custom tooltip as Partial<TooltipContentProps<number,string>>
- Prevention rule: With recharts >=3 type custom tooltip/legend renderers with *ContentProps, not TooltipProps

## L-010 · 2026-10-29 · backend/subscription
- What broke: Seeded data showed all 21 slots assigned although 2 should be free
- Root cause: createSubscription allocated a slot for back-dated subscriptions whose computed status was already 'expired'; expire_check only releases slots of live (pending/active/expiring) rows, so those slots leaked forever
- Fix: Only allocate a slot when StatusAt(starts, expires, now) is live
- Prevention rule: After seeding, sanity-check invariants with SQL (free+assigned slots vs live subscriptions); every resource acquisition must have a matching release path for every status it can be created in

## L-011 · 2026-10-29 · frontend
- What broke: eslint: Definition for rule 'react-hooks/incompatible-library' was not found
- Root cause: Added a disable comment for a React-Compiler-era rule; eslint-config-next 15 ships eslint-plugin-react-hooks v5 which lacks it, and unknown rules in disable comments are errors
- Fix: Removed the directive; ignored generated src/gql/** in eslint.config.mjs
- Prevention rule: Only write eslint-disable for rules that exist in the installed plugin version; run pnpm lint right after adding one

## L-012 · 2026-10-29 · backend/graphql
- What broke: Malformed UUID argument returned extensions.code INTERNAL_SERVER_ERROR (message masked in prod)
- Root cause: gqlgen's built-in graphql.UnmarshalUUID returns a plain error, which the error presenter cannot tell apart from a server fault
- Fix: Bound UUID/Time/Money scalars to graph/scalars (wrappers returning apperr.Validation) in gqlgen.yml
- Prevention rule: Wrap every custom scalar's Unmarshal in a validation error; curl a malformed id after adding a scalar

## L-013 · 2026-10-29 · backend/graphql
- What broke: graph package did not compile: r.Customers.List undefined (type func ...)
- Root cause: queryResolver/mutationResolver embed *Resolver; a query field named customers/subscriptions generates a method Customers() that shadows a Resolver field of the same name
- Fix: Renamed Resolver service fields to <Module>Svc (CustomerSvc, SubscriptionSvc, ...)
- Prevention rule: Never name graph.Resolver fields after GraphQL root fields; use the Svc suffix

## L-014 · 2026-10-29 · backend/build
- What broke: go build failed with 'no required module provides package github.com/jackc/pgx/v5' right after go get
- Root cause: gqlgen generate runs go mod tidy, which dropped the freshly added but not-yet-imported dependencies
- Fix: Re-ran go mod tidy after the code importing them existed
- Prevention rule: Add deps with go get only after writing the importing code (or run go mod tidy after each gqlgen generate)

## L-015 · 2026-10-29 · tooling
- What broke: curl verification script sent 'invalid UUID length: 73' for productId
- Root cause: zsh does not word-split unquoted $VAR, so 'set -- $PAIR' put both ids into one argument
- Fix: Wrote verification scripts as bash files (#!/usr/bin/env bash, read A B < <(...))
- Prevention rule: Write multi-step shell verification as explicit bash scripts; do not rely on word splitting in the interactive zsh

## L-016 · 2026-10-29 · backend/tests
- What broke: TestRenewAndExpireCheck expected the renewal to be ACTIVE after expire_check at now+31d, got EXPIRING
- Root cause: Test expectation ignored the 3-day expiring window: the renewal ends ~1 day after the simulated time
- Fix: Fixed the expectation (EXPIRING) with a comment
- Prevention rule: When simulating time in tests, compute expected statuses from StatusAt rules rather than intuition

## L-017 · 2026-10-29 · frontend
- What broke: Mocked dashboard queries never reached the POST handler; urql sent them as GET with the query in the URL
- Root cause: @urql/core >=4 defaults preferGetMethod to 'within-url-limit'; gqlgen only serves GET when transport.GET is added, and GET puts variables (phones, TrxIDs) in access logs
- Fix: Set preferGetMethod: false in makeClient so every operation is a POST
- Prevention rule: Always set preferGetMethod explicitly in the urql client and keep it consistent with the backend's enabled transports

## L-018 · 2026-10-29 · frontend
- What broke: Inbox thread overflowed horizontally at 375px (header controls, bubbles and send button clipped)
- Root cause: Inbox wrapper grid had no explicit mobile column, so the implicit auto track sized to the header's min-content
- Fix: grid-cols-1 + minmax(0,1fr) tracks and min-w-0 on panes; status select narrower on mobile
- Prevention rule: Every multi-pane grid declares explicit minmax(0,1fr) tracks and min-w-0 children; verify scrollWidth==375 in the browser

## L-019 · 2026-10-29 · backend/seed
- What broke: api seed on a fresh DB failed: 'pending payment 29: invoice is already paid'
- Root cause: seed picked the customer's newest invoice with Page{First:1}, but every row in the single seed tx has the same created_at (now() = tx start), so ORDER BY created_at DESC, id DESC returned a random (already paid) invoice; latent flake since the first seed
- Fix: Look up the invoice through its invoice_items.subscription_id (invoiceOfSubscription)
- Prevention rule: Never rely on created_at ordering for rows created in the same transaction (now() is constant per tx); select by a real foreign key instead

## L-020 · 2026-10-29 · backend/storefront
- What broke: TestPortalTokenValidThenExpired: portal view returned 0 invoices for a customer with an unpaid invoice
- Root cause: Portal filtered out draft invoices, but subscription.Create(CreateInvoice) issues invoices as draft and already books the ledger debit, so the customer's due balance had no visible invoice
- Fix: Portal hides only void invoices
- Prevention rule: Customer-facing views must be consistent with the ledger: anything that moved due_balance (draft included) must be visible

## L-021 · 2026-10-29 · tooling
- What broke: A Bash step hung for 120s and had to be killed
- Root cause: A stray 'cat > /dev/null' in a chained command waited on stdin forever
- Fix: Stopped the task and re-ran the real command alone
- Prevention rule: Never leave stdin-reading commands (cat without file, read) in non-interactive chains; review heredoc tails before running
