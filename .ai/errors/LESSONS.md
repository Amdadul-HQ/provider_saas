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

## L-022 · 2026-10-29 · security/db
- What broke: RLS policies exist (00008) but give no protection: the API connects as a Postgres superuser who owns the tables, and the policy allows all rows when app.tenant_id is unset
- Root cause: Superusers and table owners bypass RLS (no FORCE ROW LEVEL SECURITY, no app_user role created); the policy was written fail-open (IS NULL OR '' => true)
- Fix: Reported as SPRINT-01-SECURITY H1; fix: non-superuser NOBYPASSRLS app role, FORCE RLS, fail-closed policy
- Prevention rule: Every RLS change ships with an integration test that does SET ROLE <app role> and asserts 0 rows without app.tenant_id; never connect the API as the migration/owner role

## L-023 · 2026-10-29 · security/authz
- What broke: VIEWER and RESELLER can run verifyPayment, markDelivered, createSubscription(markPaid) and other mutations that trigger credential delivery
- Root cause: A bare @auth means any authenticated role; no service-level role checks and no role matrix
- Fix: Reported as SPRINT-01-SECURITY H2; fix: role-matrix ADR plus roles on every Mutation
- Prevention rule: Every Mutation field must declare @auth(roles:[...]); a schema test fails on a bare @auth on Mutation; VIEWER is read-only by default

## L-024 · 2026-10-29 · security/secrets
- What broke: Delivered master-account passwords are stored in plaintext in messages.body and the conversation preview; Meta and Telegram tokens and the app secret can leak through url.Error strings into GraphQL errors, redirect URLs and last_error
- Root cause: The secret is rendered into a general-purpose persisted field; secrets are put in URL query strings and Go's *url.Error includes the full URL
- Fix: Reported as SPRINT-01-SECURITY H3 and M3
- Prevention rule: Plaintext secrets may live only in the vault or in a transient encrypted job payload; never in a URL; unwrap *url.Error before returning or storing an upstream error; grep the stored/returned strings for secrets in review

## L-025 · 2026-10-29 · security/payments
- What broke: SMS auto-verification accepts any SMS that looks like bKash/Nagad text, so a customer can forge a payment SMS and get credentials delivered automatically
- Root cause: The parser trusts message content; the forwarder's sender field is ignored
- Fix: Reported as SPRINT-01-SECURITY H4; fix: sender short-code allowlist and a review state
- Prevention rule: Any automatic money verification must authenticate the source (sender ID or signed provider API), not only parse the content

## L-026 · 2026-10-29 · security/ratelimit
- What broke: Per-IP rate limits (global and placeOrder) can be bypassed with forged X-Forwarded-For, and aliased login fields multiply brute-force attempts per request
- Root cause: chi middleware.RealIP trusts client headers; the limiter counts HTTP requests, not GraphQL fields
- Fix: Reported as SPRINT-01-SECURITY H6 and M1
- Prevention rule: Trust XFF only from configured proxy CIDRs; rate-limit sensitive GraphQL fields per identifier and allow only one auth or public-write field per operation

## L-027 · 2026-10-29 · infra/vault
- What broke: QA: every seeded vault secret failed with 'vault: decryption failed' (credential reveal + all outbound messages) while new secrets worked
- Root cause: VAULT_KEY is not pinned for dev (no .env.example/Makefile default); seed and the restarted API used different keys and key_version=1 on both so nothing detected the mismatch
- Fix: Filed SPRINT-01-QA BUG-01: pin a dev-only key in .env.example/Makefile; add a key fingerprint or boot-time canary decrypt
- Prevention rule: Any encryption key used by seed and server must come from one committed dev default; after restarting the API run one reveal/decrypt smoke before handing the env to QA

## L-028 · 2026-10-29 · frontend/i18n
- What broke: QA: raw API strings reached users (EXPIRING_SOON, OVERDUE_INVOICE, free, active, password_changed, 'vault: decryption failed', 'invalid UUID'); wrong password showed 'session expired'
- Root cause: UI renders server enum/code strings and error.message directly; one generic UNAUTHENTICATED->session-expired mapping is reused on the login form
- Fix: Filed BUG-05/06/07
- Prevention rule: Every enum-like string from the API goes through an i18n map with a fallback; never render error.message to users; auth forms map their own error codes

## L-029 · 2026-10-29 · backend/db
- What broke: QA: new tenant's first invoice was INV-202610-0081 and first ticket #9
- Root cause: invoice_number_seq, ticket_number_seq and order_number_seq are global Postgres sequences shared by all tenants
- Fix: Filed BUG-11 (per-tenant counters needed)
- Prevention rule: Human-facing document numbers in a multi-tenant table use a per-tenant counter (tenant_counters row locked FOR UPDATE), never a global sequence

