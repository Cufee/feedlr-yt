# Phase 01: Foundations and Tokens

## Objective

Define the core design system for Polar Glass and create reusable UI primitives.

## Scope

- Semantic tokens in Tailwind/CSS.
- Base typography and spacing scales.
- Foundational `ui` components for layout and controls.
- Tailwind-first utility patterns.

## Target Files

- `tailwind.css`
- `internal/templates/components/ui/*.templ` (new)
- `internal/templates/components/ui/*.go` (optional helpers)
- `docs/FRONTEND.md` (update component contracts)

## Work Checklist

- [x] Add color, radius, blur, and motion semantic tokens in `tailwind.css`.
- [x] Define typography stack and sizing for headings/body/metadata.
- [x] Add reusable utility recipes for glass panel, chip, solid panel.
- [x] Create `ui.Button` variants (primary, neutral, ghost, danger, icon-only).
- [x] Create `ui.Input` and `ui.SearchInput` with error/disabled/focus states.
- [x] Create `ui.Card`, `ui.Section`, `ui.Badge`, `ui.Toggle`, `ui.Tabs`.
- [x] Create `ui.Dialog`, `ui.Toast`, `ui.EmptyState`.
- [x] Add component usage notes in `docs/FRONTEND.md`.

## Verification

- [x] Tokens are referenced by components (not hardcoded one-off values).
- [x] Components support responsive and interactive states via Tailwind utilities.
- [x] `motion-safe:*` and `motion-reduce:*` variants included on animated primitives.
- [x] Components render correctly on mobile and desktop in at least one page sandbox.

## Exit Criteria

- Core primitive set is sufficient to replace current ad-hoc style usage.
- New work can compose UI from `ui` primitives without DaisyUI classes.

## Notes

- Custom CSS should be limited to reusable abstractions that are unreadable with inline utility composition.
- Implemented files:
- `internal/templates/components/ui/button.templ`
- `internal/templates/components/ui/input.templ`
- `internal/templates/components/ui/layout.templ`
- `internal/templates/components/ui/tabs.templ`
- `internal/templates/components/ui/toggle.templ`
- `internal/templates/components/ui/dialog.templ`
- `internal/templates/components/ui/toast.templ`
- `internal/templates/components/ui/classes.go`
- Build validation run:
- `npm run build` (success)
- `go generate ./...` (success)
- `go build ./...` (success)
- Carryover verification (2026-02-11):
- Desktop and mobile snapshots confirm shared primitives (`ui-btn`, `ui-input`, `ui-card`, `ui-nav-shell`, `ui-footer-shell`, tab/toggle primitives) render consistently.
