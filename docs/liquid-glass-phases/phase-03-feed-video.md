# Phase 03: Feed and Video Surfaces

## Objective

Migrate feed and video experiences to the new component system while preserving HTMX/OOB behavior and player flows.

## Scope

- Feed cards and metadata.
- Watch later carousel and button variants.
- Home/recent/watch-later pages.
- Video page control rail and notification surfaces.

## Target Files

- `internal/templates/components/feed/video.templ`
- `internal/templates/pages/app/index.templ`
- `internal/templates/pages/app/history.templ`
- `internal/templates/pages/app/watchlater.templ`
- `internal/templates/pages/video.templ`
- `internal/templates/components/ui/video-card.templ` (new)
- `internal/templates/components/ui/video-meta.templ` (new)
- `internal/templates/components/ui/video-actions.templ` (new)
- `internal/templates/components/ui/video-carousel.templ` (new)

## Work Checklist

- [x] Split feed rendering into reusable `ui` subcomponents.
- [x] Preserve existing IDs and HTMX targets used by OOB swaps.
- [x] Preserve watch-later behavior for feed/video/carousel variants.
- [x] Rebuild feed section headers/dividers with new primitives.
- [x] Rebuild video page top rail buttons/chips with new styles.
- [x] Rebuild video toast/notification surface with CSS-first animation.
- [x] Ensure hidden/watched/live/offline states remain visually explicit.

## Verification

- [x] `/app` renders new/watched/watch-later correctly.
- [x] `/app/recent` renders and updates correctly.
- [x] `/app/watch-later` pagination and empty-state behavior work.
- [x] `/video/:id` retains share/open/back/login behavior.
- [x] Progress hide/unhide/unwatch/watch actions still work via HTMX.
- [x] OOB updates keep card and carousel in sync.

## Exit Criteria

- Feed and video experiences are fully migrated to `ui` primitives.
- No behavior regressions in progress/watch-later flows.

## Notes

- Keep channel metadata and relative time styles consistent across feed contexts.
- Phase 03 pass 1 completed:
- Feed card/title/meta styling moved to shared `ui-*` classes in `tailwind.css` and `internal/templates/components/feed/video.templ`.
- Video card overlays/actions/progress indicators now use tokenized custom classes instead of Daisy-style action/button/progress classes.
- `/app`, `/app/recent`, `/app/watch-later` section chrome and empty states now use `ui-feed-divider` / `ui.EmptyState`.
- `/video/:id` rail controls and sponsor-block toast moved to `ui-video-rail-*` and `ui-toast` surfaces.
- Phase 03 pass 1 polish:
- Reduced feed title and watched/hidden overlay font weight to improve scanability.
- Increased video action-button inner padding and reduced glyph scale for better icon balance.
- Reworked in-card progress bar to an inset, larger, blurred track with rounded fill (iOS-style visual treatment).
- Phase 03 pass 1 polish v3:
- Softened feed title typography further (lower weight + slightly reduced contrast) to reduce aggressiveness.
- Increased glass feel on in-card progress treatment via lower-opacity translucent track and less-solid progress fill.
- Phase 03 pass 1 polish v4:
- Introduced an explicit radius token scale and applied it consistently to feed/video surfaces and controls.
- Increased video card/media corner radius using shared `--radius-media`.
- Reduced progress fill opacity further so the bar reads as layered glass instead of a solid accent stripe.
- Phase 03 pass 1 polish v5:
- Added dedicated video spacing/size tokens (`--space-video-*`, `--size-video-action`, `--height-video-progress`) to align action buttons, duration chip, and progress track to one inset system.
- Increased media radius to `--radius-2xl` and normalized button/chip curvature to `--radius-sm` for cleaner consistency.
- Softened progress fill opacity and glow again to keep the glass layer subtle.
- Phase 03 pass 1 polish v6:
- Fixed corner anti-alias artifact by moving card stroke to an inset ring shadow and isolating the media layer.
- Increased progress track horizontal inset so it no longer feels too close on the X axis.
- Restored background blur on video action buttons to match glass-material treatment.
- Verification run for this pass:
- `npm run build` (success)
- `go generate ./...` (success)
- `go build ./...` (success)
- Carryover verification (2026-02-11):
- Revalidated `/app`, `/app/recent`, `/app/watch-later`, and `/video/:id` on desktop/mobile.
- Revalidated HTMX actions (watch-later toggle, watched/unwatched, hide/unhide, progress updates) and server OOB watch-later sync paths.
- Feed rendering split completed with shared `ui` subcomponents:
- `internal/templates/components/ui/video-card.templ` (`VideoCardShell`)
- `internal/templates/components/ui/video-meta.templ` (`VideoMeta`)
- `internal/templates/components/feed/video.templ` now composes those shared pieces for card shell and metadata blocks.
