# Sprint 1 — QA pass (task #7)

- Tester: qa-tester · Date: 2026-10-29 (Asia/Dhaka) · Build: working tree as of 11:00 BST
- Env: API `http://localhost:8080/graphql` (process started 10:57) on Postgres `psaas_dev` (seeded); web `http://localhost:3100` (Next dev). Neither was restarted.
- Method: GraphQL over curl (bash scripts, per L-015), built-in browser at desktop (1024×768) and mobile preset (375×812), build gates run uncached.
- Lessons grepped first: L-010 (slot leak), L-012 (UUID scalar), L-016 (expiring window), L-017 (POST only), L-018 (inbox 375px), L-020 (portal vs ledger).

## Summary

| | Count |
|---|---|
| PASS | 53 |
| FAIL | 10 |
| BLOCKED | 0 |
| Bugs filed | 17 (P0: 0 · P1: 4 · P2: 13) |

Tenant isolation held on every probe (lists, by-id reads and 14 cross-tenant mutations). There is no P0. P1s: vault key mismatch (credential reveal and outbound messaging broken for all seeded data), invited users can never log in, an unaccepted invite grants full access, and overpayment is accepted silently.

**Verdict:** not signed off. Task #7 stays open until BUG-01..04 are fixed or explicitly deferred by the orchestrator.

## Test data created (dev DB, safe to keep)
- Tenant `QA Tenant B` (slug `qa-tenant-b`, owner `qa.1793250048@qa.test`); staff user `staff.1793250107@qa.test` (own tenant `Staff Own Biz` + STAFF in QA Tenant B); invited-only user `staff0.1793250107@qa.test`.
- QA Tenant B: customers `Rahim QA`, `Karim QA`; products `QA Netflix`, `QA Spotify Race`, `QA ChatGPT Key`; master accounts NF-QA-1/2, SP-RACE; tickets #9, #10.
- Demo tenant: storefront orders #14 (confirmed → sub on NF-03 slot 2), #15 (out-of-stock ChatGPT), #16 (UI); UI-created sub Rahim Uddin / Spotify slot 6; verified payment `SEED000113X`; note on `QA Storefront Buyer`; 3 queued outbound messages in the Fahim Shahriar Telegram thread.

## 1. API (GraphQL via curl)

