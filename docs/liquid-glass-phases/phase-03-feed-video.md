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

- [ ] Split feed rendering into reusable `ui` subcomponents.
- [ ] Preserve existing IDs and HTMX targets used by OOB swaps.
- [ ] Preserve watch-later behavior for feed/video/carousel variants.
- [ ] Rebuild feed section headers/dividers with new primitives.
- [ ] Rebuild video page top rail buttons/chips with new styles.
- [ ] Rebuild video toast/notification surface with CSS-first animation.
- [ ] Ensure hidden/watched/live/offline states remain visually explicit.

## Verification

- [ ] `/app` renders new/watched/watch-later correctly.
- [ ] `/app/recent` renders and updates correctly.
- [ ] `/app/watch-later` pagination and empty-state behavior work.
- [ ] `/video/:id` retains share/open/back/login behavior.
- [ ] Progress hide/unhide/unwatch/watch actions still work via HTMX.
- [ ] OOB updates keep card and carousel in sync.

## Exit Criteria

- Feed and video experiences are fully migrated to `ui` primitives.
- No behavior regressions in progress/watch-later flows.

## Notes

- Keep channel metadata and relative time styles consistent across feed contexts.

