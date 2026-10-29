# Design quality bar (no "AI slop")

Every screen this team ships must look designed for *this* product — Bangladeshi subscription
sellers on a phone — not like a generated template. Compact, purposeful, eye-catching.
Read with ADR-009 (design language "Nodi"). The ui-ux-designer blocks any screen that fails this list.

## Compact
- Information density first: a seller should see what needs action without scrolling. No giant
  empty heroes, no 3-line paragraphs where one line works, no padding inflation on phones.
- One primary action per screen, reachable by thumb on 375px. Secondary actions are quieter.
- Copy is short, plain Bengali (everyday words), sentence case. Every word does one job.

## Eye-catching (spend boldness in ONE place)
- Each screen has exactly one memorable element (dashboard Today hero, auth sample inbox,
  landing live demo). Everything around it stays quiet and disciplined.
- Use the product's real world: channel icons, bKash/Nagad TrxIDs, slots, expiry dates, ৳ amounts.
  Illustrative data is labelled as a sample.
- Motion answers the user (press, open, confirm) or marks one arrival. No decorative loops
  except the single hero moment; everything respects prefers-reduced-motion.

## Banned "AI slop" tells (reject in review)
- Tracked-out ALL-CAPS eyebrow labels above headings; "01 / 02 / 03" markers on content that is
  not a sequence; '→' glued to link text; meta strings chained with " · " everywhere.
- Accenting one word of a headline in a different colour/italic; gradient text.
- The SaaS-card kit: identical rounded cards for everything, one grey shadow (rgba(0,0,0,.1)),
  decorative gradient blobs, glassmorphism on content.
- Generic stock copy ("Unlock the power of…", "Seamless", "Revolutionize", "All-in-one solution").
- Fake social proof: invented testimonials, customer counts, logos or ratings. Use real data or
  nothing.
- Emoji as icons; random illustrations unrelated to the domain; lorem ipsum.

## Definition of done for any UI task
1. Uses tokens from `frontend/src/styles/tokens.css` and shared components; no ad-hoc colours.
2. Screenshot reviewed at 375px and desktop, light and dark; `scrollWidth == innerWidth`.
3. The screen's one bold element is named in the task note; the banned list is checked.
4. Loading, empty and error states designed (shimmer, invitation-to-act empty state, actionable error).
5. Bengali and English both fit (Bengali is ~20% longer); no truncation of key info.
