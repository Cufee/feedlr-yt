# Liquid Glass UI Redesign and Migration Plan

## Goal

Rebuild the Feedlr UI from scratch with:

- Tailwind as the styling foundation
- Custom common components (no DaisyUI component classes)
- A distinct visual direction inspired by Apple's Liquid Glass behavior
- A migration plan that covers all pages, shared components, cleanup, and deployment safety

## Source Notes

Reviewed sources:

- Apple Liquid Glass overview: https://developer.apple.com/documentation/TechnologyOverviews/liquid-glass
- Apple adopting Liquid Glass: https://developer.apple.com/documentation/TechnologyOverviews/adopting-liquid-glass
- Apple HIG materials: https://developer.apple.com/design/human-interface-guidelines/materials
- Tailwind with Vite: https://tailwindcss.com/docs/installation/using-vite

Key takeaways used in this plan:

- Liquid Glass emphasizes dynamic translucency, layering, and adaptive behavior based on context.
- Adoption guidance emphasizes using system conventions, preserving legibility, and avoiding decorative overuse.
- HIG materials emphasize hierarchy, harmony, and consistency across surfaces.
- Tailwind + Vite setup should be simple and explicit: install `tailwindcss` and `@tailwindcss/vite`, register the Vite plugin, and import Tailwind in the app CSS entry.

## Current Frontend Baseline

- Server-rendered Go + Templ
- HTMX + Hyperscript interactions
- Runtime styling now uses local generated CSS (`/assets/css/style.css`) instead of Tailwind/Daisy CDN runtime injection
- Generated CSS output (`assets/css/style.css`) is gitignored and rebuilt via frontend build pipeline
- Docker build now includes a styles build stage before Go compile
- Phase 01 foundations are in place (`tailwind.css` tokens + `internal/templates/components/ui/*` primitives)
- Phase 02 shell migration is in place (layout/navbar/footer/progress on `ui` chrome primitives)
- Phase 03 pass 1 is in place (feed/video surface styling migrated to tokenized `ui-*` classes; behavior preserved)

## Design Direction: "Polar Glass"

### Direction Summary

Create a "Polar Glass" UI language:

- Cool neutral palette with high contrast text, subtle cyan accents, no purple bias
- Frosted translucent panels over soft atmospheric gradients
- Crisp boundaries (hairline borders) with restrained blur and saturation
- Utility-first layout prioritizing video scanning speed over decorative density

### Direction Updates From Implementation

Implemented refinements that slightly narrow the original aesthetic:

- Navigation controls use neutral glass states (not accent-heavy fills) to reduce visual noise.
- Brand mark sizing was reduced and baseline-aligned with nav controls for cleaner rhythm.
- Progress feedback favors stability over micro-feedback (delayed reveal to avoid flashing on fast requests).
- Spinner indicators now use delayed reveal for HTMX actions to prevent flicker on sub-perceptual responses.
- Motion defaults are subtle and short, with clear reduced-motion fallbacks.
- Feed/video chrome now favors compact glass action chips and consistent metadata rhythm across cards, carousel, and player rail.
- Feed card typography and control density were softened (lighter title/overlay weight, roomier icon padding, inset blurred progress treatment) to reduce visual heaviness.
- Glass progress surfaces now use lower-opacity layers and softer fill luminance to keep overlays atmospheric instead of visually dense.
- Radius handling now uses a shared token scale (`--radius-*`) with media/control-specific aliases to keep curvature consistent across shells, cards, and actions.
- Video-card internals now follow a shared inset/size grid so button cluster, duration chip, and progress bar align consistently at every breakpoint.

### Visual Principles

- Surfaces communicate hierarchy first, effect second.
- Motion is purposeful: navigation continuity, loading feedback, and state transitions.
- Legibility is non-negotiable: translucent surfaces must pass contrast targets.
- Touch targets remain explicit and consistent regardless of glass effect.
- Prefer native browser and CSS capabilities over JavaScript for interaction and animation.

### Tokens (Design Contract)

Define semantic tokens in `tailwind.css` using `@theme` and CSS variables:

- Color tokens:
  - `--color-bg-canvas`, `--color-bg-elev-1`, `--color-bg-elev-2`
  - `--color-glass-fill`, `--color-glass-stroke`, `--color-glass-shadow`
  - `--color-text-primary`, `--color-text-secondary`, `--color-accent`
