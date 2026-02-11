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

- [ ] Remove remaining DaisyUI dependencies and references.
- [ ] Remove dead/unused shared components (for example `internal/templates/components/shared/link.templ` if confirmed unused).
- [ ] Remove stale style classes no longer part of the new system.
- [ ] Ensure all templates use shared `ui` primitives consistently.
- [ ] Update `docs/FRONTEND.md` with final component contracts and conventions.
- [ ] Update main redesign doc with shipped status and any accepted deviations.
- [ ] Burn down deferred polish items in `docs/liquid-glass-phases/polish-backlog.md`.

## Verification

- [ ] Search confirms no DaisyUI class usage remains (except intentional compatibility shims, if any).
- [ ] Full app route smoke test passes.
- [ ] HTMX interactions still functional across all migrated pages.
- [ ] `prefers-reduced-motion` and focus-visible behavior pass final checks.
- [ ] Build/test pipelines pass with generated CSS ignored by git.

## Exit Criteria

- Redesign is complete, documented, and operationally stable.
- Styling system is Tailwind-first with minimal, reusable custom CSS.

## Notes

- Keep this phase open until every route listed in the main plan has at least one validation pass recorded.
- Deferred interaction polish should not be dropped; track and close items from `docs/liquid-glass-phases/polish-backlog.md` here.
