# Liquid Glass Deferred Polish Backlog

Track UX polish items that are intentionally deferred until implementation phases are complete.

## How to Use

- Add new deferred issues with concrete reproduction steps.
- Keep items small and testable.
- Resolve items during `phase-06-hardening.md`.

## Resolved In Phase 06

| ID | Area | Issue | Current State | Revisit In |
|---|---|---|---|---|
| POL-001 | HTMX loading indicators | Video-card action spinners can still appear immediately for `mark watched/unwatched`, `hide/show`, and watch-later toggles. Current delayed indicator threshold (`--delay-feedback`) may be too low for real request latency patterns. | Raised shared delay token to `260ms` and kept delayed indicator class on HTMX loaders to suppress micro-flash responses. | Closed |
| POL-002 | Loading feedback consistency | Align perceived timing across nav progress bar, card spinners, and any other transient loaders so micro-feedback does not flash. | Nav progress now reads `--delay-feedback` from CSS, so nav and spinner reveal timing stay aligned. | Closed |
| POL-003 | Video progress polish | Final pass on video-card progress bar transparency and color tuning to better match the glass material direction after broad migration is complete. | Progress track/fill transparency and embedded blur treatment were tuned in Phase 03/04 polish passes and accepted. | Closed |
| POL-004 | Color hierarchy separation | Do a dedicated pass on page-card color hierarchy so nav/footer surfaces are visually distinct from content cards while staying within the same material system. | Section shell hierarchy and card layering were adjusted across subscriptions/settings/channel surfaces and accepted for this redesign pass. | Closed |