| ID | Case | Result | Evidence |
|---|---|---|---|
| A01 | `register` new tenant | PASS | OWNER membership, tenant `qa-tenant-b`, tz `Asia/Dhaka`, currency BDT |
| A02 | `login` new owner | PASS | accessToken issued |
| A03 | New tenant `customers` empty | PASS | `totalCount:0`; demo has 31 at that moment |
| A04 | `customer(id: <demo customer>)` with new token | PASS | `{"customer":null}` |
| A05 | Cross-tenant by-id reads: subscription, invoice, conversation, masterAccount, ticket, order, product, freeSlots, ledger, customerNotes, subscriptions(filter.customerId) | PASS | all `null` / `[]` / `totalCount:0` |
| A06 | Cross-tenant mutations: updateCustomer, createPortalLink, recordPayment (invoice & customer), voidInvoice, verifyPayment, sendMessage, updateSlot, cancelSubscription, updateTicket, confirmOrder, linkConversationCustomer, revealMasterAccountCredential | PASS | every one `NOT_FOUND` |
| A07 | New tenant aggregate lists + `dashboardStats` | PASS | all zero, no demo data |
| A08 | Unauthenticated `customers` | PASS | `UNAUTHENTICATED` |
| A09 | `inviteMember` role STAFF | PASS | `status:"invited"`, audit `member.invite` written |
| A10 | Invited brand-new user can log in | **FAIL** | user has no password, no accept/set-password flow → `invalid credentials` forever (BUG-02) |
| A11 | Staff (via switchTenant) `deleteProduct` | PASS | `FORBIDDEN "requires role owner or admin"` |
| A12 | Staff `auditLogs` | PASS | `FORBIDDEN` |
| A13 | Staff revealMasterAccountCredential / inviteMember / updateTenant / deleteCustomer | PASS | all `FORBIDDEN` |
| A14 | Unaccepted invite cannot act in tenant | **FAIL** | `switchTenant` into a tenant where membership `status:"invited"` succeeds; existing user was attached without consent (BUG-03) |
| A15 | Invoice maths: 3×33333 + 1×35050 − 1 discount | PASS | subtotal 135049, total 135048, line totals 99999/35050 (integer paisa) |
| A16 | `recordPayment` CASH 10001 then 15149 on 25150 invoice | PASS | PARTIAL (paid 10001, due 15149) → PAID (due 0); ledger sum 0 == dueBalance 0 |
| A17 | bKash/Nagad payments PENDING until `verifyPayment` | PASS | verify 50000 → PARTIAL; verify rest → PAID; second verify → `payment is already verified` |
| A18 | Overpayment guard | **FAIL** | pending 135048 verified after 50500 already paid → invoice `paidAmount:185548 > total:135048`, customer dueBalance −50500, no error (BUG-04) |
| A19 | Ledger sums to `dueBalance` | PASS | Rahim QA: ledger Σ −50500 == dueBalance −50500; Karim QA 0 == 0; storefront buyer 0 == 0 |
| A20 | Duplicate TrxID | PASS | `CONFLICT "this TrxID has already been used"`; `" qatrx0001 "` also caught (normalised) |
| A21 | Amount 0 / −100 / 10.5 | PASS | `amount: must be > 0` ×2; `invalid money amount: send integer minor units` |
| A22 | `createSubscription durationDays:30 startsAt 2026-10-29T22:30+06:00` | PASS | expiresAt `2026-11-28T22:30:00+06:00` (exactly +30d, Dhaka offset), slot 1 assigned, invoice created & linked |
| A23 | `durationDays` 0 / −5 | PASS | `durationDays: must be 1..3650` |
| A24 | Back-dated already-expired sub | PASS | status EXPIRED, `slot:null` (L-010 holds) |
| A25 | `expiringSubscriptions(days:7)` | PASS | contains EXPIRING (2d) and ACTIVE (6d); excludes 9d and PENDING 30d; equals `subscriptions(filter:{expiringWithinDays:7}).totalCount` (2) |
| A26 | Slot overselling, sequential | PASS | 4th sub on 3-slot account → `CONFLICT "no free slot"` |
| A27 | Slot overselling, 6 concurrent requests on 2 free slots | PASS | 2 succeeded (slots 1, 2), 4 `no free slot`; product freeSlotCount 0, subs 2 |
| A28 | `placeOrder` public (no auth) | PASS | order #14 AWAITING_PAYMENT, amount 35000, payment numbers returned |
| A29 | placeOrder guards: foreign tenant product, bad phone, reused TrxID, burst | PASS | NOT_FOUND / `customerPhone: enter a valid mobile number` / `trxId: already used` / `too many orders from this device` |
| A30 | placeOrder for out-of-stock product (`inStock:false`) | **FAIL** | order #15 created AWAITING_PAYMENT (BUG-12) |
| A31 | `publicOrder` lookup | PASS | right phone → order; wrong phone → null |
| A32 | `confirmOrder` | PASS | DELIVERED; sub ACTIVE on slot, deliveredAt set; payment VERIFIED; invoice PAID; ledger 0; second confirm → `order is already delivered` |
| A33 | `createPortalLink` + `portal(token)` | PASS | URL `/p/<token>`; view shows sub, PAID invoice, due 0; bad/tampered token → null |
| A34 | `conversations(filter:{customerId})` | PASS | 1 thread, matching customer |
| A35 | `sendMessage` validation | PASS | closed FB 24h window → clear error; empty body → `body: message is empty` |
| A36 | `sendMessage` delivery (Telegram, canReply) | **FAIL** | message stored `status:"queued"`, `error:"retrying: vault: decryption failed"` (BUG-01); template variables substituted correctly |
| A37 | `resolveReplacement` | PASS | sub moved to NF-QA-2 slot 1, old slot BANNED, expiresAt +3d, ticket RESOLVED; re-resolve → `already resolved`; no free slot → `CONFLICT`, ticket stays OPEN |
| A38 | `dashboardStats` consistency (demo + QA tenant) | PASS | activeSubscriptions 29 == ACTIVE+EXPIRING 29 (QA: 6 == 6); expiringIn7Days == list 5/2; customers 33; pendingPayments 4; openTickets 3 == open 2 + inProgress 1; openConversations 7 == inboxCounts.open; totalDue 304000 == Σ positive dueBalance; freeSlots 3 == Σ masterAccount.freeSlots |
| A39 | `revealMasterAccountCredential` | **FAIL** | secret created this session reveals OK; every seeded secret → `INTERNAL_SERVER_ERROR "vault: decryption failed"` (BUG-01) |
| A40 | Human-readable numbers per tenant | **FAIL** | new tenant's first invoice `INV-202610-0081`, first ticket #9: global sequences (BUG-11) |

