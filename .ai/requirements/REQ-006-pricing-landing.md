# REQ-006 Pricing, free trial and marketing landing page

Owner: business-analyst · 2026-10-29 · status: **proposed (prices need owner sign-off)**

## Problem
A seller finds us from a Facebook group or a friend's link and has one minute on a phone to decide.
Today `/` redirects straight to the dashboard, so there is nothing that explains what we do, what it
costs, or why it is safe to try. Sellers here are price-sensitive, rarely have a card, pay with
bKash/Nagad, and distrust "free" offers that ask for payment details up front.

## Who pays and what they earn (sizing)
- Typical small seller (our demo tenant is realistic): ~30 active subscriptions, ~৳10,000 sales and
  ~৳3,000 profit per month, 1–2 people, 2–3 channels.
- Growing seller: 150–1,000 active subscriptions, 2–5 staff or resellers, all channels.
- Their biggest money leaks (goal.md): forgotten renewals (৳300–600 each), oversold slots,
  unpaid baki, banned master accounts. One saved renewal a month must pay for the entry plan.

## Value metric
**Active subscriptions** (live = pending/active/expiring). It grows with the seller's revenue, is
already counted by `dashboardStats.activeSubscriptions`, and does not punish them for chatting
with many leads (conversations stay unlimited on every plan).

## Plans (BDT, monthly; yearly = pay 10 months, 2 months free)
| Plan | Price | Active subscriptions | Team | Channels | Adds |
|------|-------|----------------------|------|----------|------|
| শুরু / Starter | ৳499 | up to 150 | 1 owner + 1 staff | 2 | Inbox, customers, slots, invoices + baki, reminders, TrxID auto-verify, storefront + portal |
| বিজনেস / Business (suggested) | ৳1,299 | up to 1,000 | 5 | all 4 | + auto-reply, resellers + commission, campaigns, CSV import, post scheduler, profit reports |
| প্রো / Pro | ৳2,999 | unlimited | unlimited | all 4 | + multiple businesses, priority WhatsApp support, early access |

Rationale: Starter ≈ 1/6 of a small seller's monthly profit; Business is priced below one part-time
helper; Pro anchors the top. Prices end in 99 (local norm) and stay under round psychological
thresholds (৳500 / ৳1,300 / ৳3,000).

## Free trial
- **First month free, every feature of Business, no card or payment details.** Sign-up is the
  existing 2-step form; the trial starts at registration.
- In-app + SMS/WhatsApp reminders on day 23, 28 and 30 with a one-tap "pay with bKash/Nagad".
- After day 30 without payment: 7-day **read-only grace** (nothing deleted, reminders to their
  customers keep running so the seller does not lose money), then the account locks; data is kept
  90 days.
- Paying = choose plan → pay via bKash/Nagad (merchant number + TrxID, verified by our own
  TrxID matcher) → active for 30 or 365 days.

## Landing page (`/`)
User stories
1. As a seller on a phone, within the first screen I understand: one app for page messages,
   payments, slots and renewals; first month free; no card.
2. I can see every feature grouped by the job I do (sell, money, stock, service, team).
3. I can compare plans, switch monthly/yearly and see the yearly saving in ৳.
4. I get honest answers: what happens after the trial, how I pay, is my account data safe,
   who is responsible for shared accounts.
5. A signed-in seller sees "Open dashboard" instead of sign-up buttons.

Acceptance
- Bengali first, English via the language toggle; all copy in `src/i18n`.
- No invented testimonials, customer counts or logos (design-quality.md). Demo data labelled as a sample.
- Passes `.ai/design-quality.md`; 375px has no horizontal scroll; Lighthouse-friendly (SSR, metadata).
- CTAs go to `/register`; login to `/login`; PWA `start_url` stays `/dashboard`.

## Not built yet (PM: break into Sprint 3 tasks)
- Backend: `tenant_plans` (plan, status trial/active/grace/locked, period end), limit checks on
  active subscriptions/team/channels, trial reminder jobs, read-only enforcement in the API.
- Billing UI: plan page in settings, bKash/Nagad payment + TrxID submission, invoices for the seller.
- Until then the landing page states the offer; nothing is enforced.

## BN UX note
Use "১ মাস ফ্রি", "কার্ড লাগবে না", "bKash/Nagad-এ পেমেন্ট", "যেকোনো সময় বাতিল". Avoid "ট্রায়াল"
alone in headlines; sellers read it as "demo". Prices with ৳ and Bengali digits when the user's digit
setting is Bengali.