## L-030 · 2026-10-29 · backend/billing
- What broke: QA: verifying two pending payments made invoice paidAmount 185548 > total 135048 and customer due -50500 silently
- Root cause: recordPayment/verifyPayment never compare amount against invoice dueAmount (including other pending payments)
- Fix: Filed BUG-04
- Prevention rule: Money mutations validate against the remaining due at verify time; overpayment is either rejected or booked as an explicit advance; add a test for it

## L-031 · 2026-10-29 · infra/dev-run
- What broke: make backend-dev could not bind :8080: an orphaned 'go run ./cmd/api serve' (parent launchd, built from older code, pointed at psaas_dev) was still listening
- Root cause: A previous session started the API with a bare go run in a shell that exited; the server outlived it and nothing tracked it. Also no .env existed and the psaas role from .env.example is absent on the Homebrew Postgres
- Fix: Ran this stack on :8081 (root .env + frontend/.env.local) against a fresh local DB 'psaas' created with createdb; added .claude/launch.json (api/worker/frontend) so servers are tracked by the app
- Prevention rule: Before starting the API run 'lsof -nP -iTCP:8080 -sTCP:LISTEN'; start servers through .claude/launch.json (preview_start), never a detached go run; on this Mac use DATABASE_URL=postgres://$USER@localhost:5432/psaas?sslmode=disable

## L-032 · 2026-10-29 · frontend
- What broke: Wrong-password login showed 'session expired' and could trigger a refresh+retry of the login mutation
- Root cause: describeError mapped every UNAUTHENTICATED to the session message, and urql authExchange.didAuthError treated a public auth mutation's UNAUTHENTICATED as an expired session
- Fix: describeError(t, err, 'login'|'invite') context; didAuthError returns false for PUBLIC_OPERATIONS
- Prevention rule: Auth forms pass their own error context; public operations never go through refresh/retry in the auth exchange

## L-033 · 2026-10-29 · frontend
- What broke: Bengali digits leaked into a Latin-digit UI (dates, '৭ দিনে') while amounts stayed Latin
- Root cause: Intl bn-BD defaults to beng numerals and bn.ts hardcodes ০-৯ in static strings; only money used the digit setting
- Fix: useT() returns a digit-localised dictionary (string args masked), dates use a -u-nu-<digits> locale tag, useFormat() for every number/date
- Prevention rule: Never call Intl/formatters with a bare locale; go through useFormat() or pass digits; literal-digit strings go in KEEP_DIGITS

## L-034 · 2026-10-29 · frontend
- What broke: Overwriting app/s/[slug]/page.tsx (client) with a server page exporting generateMetadata logged 'use client' export errors in the running dev server
- Root cause: next dev compiled the file between the old 'use client' version and the rewrite; stale overlay errors persisted in the console log
- Fix: Rewrote the files in one write and confirmed with next build; ignored the stale HMR errors after re-navigating
- Prevention rule: When converting a client page to server, write the full file at once and verify with a build, not with HMR console history

## L-035 · 2026-10-29 · frontend/security
- What broke: Production CSP with upgrade-insecure-requests would rewrite the http API origin when running next start locally
- Root cause: upgrade-insecure-requests upgrades every http subresource, including http://localhost:8080 connect targets
- Fix: Dropped it; HSTS (prod only) pins the app origin instead
- Prevention rule: Do not add upgrade-insecure-requests while any environment talks to an http API; verify CSP with a prod build (NEXT_DIST_DIR=.next-build next start) before shipping

## L-036 · 2026-10-29 · infra/db
- What broke: Ran backend/scripts/dev-db-role.sql against the 'postgres' maintenance DB instead of psaas_dev, granting psaas_app on that DB's public schema
- Root cause: psql without -d uses the default DB; I forgot -d psaas_dev
- Fix: Revoked the grants/default privileges on 'postgres' and re-ran with -d psaas_dev
- Prevention rule: Always pass -d <db> explicitly to psql/createuser scripts; role scripts carry per-database GRANTs even though roles are cluster-wide

## L-037 · 2026-10-29 · tooling
- What broke: A scripted str.replace edit (ledger.adjust audit) silently did nothing; the integration test caught 0 audit rows
- Root cause: Python str.replace returns the input unchanged when the pattern (here with different indentation) is not found
- Fix: Re-applied with assert s.count(old)==1 and grepped every other scripted edit for its marker
- Prevention rule: Every scripted code edit asserts the pattern matched exactly once (assert s.count(old)==1) before replacing; grep the result

