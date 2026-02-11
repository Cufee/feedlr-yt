# Post-Phase 06 TODO (Carryovers + New UI Feedback)

This is the active working checklist after phase 06 closeout.

It combines:
- Unchecked carryover items from earlier phase docs.
- New feedback items raised after phase 06 completion.

## Execution Rules

- Keep diffs small and easy to review.
- Prefer shared `ui-*` primitives and token updates over one-off styles.
- Preserve behavior unless explicitly changed in this checklist.
- Add/adjust checkpoints after each logical batch.

## A. Visual System Corrections

- [x] `VIS-001` Reduce video title size/weight, especially desktop feed cards, while preserving mobile readability.
  - Acceptance: desktop card title no longer dominates thumbnail; mobile remains readable.
- [x] `VIS-002` Apply real backdrop blur to video duration pill.
  - Acceptance: duration chip visibly blurs underlying thumbnail content.
- [x] `VIS-003` Revert/normalize primary color direction to approved palette (no unrequested primary color drift).
  - Acceptance: documented palette update with explicit rationale and before/after screenshots.
- [x] `VIS-004` Remove page background gradients globally.
  - Acceptance: all routes use flat/neutral background treatment.
- [x] `VIS-005` Comprehensive color pass:
  - remove blue tint bias from global surfaces
  - make header/footer visually distinct from page cards
  - keep cards distinct from app chrome
  - Acceptance: nav/footer vs content cards are separable by value/contrast, not only borders.
- [x] `VIS-009` Brightness diagnosis/final darkening pass.
  - Evaluate whether perceived brightness is from base palette, surface alpha, border contrast, or blur interactions.
  - Acceptance: app no longer feels "washed/bright" across nav, cards, and footer while preserving readability.
- [x] `VIS-006` Radius consistency pass across:
  - header/footer containers
  - video cards
  - buttons/icon buttons
  - pills/chips
  - Acceptance: radius token usage is consistent and documented; no outlier curvature.
- [x] `VIS-007` Navbar icon-button radius consistency correction.
  - Acceptance: nav icon buttons use the same compact control radius contract as the rest of the app.
- [x] `VIS-008` Header brand/app-name scale tuning.
  - Acceptance: app name in header aligns with overall typography hierarchy and no longer appears oversized.

## B. Feed and App Page Polish

- [x] `FEED-001` `/app` open-video input area spacing fix (remove excessive gap below input block).
  - Acceptance: spacing aligns with surrounding section rhythm on desktop and mobile.
- [x] `FEED-002` `/app` open-video action button radius fix (match control radius tokens).
  - Acceptance: button corner radius matches shared control spec.
- [x] `FEED-003` Video page layout refinement:
  - pure black background for player page
  - no gap between controls and video container
  - maximize usable video area while keeping top controls always visible
  - Acceptance: controls remain pinned/visible; player consumes remaining viewport height.

## C. Settings and Interaction Fixes

- [x] `SET-001` Passkey add error layout stabilization.
  - Make error wrap cleanly or force button+error into stable column.
  - Acceptance: no major horizontal/vertical layout jump when error appears.
- [x] `SET-002` Prevent scroll-to-top when pausing/unpausing YouTube sync and TV sync.
  - Acceptance: toggle actions preserve current scroll position and context.
- [x] `SET-003` Passkeys row label stability regression fix.
  - Keep `Passkeys` label anchored when error state appears.
  - Acceptance: label position remains fixed with and without error text.
- [x] `SET-004` YouTube/TV sync frontend error messaging simplification.
  - Replace raw/technical frontend errors with generic user-friendly messages.
  - Acceptance: UI no longer surfaces backend/network exception text directly in sync sections.
- [x] `SET-005` TV sync disabled-state labeling fix.
  - Validate and correct disabled/paused/unavailable labels for TV sync controls and status badges.
  - Acceptance: labels accurately reflect state and match playlist sync wording conventions.

### Notes

- `VIS-005` has an initial implementation pass (global tint reduced and nav/footer separated from content cards), but remains open for final polish iteration.
- Added an extra brightness-focused follow-up (`VIS-009`) to capture uncertainty between color values vs material treatment.

## D. Legal Pages

- [x] `LEGAL-001` Polish privacy policy and terms pages.
  - Apply readable typographic container and consistent spacing hierarchy.
  - Acceptance: long-form content has stable line length, heading hierarchy, and paragraph rhythm.

## E. Carryover Verification From Earlier Phases

These remain unchecked in phase docs and must be explicitly validated or marked not applicable.

- [x] `CV-001` Phase 00 carryover: confirm styled output in both dev and production-style runs.
- [x] `CV-002` Phase 01 carryover: confirm primitives render correctly on mobile and desktop sandbox.
- [x] `CV-003` Phase 02 carryover: route/layout smoke set:
  - `/`, `/login`, `/app`, `/app/recent`, `/app/subscriptions`, `/app/settings`, `/video/:id`
  - navbar active-state correctness
  - no full-page layout shift during HTMX navigation
- [x] `CV-004` Phase 02.5 carryover: validate interactions remain responsive on low-end/mobile.
- [x] `CV-005` Phase 03 carryover:
  - `/app`, `/app/recent`, `/app/watch-later`, `/video/:id` behavior checks
  - progress/hide/watch-later HTMX actions
  - OOB sync between card and carousel
  - evaluate remaining split of feed rendering into reusable `ui` subcomponents
- [x] `CV-006` Phase 04 carryover:
  - subscriptions search/filter UX
  - onboarding flow
  - channel subscribe/unsubscribe
  - filter tab swaps
  - mobile tile readability/tap targets
- [x] `CV-007` Phase 05 carryover:
  - settings account actions (passkey add/delete)
  - sponsorblock global/category toggles
  - YouTube sync + TV sync controls
  - fallback pages `/`, `/login`, `/error`, `/429`
  - outage page behavior
  - legal remote content functionality

## F. Checkpoint Plan

- [x] `CP-001` Create a checkpoint after visual system corrections (`VIS-*`).
- [x] `CP-002` Create a checkpoint after interaction/settings fixes (`SET-*`, `FEED-*`).
- [x] `CP-003` Create a checkpoint after legal polish + carryover verification closure (`LEGAL-*`, `CV-*`).

## Verification Notes (2026-02-11)

- Visual darkening/color separation (`VIS-005`, `VIS-009`) shipped by tightening neutral OKLCH values and separating `--color-chrome` from `--color-surface` usage in nav/footer vs page cards.
- Video page top control rail now uses plain black with no glass border/background layer, preserving control padding while maximizing video-first presentation.
- Carryover verification executed with route smoke (`curl`) and browser checks (Playwright snapshots/screenshots) across desktop and mobile viewports.
- Production-style styled-output validation was run via non-dev `go run .` on alternate ports with explicit runtime env overrides (`METRICS_PORT`, cron env) and confirmed stylesheet/link + `ui-*` class rendering on login route.
- HTMX interaction coverage validated for representative flows: watch-later toggle (`style=card`), settings toggles, channel filter tab swaps, subscriptions search updates, and outage/legal/fallback routes.
- Feed subcomponent split remains intentionally deferred as a non-blocking structural refactor; behavior and OOB sync paths were revalidated.
