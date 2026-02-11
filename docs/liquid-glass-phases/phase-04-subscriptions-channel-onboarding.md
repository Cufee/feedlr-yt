# Phase 04: Subscriptions, Channel, and Onboarding

## Objective

Unify channel discovery and subscription management UI with the new component system.

## Scope

- Channel tiles and subscribe/unsubscribe controls.
- Search and results surfaces.
- Channel page header and filter tabs.
- Onboarding and subscriptions pages.

## Target Files

- `internal/templates/components/subscriptions/channels.templ`
- `internal/templates/components/subscriptions/search.templ`
- `internal/templates/components/channel/channel.templ`
- `internal/templates/pages/app/subscriptions.templ`
- `internal/templates/pages/app/onboarding.templ`
- `internal/templates/pages/channel.templ`
- `internal/server/routes/api/channels.templ` (response compatibility checks)
- `internal/templates/components/ui/channel-tile.templ` (new)
- `internal/templates/components/ui/filter-tabs.templ` (new)

## Work Checklist

- [x] Rebuild channel tile using shared card and media primitives.
- [x] Rebuild search input/results with consistent form and tile styles.
- [x] Rebuild channel header section and subscribe action placement.
- [x] Rebuild filter tab visuals (`all/videos/streams`) while keeping endpoint behavior.
- [x] Keep search result overlays and loading indicators compatible with HTMX requests.
- [x] Align onboarding and subscriptions page structure to shared sections.

## Verification

- [ ] `/app/subscriptions` search and filtering UX works.
- [ ] `/app/onboarding` subscription flow works.
- [ ] `/channel/:id` subscribe/unsubscribe actions work.
- [ ] Channel filter tab updates still swap the correct feed content.
- [ ] Mobile layout supports tile readability and tap targets.

## Exit Criteria

- Discovery/subscription experiences match new visual direction and remain behaviorally stable.

## Notes

- Reuse one channel tile primitive across all channel contexts to avoid drift.
- Phase 04 pass 1 completed:
- Added shared channel/search surface classes in `tailwind.css` (`ui-channel-*`, `ui-search-*`, `ui-filter-tabs`).
- Migrated subscription and search tiles to consistent glass cards while preserving HTMX boost/targets/indicators.
- Migrated subscribe/unsubscribe controls to tokenized channel action button styles.
- Migrated channel filter tabs to `ui-tab` visuals while preserving `/api/channels/:id/filter` behavior.
- Reworked `/app/subscriptions`, `/app/onboarding`, and `/channel/:id` page structure to shared section/card conventions.
- Updated Fuse search input to `ui-input` with `ui-input-error` invalid state class handling.
- Build verification run:
- `npm run build` (success)
- `go generate ./...` (success)
- `go build ./...` (success)