## L-038 · 2026-10-29 · backend/vault
- What broke: Integration self-check test failed: secrets of tenants created by an earlier test run (VAULT_KEY ab*32) could not be decrypted with the dev key
- Root cause: The test container used its own VAULT_KEY while seed used the committed dev key, on the same psaas_dev database - the BUG-01 mismatch reproduced by our own tests
- Fix: Tests use config.DevVaultKey (same as seed/.env.example); recreated psaas_dev
- Prevention rule: Seed, API, worker AND integration tests share one VAULT_KEY per database; never invent a per-test key against a shared DB

## L-039 · 2026-10-29 · backend/tests
- What broke: RLS integration test failed with 'commit unexpectedly resulted in rollback'
- Root cause: An INSERT that was expected to fail (WITH CHECK) aborted the surrounding Postgres transaction; the test returned nil and the commit failed
- Fix: Run the expected-failure statement in its own WithTenantTx and assert the error
- Prevention rule: In Postgres any failed statement poisons the transaction: put expected-failure statements in their own tx (or a SAVEPOINT)

## L-040 · 2026-10-29 · tooling
- What broke: curl verification script: jq --argjson got invalid JSON for queries without variables
- Root cause: bash default expansion ${3:-{\}} produced '{}}' (brace quoting inside parameter expansion)
- Fix: v=${3:-}; [ -z "$v" ] && v='{}'
- Prevention rule: Never put braces in ${var:-default}; assign the default on a separate line in verification scripts

## L-041 · 2026-10-29 · testing
- What broke: integration tests failed with 0 rows (storefronts=0, audit rows=0) when run manually
- Root cause: harness treats DATABASE_URL/TEST_DATABASE_URL as the OWNER connection and derives psaas_app itself; passing the app role as DATABASE_URL made assertion queries hit fail-closed RLS
- Fix: ran with owner URL; use 'make test-integration'
- Prevention rule: always run integration tests via make test-integration; never hand-roll env for them

## L-042 · 2026-10-29 · frontend/dev
- What broke: Browser preview of the frontend showed 'cannot reach server': the running next dev on :3000 called the API on :8080 while CSP only allowed :8081
- Root cause: next dev inlines NEXT_PUBLIC_* at start; the server predated .env.local, and a second dev server on another port is blocked by the API CORS allow-list (:3000 only)
- Fix: Added a dev-only rewrites() proxy in next.config.ts (NEXT_DEV_API_PROXY) and a 'frontend-alt' launch config on :3001 with NEXT_DIST_DIR=.next-alt that talks to the API same-origin
- Prevention rule: After changing .env.local restart next dev; for a second dev server use the frontend-alt launch config (proxy) instead of widening CORS; never run two next dev in the same distDir

## L-043 · 2026-10-29 · frontend/tooling
- What broke: pnpm lint reported 3683 problems
- Root cause: eslint walked the alternate build dir .next-alt (not in ignores) and git had it tracked from an earlier session
- Fix: Added .next-alt/** to eslint ignores and .gitignore; lint src with 'pnpm exec eslint src' when in doubt
- Prevention rule: Every alternate distDir must be listed in eslint ignores and .gitignore before first use; never commit .next-* (frontend/.next-alt is still tracked: git rm -r --cached it)

## L-044 · 2026-10-29 · tooling/preview
- What broke: launch.json entry 'sh -c "HTTP_ADDR=:8082 ... make backend-dev"' bound :8080, failed the port check and left an orphaned api process
- Root cause: The preview launcher does not pass inline env assignments through to the child the way a shell does; the api fell back to defaults and outlived the failed launch
- Fix: Killed the orphan (go run + api serve) by PID; dropped the api-alt config and proxied through the frontend instead
- Prevention rule: Do not rely on inline env in launch.json commands; put env in .env files or a wrapper script, and after a failed preview_start check lsof for orphans

## L-045 · 2026-10-29 · frontend/css
- What broke: Every Latin string rendered in Times and Bengali in the system font; computed body font-family was 'Times'
- Root cause: html { @apply font-sans } referenced var(--font-manrope)/var(--font-bengali), but next/font defines those variables on <body> (className), so on <html> the var() is undefined and the whole font-family declaration is invalid at computed-value time; latent since Sprint 1 (Inter/Noto had the same structure)
- Fix: Moved font-sans from html to body in globals.css
- Prevention rule: Apply font utilities on the element that owns the next/font variables (body) or below; after a font change verify getComputedStyle(document.body).fontFamily in the browser
