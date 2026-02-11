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

- [ ] Rebuild settings sections with `ui.Section` + `ui.Card` patterns.
- [ ] Normalize toggle, badge, status, and action button styles.
- [ ] Keep all settings endpoints and form actions unchanged.
- [ ] Rebuild login form and feedback surfaces using shared input/button/toast primitives.
- [ ] Rebuild landing/error/429/outage pages with unified fallback layout language.
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