## 2. Frontend (built-in browser)

| ID | Case | Desktop | 375px | Evidence / notes |
|---|---|---|---|---|
| F01 | Login wrong password | **FAIL** | — | shows "আপনার সেশন শেষ হয়ে গেছে, আবার লগইন করুন।" (session expired) instead of invalid credentials (BUG-05) |
| F02 | Login / logout | PASS | — | redirect to /dashboard; profile menu → logout → /login |
| F03 | Dashboard renders, numbers match API | PASS | PASS | 29 active / 5 expiring / due ৳3,040 == API; scrollWidth 375 |
| F04 | Dashboard i18n | **FAIL** | **FAIL** | at-risk reasons rendered raw `EXPIRING_SOON`, `OVERDUE_INVOICE` (BUG-06); mixed digits (BUG-08) |
| F05 | Customers list + filters | PASS | PASS | cards on mobile; one filter label clipped (BUG-15) |
| F06 | Customer detail: 6 tabs, add note, portal link copy | PASS | PASS | subscriptions/invoices/ledger/messages/notes/reminders all have empty states; note saved; toast "পোর্টাল লিংক কপি হয়েছে" |
| F07 | Subscription create sheet: actually create | PASS | PASS (layout) | Rahim Uddin + Spotify → slot 6 auto-picked, invoice toggle, toast; stored startsAt `2026-10-29T00:00+06:00` (Dhaka midnight, correct); mobile renders as full-width bottom sheet |
| F08 | Invoice detail | PASS | PASS | items, totals, payments, actions; status flips to পরিশোধিত after verify |
| F09 | Payments: verify pending | PASS | PASS | SEED000113X verified, removed from pending list, toast; desktop needs horizontal scroll to reach the button (BUG-16) |
| F10 | Inbox: open thread, send message, quick reply | PASS | PASS | messages + template sent and rendered; failed bubbles show raw English error (BUG-07); delivery itself is blocked by BUG-01 |
| F11 | Inventory: master list + slot grid | PASS | PASS | NF-03 2/4, slot cards show customer/PIN; mobile sheet fits |
| F12 | Credential reveal dialog | **FAIL** | PASS (layout) | audit-warning confirm dialog OK; reveal → toast `vault: decryption failed` (BUG-01, BUG-07) |
| F13 | Tickets list + detail + replacement dialog | PASS | PASS | reason shows raw `password_changed` (BUG-06) |
| F14 | Settings: 6 tabs | PASS | PASS | business/team/storefront/templates/language/account render; `free`, `active` untranslated (BUG-06) |
| F15 | Storefront `/s/demo`: place order | PASS | PASS | order #16 placed → success modal with bKash/Nagad numbers; empty submit shows Bengali inline errors; out-of-stock card button disabled |
| F16 | Portal link page | PASS | PASS | `/p/<token>` shows due, sub, invoice, renew button; invalid token → friendly Bengali page |
| F17 | Not-found / bad id states | PASS | — | unknown uuid → "কাস্টমার পাওয়া যায়নি"; malformed id shows raw `invalid UUID` (BUG-07) |
| F18 | 375px horizontal overflow | — | PASS | `documentElement.scrollWidth == 375` on dashboard, customers, customer detail, subscriptions (+sheet), invoice detail, payments, inbox thread (+quick-reply popover), inventory (+slot sheet, reveal dialog), tickets, ticket detail, settings (6 tabs), storefront (+order sheet), portal |
| F19 | Console errors | PASS | PASS | none across the run (only React DevTools info) |

