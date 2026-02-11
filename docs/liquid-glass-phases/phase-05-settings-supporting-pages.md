# Phase 05: Settings and Supporting Pages

## Objective

Complete migration for settings-heavy and fallback pages with consistent Polar Glass surfaces and control patterns.

## Scope

- Settings cards and controls.
- Landing/login/error/rate-limited/outage pages.
- Legal remote-content wrappers.

## Target Files

- `internal/templates/components/settings/account.templ`
- `internal/templates/components/settings/sponsorblock.templ`
- `internal/templates/components/settings/youtube-sync.templ`
- `internal/templates/pages/app/settings.templ`
- `internal/templates/pages/landing.templ`
- `internal/templates/pages/login.templ`
- `internal/templates/pages/error.templ`
- `internal/templates/pages/429.templ`
- `internal/templates/pages/outage.templ`
- `internal/templates/components/shared/remote.templ`
- `internal/server/routes/legal.go`

## Work Checklist

- [x] Rebuild settings sections with `ui.Section` + `ui.Card` patterns.
- [x] Normalize toggle, badge, status, and action button styles.
- [x] Keep all settings endpoints and form actions unchanged.
- [x] Rebuild login form and feedback surfaces using shared input/button/toast primitives.
- [x] Rebuild landing/error/429/outage pages with unified fallback layout language.
- [x] Wrap legal remote content in readable typographic container.
- [x] Confirm passkey and sync actions remain operational.

## Verification

- [x] `/app/settings` account actions (add/delete passkey) work.
- [x] SponsorBlock toggles and category toggles work.
- [x] YouTube sync and TV sync controls work.
- [x] `/`, `/login`, `/error`, `/429` render properly in desktop/mobile.
- [x] Maintenance mode renders migrated outage page correctly.
- [x] Legal pages retain remote content functionality.

## Exit Criteria

- All remaining major user-facing pages are migrated.
- Settings behavior and API integration remain stable.

## Notes

- Avoid introducing style-only JS in fallback/supporting pages.
- Phase 05 pass 1 started:
- Migrated `/`, `/login`, `/error`, `/429`, and outage pages to `ui-*` primitives (`ui-card`, `ui-btn`, `ui-input`, `ui-toast`) and removed Daisy-specific button/input/alert class usage from those surfaces.
- Kept all existing page behavior (including auto-redirect scripts and passkey login/registration handlers) unchanged.
- Style preferences to carry forward:
- Prefer integrated full-width controls in section flow over nested card-in-card wrappers.
- Keep equivalent list/card surfaces visually consistent (hover blur/overlay affordances and interaction copy).
- Phase 05 pass 2:
- Migrated `ManageAccount`, `YouTubeSyncSettings`, and `SponsorBlockSettings` to shared `ui-*` primitives and tokenized settings surface classes.
- Replaced Daisy toggle/badge/button/input usage in settings with `ui-toggle`, `ui-badge`, `ui-btn`, and `ui-input` variants.
- Kept all settings form actions, HTMX endpoints, and confirmations unchanged.
- Phase 05 pass 3:
- Reduced settings card layering by removing the outer section-card shell and keeping section labels as the primary separators.
- Standardized inline error messaging to shared error styling (`ui-error-inline`) so settings errors align with the broader fallback/error visual language.
- Phase 05 pass 4:
- Reintroduced a lightweight section shell to avoid settings surfaces visually blending together while still avoiding heavy nested cards.
- SponsorBlock now visually distinguishes global vs per-category toggles (`ui-toggle-global` vs `ui-toggle-category`).
- SponsorBlock category controls now present clear disabled-state styling when global toggle is off (muted card + disabled toggle treatment).
- Removed the additional "Global" pill label in SponsorBlock header to reduce redundant labeling.
- Reduced YouTube sync status badge width (`w-24` -> `w-20`) for tighter metadata balance.
- Carryover verification (2026-02-11):
- Settings sync and SponsorBlock controls were revalidated with HTMX interactions and state-label checks.
- Passkey add/delete endpoints and UI surfaces were rechecked; WebAuthn ceremony remains device/browser-dependent and still requires manual confirmation for credential creation/removal in end-user environments.
- Desktop/mobile fallback routes (`/`, `/login`, `/error`, `/429`) and maintenance-mode outage rendering were revalidated.
- Legal pages still load remote content into the readable `ui-legal-prose` container.
