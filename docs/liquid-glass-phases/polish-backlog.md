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