- Radius tokens:
  - `--radius-sm`, `--radius-md`, `--radius-lg`, `--radius-xl`
- Motion tokens:
  - `--dur-fast`, `--dur-base`, `--dur-slow`
  - `--ease-standard`, `--ease-emphasized`
- Blur tokens:
  - `--blur-glass-sm`, `--blur-glass-md`

### Typography

- Display/headings: `Space Grotesk`
- Body/UI: `IBM Plex Sans`
- Mono/data: `IBM Plex Mono`

Rules:

- Tight heading rhythm, compact list/card metadata
- Explicit sizes for card title, channel text, and timestamps to preserve scanability

### Surface Language

- `glass-panel`: primary translucent container (settings sections, nav shell, modal surfaces)
- `glass-chip`: compact actions/toggles/tabs
- `solid-panel`: high-contrast fallback for dense or error-critical regions

### Motion and Interaction Policy (HTMX + CSS-first)

Priority order:

1. Tailwind utilities and variants (`transition-*`, `duration-*`, `ease-*`, `motion-safe:*`, `motion-reduce:*`, `group-*`, `peer-*`, responsive variants)
2. Native browser semantics (`dialog`, `details/summary`, focus states, CSS pseudo classes/selectors)
3. HTMX swap classes and timing (`.htmx-added`, `.htmx-swapping`, `.htmx-settling`, swap/settle delays)
4. Minimal JavaScript only when behavior cannot be expressed cleanly with CSS/browser primitives

Rules:

- Do not introduce JavaScript-only animation if equivalent CSS behavior exists.
- Do not use Hyperscript for visual-only transitions that CSS can handle.
- Keep custom CSS small and tokenized; prefer Tailwind classes in templates first.
- Add custom CSS only for reusable patterns that cannot be expressed readably with Tailwind utilities.

## Tailwind Architecture (Target)

Use Tailwind with a Vite pipeline and local generated assets.

### Build Direction

- Keep `tailwind.css` as source of tokens/utilities/components
- Compile to `assets/css/style.css` via Vite build output or a Tailwind-only Vite entry
- Remove runtime CDN dependencies from `head.templ`:
  - remove DaisyUI CDN links
  - remove `@tailwindcss/browser` script
- Link local stylesheet in `head.templ`

### Custom Common Components

Create a new shared UI layer at:

- `internal/templates/components/ui/`

Initial primitives:

- `ui.PageShell`
- `ui.NavBar`
- `ui.Footer`
- `ui.Section`
- `ui.Card`
- `ui.GlassPanel`
- `ui.Button`
- `ui.IconButton`
- `ui.Input`
- `ui.SearchInput`
- `ui.Toggle`
- `ui.Badge`
- `ui.Tabs`
- `ui.Dialog`
- `ui.Toast`
- `ui.ProgressBar`
- `ui.EmptyState`

Then recompose domain components (`feed`, `settings`, `subscriptions`, `channel`) from those primitives.

## Full Page Migration Inventory

| Route | Current Template | Layout | Migration Target |
|---|---|---|---|
| `/` | `internal/templates/pages/landing.templ` | `layouts.Main` | New marketing landing shell with Polar Glass hero and clear primary CTA |
| `/login` | `internal/templates/pages/login.templ` | `layouts.Main` | Rebuild auth card, input, buttons, and inline error toast using `ui.Card`, `ui.Input`, `ui.Button`, `ui.Toast` |
| `/app` | `internal/templates/pages/app/index.templ` | `layouts.App` | Rebuild feed sections, watch-later strip, and card actions using new `ui` primitives |
| `/app/recent` | `internal/templates/pages/app/history.templ` | `layouts.App` | Rebuild list/feed state using same feed component family as `/app` |
| `/app/watch-later` | `internal/templates/pages/app/watchlater.templ` | `layouts.App` | Rebuild paginated feed, empty state, and nav actions |
| `/app/subscriptions` | `internal/templates/pages/app/subscriptions.templ` | `layouts.App` | Rebuild channel grid/search/filter shell with shared form and tile primitives |
| `/app/onboarding` | `internal/templates/pages/app/onboarding.templ` | `layouts.App` | Rebuild onboarding stack with unified section cards |
| `/app/settings` | `internal/templates/pages/app/settings.templ` | `layouts.App` | Rebuild settings surface groups and controls with consistent spacing and component contracts |
| `/channel/:id` | `internal/templates/pages/channel.templ` | `layouts.App` | Rebuild channel header card, subscribe action, and filter tabs with unified tile/buttons |
| `/video/:id` | `internal/templates/pages/video.templ` | `layouts.Video` | Rebuild control rail and player chrome overlays with glass chips and high contrast states |
| `/error` | `internal/templates/pages/error.templ` | `layouts.Main` | Rebuild error panel with explicit severity hierarchy |
| `/429` | `internal/templates/pages/429.templ` | `layouts.Main` | Rebuild rate-limit page using shared empty/error state component |
| Maintenance mode | `internal/templates/pages/outage.templ` | direct render in middleware | Rebuild outage state with same fallback page shell as `/429` |
| `/legal/privacy-policy` | remote in `internal/server/routes/legal.go` | `layouts.Main` | Wrap remote content in new prose container with consistent typography |
| `/legal/terms-of-service` | remote in `internal/server/routes/legal.go` | `layouts.Main` | Same as privacy policy |
| (not currently routed) `/app/admin` | `internal/templates/pages/app/admin.templ` | `layouts.App` | Keep as low-priority shell migration once route is re-enabled |

