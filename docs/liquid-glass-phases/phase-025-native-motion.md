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

- [x] Add reusable Tailwind motion recipes for cards, panels, toasts, and modal surfaces.
- [x] Attach HTMX swap-state classes and transitions to list/feed update containers.
- [x] Remove script-driven animation where CSS can replace it.
- [x] Preserve script usage only for behavioral logic:
- [x] WebAuthn flows
- [x] YouTube player API integration
- [x] Clipboard/share APIs
- [x] Timed progress persistence
- [x] Add `motion-reduce:*` fallbacks for major animated components.
- [x] Verify no new Hyperscript is added for visual-only transitions.
- [x] Delay HTMX spinner indicators to avoid flash on sub-perceptual requests.

## Verification

- [x] HTMX content enters/exits smoothly with CSS-only effect rules.
- [x] Modals and toasts animate without custom JS animation functions.
- [x] `prefers-reduced-motion` disables non-essential motion.
- [x] Loading spinners do not flash for very fast HTMX requests.
- [ ] Interaction remains responsive on low-end/mobile devices.

## Exit Criteria

- Major UI transitions are CSS-driven.
- JavaScript is reserved for non-visual functionality only.

## Notes

- If one-off custom CSS is needed, promote it into reusable tokenized patterns instead of per-component ad-hoc rules.
- Implemented motion classes:
- `ui-motion-swap` (HTMX enter/exit/settle)
- `ui-motion-toast` (toast enter/settle)
- `ui-motion-modal-panel` (dialog panel open transition)
- `ui-indicator-delayed` (HTMX indicator reveal delay, 180ms)
- Updated templates:
- `internal/templates/components/feed/video.templ`
- `internal/templates/components/subscriptions/search.templ`
- `internal/templates/pages/channel.templ`
- `internal/templates/components/shared/open-video.templ`
- `internal/templates/pages/video.templ`
- `internal/templates/layouts/partials/navbar.templ`
- Build verification run:
- `npm run build` (success)
- `go generate ./...` (success)
- `go build ./...` (success)
