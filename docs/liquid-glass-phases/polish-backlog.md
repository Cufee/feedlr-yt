# Liquid Glass Deferred Polish Backlog

Track UX polish items that are intentionally deferred until implementation phases are complete.

## How to Use

- Add new deferred issues with concrete reproduction steps.
- Keep items small and testable.
- Resolve items during `phase-06-hardening.md`.

## Open Items

| ID | Area | Issue | Current State | Revisit In |
|---|---|---|---|---|
| POL-001 | HTMX loading indicators | Video-card action spinners can still appear immediately for `mark watched/unwatched`, `hide/show`, and watch-later toggles. Current delayed indicator threshold (`--delay-feedback`) may be too low for real request latency patterns. | Delay is `180ms` via `ui-indicator-delayed`; needs threshold tuning and consistency checks across interactions. | Phase 06 |
| POL-002 | Loading feedback consistency | Align perceived timing across nav progress bar, card spinners, and any other transient loaders so micro-feedback does not flash. | Nav progress delay implemented; spinner delay implemented; end-to-end timing audit not finished. | Phase 06 |
| POL-003 | Video progress polish | Final pass on video-card progress bar transparency and color tuning to better match the glass material direction after broad migration is complete. | Progress bar has multiple polish iterations; still needs final global color/transparency balance pass. | Phase 06 |
| POL-004 | Color hierarchy separation | Do a dedicated pass on page-card color hierarchy so nav/footer surfaces are visually distinct from content cards while staying within the same material system. | Current glass palette is cohesive but nav/footer vs page-card separation can be clearer. | Phase 06 |
