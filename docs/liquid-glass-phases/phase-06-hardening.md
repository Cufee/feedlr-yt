# Phase 06: Hardening and Cleanup

## Objective

Remove legacy styling debt, confirm consistency, and finalize documentation.

## Scope

- DaisyUI dependency removal.
- Dead code cleanup.
- Final cross-device QA and motion/accessibility checks.
- Finalize frontend docs.
- Close deferred UX polish backlog items.

## Target Files

- `package.json`
- `package-lock.json`
- `tailwind.css`
- `internal/templates/**/*.templ`
- `docs/FRONTEND.md`
- `docs/LIQUID-GLASS-REDESIGN.md`

## Work Checklist

- [x] Remove remaining DaisyUI dependencies and references.
- [x] Remove dead/unused shared components (for example `internal/templates/components/shared/link.templ` if confirmed unused).
- [x] Remove stale style classes no longer part of the new system.
- [x] Ensure all templates use shared `ui` primitives consistently.
- [x] Update `docs/FRONTEND.md` with final component contracts and conventions.
- [x] Update main redesign doc with shipped status and any accepted deviations.
- [x] Burn down deferred polish items in `docs/liquid-glass-phases/polish-backlog.md`.

## Verification

- [x] Search confirms no DaisyUI class usage remains (except intentional compatibility shims, if any).
- [x] Full app route smoke test passes.
- [x] HTMX interactions still functional across all migrated pages.
- [x] `prefers-reduced-motion` and focus-visible behavior pass final checks.
- [x] Build/test pipelines pass with generated CSS ignored by git.

## Exit Criteria

- Redesign is complete, documented, and operationally stable.
- Styling system is Tailwind-first with minimal, reusable custom CSS.

## Notes

- Keep this phase open until every route listed in the main plan has at least one validation pass recorded.
- Deferred interaction polish should not be dropped; track and close items from `docs/liquid-glass-phases/polish-backlog.md` here.
- Phase 06 pass 1:
- Migrated `internal/templates/components/shared/open-video.templ` from legacy Daisy modal/join/input/button classes to shared `ui-dialog`, `ui-input`, and `ui-btn` primitives.
- Kept modal/open-video behavior intact while removing stale class usage and a duplicate `showModal()` call in the open-modal fallback path.
- Phase 06 pass 2:
- Fixed open-video modal rendering regression by restoring proper full-viewport dialog positioning/layering (`ui-dialog`) and removing the legacy backdrop form node that surfaced as visible "close" content.
- Added click-outside close behavior directly on the native dialog container to preserve expected dismissal behavior without Daisy modal helpers.
- Phase 06 pass 3:
- Removed final DaisyUI build/runtime dependency (`daisyui` package and `@plugin "daisyui"` usage), replaced remaining Daisy loader classes with shared `ui-spinner` styling, and removed unused `shared/link` component.
- Unified delayed loading feedback by raising `--delay-feedback` to `260ms` and making nav-progress reveal delay read the same CSS token used by HTMX indicators.
- Phase 06 pass 4:
- Updated `docs/FRONTEND.md` to reflect shipped Tailwind + `ui-*` primitive contracts and removed outdated DaisyUI class/examples.
- Closed Phase 06 polish backlog items in `docs/liquid-glass-phases/polish-backlog.md` with implemented outcomes.