## 3. Build gates

| ID | Gate | Result | Evidence |
|---|---|---|---|
| B1 | `go vet ./...` | PASS | exit 0 |
| B2 | `go test ./...` (`-count=1 -v`) | PASS | 26 packages ok, 61 tests passed, 0 skipped |
| B3 | `pnpm typecheck` | PASS | `tsc --noEmit` exit 0 |
| B4 | `pnpm lint` | PASS | `eslint` exit 0 |

## Bugs

| ID | Sev | Area | Title | Repro | Expected | Actual |
|---|---|---|---|---|---|---|
| BUG-01 | P1 | vault / env | Seeded vault secrets cannot be decrypted by the running API | 1) `revealMasterAccountCredential(id: <any seeded master account>)` as demo owner. 2) `sendMessage` on Telegram thread `7b052522…` | plaintext credential; message sent | `INTERNAL_SERVER_ERROR "vault: decryption failed"`; outbound messages stuck `queued` with `retrying: vault: decryption failed`. Secrets created after the 10:57 restart decrypt fine. All 94 rows are `key_version=1`. Root cause: `VAULT_KEY` is not pinned anywhere in the repo (no `.env.example`/Makefile default), so seed and API ran with different keys, and the version number can't tell them apart. Fix: commit a dev-only key to `.env.example` + Makefile, store a key fingerprint with each secret (or fail fast at boot if a canary secret won't decrypt) |
| BUG-02 | P1 | auth/tenant | Invited users can never log in | `inviteMember(input:{email:"new@x.test",name:"N",role:STAFF})` for an email with no account → try `login` | invite link / set-password flow | user row created with `password_hash NULL`; there is no accept, set-password or magic-link mutation, so login always says `invalid credentials` |
| BUG-03 | P1 | auth/tenant (security) | Unaccepted invite grants full tenant access; existing users attached without consent | Owner A invites B's existing email → B calls `switchTenant(<A tenant>)` | membership must be accepted first; switchTenant rejects `invited` | switchTenant succeeds while status is `invited`; B gets a STAFF token for A and the tenant appears in B's memberships without B's consent. Handed to security-expert |
| BUG-04 | P1 | billing | Overpayment silently accepted; invoice paid > total | Invoice total 135048 → record bKash 50000 (pending), Nagad 135048 (pending), cash 500 → verify both | reject or warn when the amount exceeds `dueAmount`, or book the excess as an explicit advance/credit | invoice `paidAmount 185548`, `dueAmount 0`; customer `dueBalance −50500` with no warning. Ledger stays consistent. Seed customer Nabila Chowdhury already shows −৳100 |
| BUG-05 | P2 | frontend auth | Wrong password shows "session expired" | /login → valid email + wrong password | "ভুল ইমেইল/ফোন বা পাসওয়ার্ড" | "আপনার সেশন শেষ হয়ে গেছে, আবার লগইন করুন।": `lib/graphql/errors.ts` maps every `UNAUTHENTICATED` to `errors.unauthenticated` |
| BUG-06 | P2 | i18n | Raw API enum/key strings rendered untranslated | Dashboard → ঝুঁকিতে থাকা কাস্টমার; Settings → ব্যবসা/টিম; ticket #1; storefront order modal | Bengali labels | `EXPIRING_SOON`, `OVERDUE_INVOICE`, plan `free`, status `active`, `কারণ: password_changed`, `bkash · personal` |
| BUG-07 | P2 | frontend errors | Raw backend error text shown to users | reveal credential (BUG-01), failed inbox bubbles, `/customers/not-a-uuid` | translated, friendly error | toasts/bubbles show `vault: decryption failed`, `retrying: vault: decryption failed`, `invalid UUID` |
| BUG-08 | P2 | i18n | Mixed Bengali/Latin numerals | Dashboard with "বাংলা সংখ্যা" setting OFF | consistent digits per setting | dates/labels use Bengali digits (`২৯ অক্টো, ২০২৬`, `৭ দিনে`) while amounts/counts use Latin (`৳ 11,190`, `2 দিন বাকি`, `রিনিউ হয়নি (৩০ দিন): 4`) |
| BUG-09 | P2 | security (observation) | Delivery messages store plaintext credentials in conversation history | demo conv `8289095d…` → `lastMessagePreview` / messages | credentials only through audited reveal (CLAUDE.md) | `Password: Seed-NF-03-pass` readable by any STAFF via `conversation.messages`/preview; slot PINs also plaintext. Handed to security-expert to decide |
| BUG-10 | P2 | a11y / UX | List rows are not links or buttons | Customers, invoices, tickets list rows; inbox conversation items | `<a>`/button with accessible name; open-in-new-tab, keyboard | rows are click-handlers on generic elements (invoice numbers are `generic` in the a11y tree); inbox items are unnamed `button`s; storefront plan radios and one subscriptions filter combobox have no label |
| BUG-11 | P2 | billing/ticket/storefront | Invoice, ticket and order numbers use global sequences | new tenant: first invoice `INV-202610-0081`, first ticket #9 | per-tenant contiguous numbering (0001, #1) | `invoice_number_seq`, `ticket_number_seq`, `order_number_seq` are shared by all tenants, which leaks platform volume and leaves gaps in a tenant's books |
| BUG-12 | P2 | storefront API | `placeOrder` accepts products with `inStock:false` | `placeOrder` on demo ChatGPT Plus (inStock false) | `CONFLICT`/validation "out of stock" (UI already disables button) | order #15 created AWAITING_PAYMENT |
| BUG-13 | P2 | storefront/portal | Public pages use SaaS title | open `/s/demo`, `/p/<token>` | `<title>` = store name | "সেলারডেস্ক — SellerDesk" |
| BUG-14 | P2 | inbox 375px | Thread header truncates customer name to ~4 chars | 375px → inbox → Fahim Shahriar | readable name (status select can shrink) | "Fahi…", "Demo St…"; failed-bubble timestamps have very low contrast |
| BUG-15 | P2 | 375px filters | Filter select label clipped | 375px → customers / inventory | full label or shorter text | "মেয়াদ শেষ হবে (দিন): স" cut off |
| BUG-16 | P2 | payments desktop | Verify/reject actions hidden behind horizontal scroll at 1024px | /payments at 1024×768 | action column visible (sticky or fewer columns) | must scroll the table right to reach "যাচাই করুন" |
| BUG-17 | P2 | inbox | Quick reply inserts raw `{{customer}}` into composer | inbox → ⚡ → "ধন্যবাদ {{customer}}!" | preview with variables filled | composer shows the placeholder text; the sent message is substituted correctly |

Retracted during the run: "no Settings link on desktop": the link exists in the sidebar below the fold (top 776px at 768px height).

## Notes for the team
- Isolation, role checks, TrxID dedupe, slot allocation under concurrency, Dhaka-offset expiry math and dashboard consistency are solid. Keep the API scripts (`/tmp/qa/scripts/t1..t11.sh` pattern) as a regression suite candidate.
- `daysLeft` uses ceil of hours from now. A 30-day sub starting tonight shows "31 days left" until midnight. This is documented behaviour, not filed.
- Expired subscriptions in the list still show the "ডেলিভারি বাকি" badge. Low value, not filed.
