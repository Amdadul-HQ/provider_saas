# ADR-009 · UI design language v2 ("Nodi")

Status: accepted · 2026-10-29 · owner: ui-ux-designer / frontend-expert

## Context
The v1 UI was a correct but templated shadcn layout (identical bordered cards, Inter, pulse
skeletons, static navigation). The product owner asked for a Dribbble-level, attractive and
smooth experience. We reviewed ~40 popular Dribbble shots (SaaS subscription dashboards,
WhatsApp CRM inboxes, subscription-manager mobile apps). What the best ones share:

1. Soft layered surfaces: a tinted canvas, white cards with hairline borders and a *tinted*
   two-layer shadow, radii that scale with hierarchy (hero 24px → card 16px → control 10px).
2. One human "hero" moment per screen (greeting + what needs attention), not a wall of equal KPIs.
3. KPI tiles with a tonal icon chip and one big number; everything else quiet.
4. Inbox: pill filters with counts, avatar + platform badge, tinted bubbles, rounded composer.
5. Mobile: a floating dock instead of a flush tab bar.
6. Motion only on arrival and in answer to an action; never decorative loops.

## Decision
- Palette stays "Ghat" (river-teal primary, amber reserved for money/alerts, warm paper
  canvas) — it is already ours and not a Dribbble default. We add elevation tokens
  (`--elev-1..3`, tinted teal), hero gradient tokens, and motion tokens (`--dur-*`, `--ease-smooth`,
  `--ease-spring`).
- Type: **Manrope** (Latin + digits, geometric, tabular nums) and **Hind Siliguri**
  (Bengali, the face Bangladeshi fintech UIs use). Inter / Noto Sans Bengali retired.
- Radius scale: 8 / 10 / 12 / 16 / 20 / 24 (sm…3xl). Cards `rounded-2xl`, hero `rounded-3xl`,
  controls `rounded-lg`.
- Navigation: dark-teal sidebar with sentence-case section labels and an amber active
  indicator; phones get a floating blurred dock (`--dock-height` + `--dock-gap`).
- Dashboard opens with a **Today** hero (greeting, count of things needing attention,
  action chips that deep-link) and the primary action; KPI tiles follow.
- Motion: one `page-enter` rise per route change, `stagger` on tile grids, shimmer skeletons,
  `active:scale-[0.98]` on buttons, hover lift only on cards that are links.
  `prefers-reduced-motion` collapses all of it.
- No ALL-CAPS eyebrow labels, no decorative gradients on content cards, no arrows glued to
  link text.

## Consequences
- `frontend/src/styles/tokens.css` is the single source of colour / elevation / motion tokens;
  components use semantic names only.
- New shared pieces: `TodayFocus` (analytics module), shimmer `Skeleton`, `stagger`/`page-enter`
  utilities. Existing pages pick the look up through the shared components they already use.
- Dev-only: `NEXT_DEV_API_PROXY` rewrite in `next.config.ts` lets a second dev server on another
  port reach the API same-origin (used for design verification next to a running dev server).
