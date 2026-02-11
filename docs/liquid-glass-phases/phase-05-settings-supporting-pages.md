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
- [ ] Wrap legal remote content in readable typographic container.
- [ ] Confirm passkey and sync actions remain operational.

## Verification

- [ ] `/app/settings` account actions (add/delete passkey) work.
- [ ] SponsorBlock toggles and category toggles work.
- [ ] YouTube sync and TV sync controls work.
- [ ] `/`, `/login`, `/error`, `/429` render properly in desktop/mobile.
- [ ] Maintenance mode renders migrated outage page correctly.
- [ ] Legal pages retain remote content functionality.

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
