# Phase 02.5: Native Motion (HTMX + CSS-first)

## Objective

Implement UI motion with native CSS/browser primitives in an HTMX-heavy app, minimizing JavaScript for purely visual effects.

## Scope

- HTMX swap animations and load transitions.
- Modal, toast, card, and section motion.
- Reduced-motion compliance.

## Priority Rules

1. Tailwind utilities/variants first.
2. Native browser behavior second (`dialog`, focus, pseudo classes/selectors).
3. HTMX lifecycle classes third (`.htmx-added`, `.htmx-swapping`, `.htmx-settling`).
4. JavaScript only when CSS/browser primitives cannot implement behavior.

## Target Files

- `tailwind.css`
- `internal/templates/layouts/partials/progress.templ`
- `internal/templates/components/ui/*.templ`
- `internal/templates/components/feed/video.templ`
- `internal/templates/components/shared/open-video.templ`

## Work Checklist

- [ ] Add reusable Tailwind motion recipes for cards, panels, toasts, and modal surfaces.
- [ ] Attach HTMX swap-state classes and transitions to list/feed update containers.
- [ ] Remove script-driven animation where CSS can replace it.
- [ ] Preserve script usage only for behavioral logic:
- [ ] WebAuthn flows
- [ ] YouTube player API integration
- [ ] Clipboard/share APIs
- [ ] Timed progress persistence
- [ ] Add `motion-reduce:*` fallbacks for major animated components.
- [ ] Verify no new Hyperscript is added for visual-only transitions.

## Verification

- [ ] HTMX content enters/exits smoothly with CSS-only effect rules.
- [ ] Modals and toasts animate without custom JS animation functions.
- [ ] `prefers-reduced-motion` disables non-essential motion.
- [ ] Interaction remains responsive on low-end/mobile devices.

## Exit Criteria

- Major UI transitions are CSS-driven.
- JavaScript is reserved for non-visual functionality only.

## Notes

- If one-off custom CSS is needed, promote it into reusable tokenized patterns instead of per-component ad-hoc rules.

