# Phase 02: Layout Shell Migration

## Objective

Rebuild top-level page shells and app chrome on the new `ui` primitives while preserving routing and HTMX behavior.

## Scope

- Main/app/video layouts.
- Head includes and global body wrappers.
- Navbar, footer, and progress shell.

## Target Files

- `internal/templates/layouts/main.templ`
- `internal/templates/layouts/app.templ`
- `internal/templates/layouts/video.templ`
- `internal/templates/layouts/partials/head.templ`
- `internal/templates/layouts/partials/navbar.templ`
- `internal/templates/layouts/partials/footer.templ`
- `internal/templates/layouts/partials/progress.templ`
- `internal/templates/components/ui/pageshell.templ` (new)
- `internal/templates/components/ui/navbar.templ` (new)
- `internal/templates/components/ui/footer.templ` (new)
- `internal/templates/components/ui/progress.templ` (new)

## Work Checklist

- [x] Introduce `ui.PageShell` and apply it to main/app/video layout wrappers.
- [x] Replace legacy navbar markup with `ui.NavBar` variants for authed and guest states.
- [x] Rebuild footer as a consistent primitive with legal and repo links.
- [x] Keep HTMX attributes (`hx-boost`, `hx-push-url`, indicators) functionally equivalent.
- [x] Keep nav progress integration compatible with request lifecycle.
- [x] Preserve SEO/meta behavior from existing head partial and page-level head blocks.

## Verification

- [x] Routed pages still render with correct layout:
- [x] `/`
- [x] `/login`
- [x] `/app`
- [x] `/app/recent`
- [x] `/app/subscriptions`
- [x] `/app/settings`
- [x] `/video/:id`
- [x] Navbar state and active route highlighting are correct.
- [x] No full-page layout shift during HTMX navigation.

## Exit Criteria

- All pages run on new shell primitives.
- Existing route behavior and page metadata remain intact.

## Notes

- Keep class composition utility-first; avoid introducing style logic in JS.
- Implemented files:
- `internal/templates/components/ui/pageshell.templ`
- `internal/templates/components/ui/navbar.templ`
- `internal/templates/components/ui/footer.templ`
- `internal/templates/components/ui/progress.templ`
- `internal/templates/layouts/main.templ`
- `internal/templates/layouts/app.templ`
- `internal/templates/layouts/video.templ`
- `internal/templates/layouts/partials/navbar.templ`
- `internal/templates/layouts/partials/footer.templ`
- `internal/templates/layouts/partials/progress.templ`
- Build validation run:
- `npm run build` (success)
- `go generate ./...` (success)
- `go build ./...` (success)
- Phase 02 polish pass:
- Adjusted logo typography/alignment for nav cohesion.
- Tuned nav icon color and stroke weight to match shell tone.
- Normalized nav text hierarchy with display/body font usage.
- Phase 02 polish pass (feedback iteration 2):
- Reduced nav logo size to better match shell rhythm.
- Reduced icon button framing/border density for cleaner controls.
- Aligned hover/active/selected icon states to remove click flash and purple state shift.
- Phase 02 polish pass (feedback iteration 3):
- Added delayed nav progress reveal (180ms threshold) to prevent flash on sub-100ms requests.
- Preserved existing request lifecycle behavior once progress is visible.
- Phase 02 polish pass (feedback iteration 4):
- Fixed footer overlap by removing fixed-height shell behavior in main content wrapper.
- Switched main/app body wrappers back to flex-column flow for proper footer placement after content.
- Improved footer responsive alignment for narrow widths.
- Carryover verification (2026-02-11):
- Route smoke rechecked for `/`, `/login`, `/app`, `/app/recent`, `/app/subscriptions`, `/app/settings`, and `/video/:id`.
- Navbar active-state class toggles were validated by route and HTMX nav swaps in desktop and mobile snapshots.
