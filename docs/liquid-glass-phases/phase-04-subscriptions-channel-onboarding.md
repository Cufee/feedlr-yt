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

- [ ] Rebuild channel tile using shared card and media primitives.
- [ ] Rebuild search input/results with consistent form and tile styles.
- [ ] Rebuild channel header section and subscribe action placement.
- [ ] Rebuild filter tab visuals (`all/videos/streams`) while keeping endpoint behavior.
- [ ] Keep search result overlays and loading indicators compatible with HTMX requests.
- [ ] Align onboarding and subscriptions page structure to shared sections.

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