## Common Component Migration Inventory

### Layout and App Chrome

- `internal/templates/layouts/main.templ`
- `internal/templates/layouts/app.templ`
- `internal/templates/layouts/video.templ`
- `internal/templates/layouts/partials/head.templ`
- `internal/templates/layouts/partials/navbar.templ`
- `internal/templates/layouts/partials/footer.templ`
- `internal/templates/layouts/partials/progress.templ`

Actions:

- Replace Daisy-style utility composition with `ui` shell primitives.
- Move nav progress animation into `ui.ProgressBar`.
- Standardize max-width, gutters, and responsive spacing rules in one location.

### Feed Components

- `internal/templates/components/feed/video.templ`

Actions:

- Split into composable units:
  - `ui.VideoCard`
  - `ui.VideoMeta`
  - `ui.VideoActionCluster`
  - `ui.WatchLaterButton`
  - `ui.VideoCarousel`
- Preserve HTMX behavior and IDs used by OOB swaps.

### Settings Components

- `internal/templates/components/settings/account.templ`
- `internal/templates/components/settings/sponsorblock.templ`
- `internal/templates/components/settings/youtube-sync.templ`

Actions:

- Replace section-specific card styles with `ui.Section` + `ui.Card`.
- Standardize toggles, status badges, and action groups.
- Keep interaction endpoints unchanged while style layer is swapped.

### Subscription and Channel Components

- `internal/templates/components/subscriptions/channels.templ`
- `internal/templates/components/subscriptions/search.templ`
- `internal/templates/components/channel/channel.templ`

Actions:

- Consolidate tile and action button patterns with feed card styles.
- Reuse one searchable list pattern across onboarding/subscriptions.

### Shared Components

- `internal/templates/components/shared/logo.templ`
- `internal/templates/components/shared/textbox.templ`
- `internal/templates/components/shared/search.templ`
- `internal/templates/components/shared/open-video.templ`
- `internal/templates/components/shared/remote.templ`
- `internal/templates/components/shared/deleted.templ`
- `internal/templates/components/shared/link.templ`

Actions:

- Keep behavior helpers (`deleted`, `remote`) but move visual classes to new `ui` components.
- `shared/link.templ` appears unused and inconsistent; remove during cleanup if confirmed dead.

### Icons

- `internal/templates/components/icons/*.templ`

Actions:

- Keep icon files, normalize sizing/stroke conventions via wrapper classes in `ui.IconButton`.

## Detailed Migration Plan

Working phase files (mutable implementation trackers) are in `docs/liquid-glass-phases/`.

- `docs/liquid-glass-phases/README.md`
- `docs/liquid-glass-phases/phase-00-cleanup-build-safety.md`
- `docs/liquid-glass-phases/phase-01-foundations-tokens.md`
- `docs/liquid-glass-phases/phase-02-layout-shell.md`
- `docs/liquid-glass-phases/phase-025-native-motion.md`
- `docs/liquid-glass-phases/phase-03-feed-video.md`
- `docs/liquid-glass-phases/phase-04-subscriptions-channel-onboarding.md`
- `docs/liquid-glass-phases/phase-05-settings-supporting-pages.md`
- `docs/liquid-glass-phases/phase-06-hardening.md`

## Phase 0: Initial Cleanup and Build Safety

1. Remove runtime style CDN usage in `internal/templates/layouts/partials/head.templ`.
2. Add local stylesheet link (`/assets/css/style.css` or Vite output equivalent).
3. Add generated CSS to `.gitignore`:
   - `assets/css/style.css`
4. Untrack generated CSS from git:
   - `git rm --cached assets/css/style.css` (after pipeline is ready)
5. Update Docker build to generate CSS before Go build.
6. Ensure `task dev` and production build both generate styles consistently.
7. Remove DaisyUI package and classes from templates as migration progresses.

Exit criteria:

- App renders with local generated CSS only.
- No dependency on CDN Tailwind or DaisyUI at runtime.
- Clean reproducible build path for dev and Docker.

## Phase 1: Foundations and Tokens

1. Define design tokens in `tailwind.css`.
2. Add base utility classes for glass surfaces, borders, focus rings, motion.
3. Establish typography scale and spacing scale.
4. Build foundational `ui` primitives (button/input/card/badge/tabs/section).

Exit criteria:

- New primitives cover all current control variants without Daisy classes.

## Phase 2: Layout Shell Migration

1. Migrate `layouts/main.templ`, `layouts/app.templ`, `layouts/video.templ`.
2. Replace nav/footer/progress partials with shared `ui` shell pieces.
3. Preserve current route behavior and HTMX attributes.

Exit criteria:

- All pages render through new shell without visual regressions in navigation flow.

## Phase 2.5: Native Motion Follow-up (HTMX-heavy UI)

1. Replace script-driven visual transitions with CSS transitions where possible.
2. Add HTMX swap animation patterns using CSS classes for enter/exit/settle states.
3. Introduce shared Tailwind motion recipes for:
   - card reveal
   - feed swap fade/slide
   - modal open/close
   - toast enter/exit
   - nav progress and loading affordances
4. Gate non-essential motion with `motion-safe:*` and define reduced motion fallbacks with `motion-reduce:*`.
5. Keep JavaScript only for non-CSS behavior (WebAuthn, YouTube player API, clipboard/share APIs, timed progress persistence).

Exit criteria:

- Visual transitions for HTMX content updates are CSS-driven.
- No new JS/Hyperscript is added for effects that CSS can provide.
- Reduced-motion behavior is defined for all major animated surfaces.

## Phase 3: Feed and Video Surfaces

1. Migrate feed card, carousel, and watch-later button variants.
2. Migrate `/app`, `/app/recent`, `/app/watch-later`.
3. Migrate `/video/:id` action rail and toast styling.

Exit criteria:

- All feed/watch interactions (progress, hide, watch later, OOB swaps) remain functional.

## Phase 4: Subscriptions, Channel, Onboarding

1. Migrate channel tile and subscribe/unsubscribe controls.
2. Migrate search UI and results tiles.
3. Migrate `/app/subscriptions`, `/app/onboarding`, `/channel/:id`.

Exit criteria:

- Search, subscribe actions, and filter tabs maintain existing endpoint behavior.

## Phase 5: Settings and Supporting Pages

1. Migrate account, sponsorblock, and YouTube sync settings cards.
2. Migrate `/settings`, `/login`, `/`, `/error`, `/429`, and outage page.
3. Apply legal page typography container for remote content.

Exit criteria:

- All interactive settings actions remain unchanged at API boundary.
- Fallback/error pages are visually consistent with the new system.

## Phase 6: Deletion and Hardening

1. Remove dead classes/components and leftover Daisy utility usage.
2. Remove obsolete dependencies from `package.json`.
3. Run full route QA and responsive checks.
4. Document final component contracts in `docs/FRONTEND.md`.

Exit criteria:

- No DaisyUI class usage remains.
- All rendered pages use new common components.
- CI/build passes with generated CSS ignored by git.

## QA and Validation Checklist

- Desktop + mobile checks for every routed page
- Keyboard navigation and focus-visible states
- Contrast checks on translucent surfaces
- HTMX swaps and out-of-band updates
- HTMX swap animations work without JavaScript-driven effect code
- Video page controls and hotkeys
- No layout shift during nav progress animation
- `prefers-reduced-motion` behavior verified
- Docker build produces styled output without committed generated CSS
