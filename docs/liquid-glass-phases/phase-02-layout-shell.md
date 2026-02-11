# Phase 02: Layout Shell Migration

## Objective

Rebuild top-level page shells and app chrome on the new `ui` primitives while preserving routing and HTMX behavior.

## Scope

- Main/app/video layouts.
- Head includes and global body wrappers.
- Navbar, footer, and progress shell.

## Target Files

- `internal/templates/layouts/main.templ`
- `internal/templates/layouts/app.templ`
- `internal/templates/layouts/video.templ`
- `internal/templates/layouts/partials/head.templ`
- `internal/templates/layouts/partials/navbar.templ`
- `internal/templates/layouts/partials/footer.templ`
- `internal/templates/layouts/partials/progress.templ`
- `internal/templates/components/ui/pageshell.templ` (new)
- `internal/templates/components/ui/navbar.templ` (new)
- `internal/templates/components/ui/footer.templ` (new)
- `internal/templates/components/ui/progress.templ` (new)

## Work Checklist

- [x] Introduce `ui.PageShell` and apply it to main/app/video layout wrappers.
- [x] Replace legacy navbar markup with `ui.NavBar` variants for authed and guest states.
- [x] Rebuild footer as a consistent primitive with legal and repo links.
- [x] Keep HTMX attributes (`hx-boost`, `hx-push-url`, indicators) functionally equivalent.
- [x] Keep nav progress integration compatible with request lifecycle.
- [x] Preserve SEO/meta behavior from existing head partial and page-level head blocks.

## Verification

- [ ] Routed pages still render with correct layout:
- [ ] `/`
- [ ] `/login`
- [ ] `/app`
- [ ] `/app/recent`
- [ ] `/app/subscriptions`
- [ ] `/app/settings`
- [ ] `/video/:id`
- [ ] Navbar state and active route highlighting are correct.
- [ ] No full-page layout shift during HTMX navigation.

## Exit Criteria

- All pages run on new shell primitives.
- Existing route behavior and page metadata remain intact.

## Notes

- Keep class composition utility-first; avoid introducing style logic in JS.
- Implemented files:
- `internal/templates/components/ui/pageshell.templ`
- `internal/templates/components/ui/navbar.templ`
- `internal/templates/components/ui/footer.templ`
- `internal/templates/components/ui/progress.templ`
- `internal/templates/layouts/main.templ`
- `internal/templates/layouts/app.templ`
- `internal/templates/layouts/video.templ`
- `internal/templates/layouts/partials/navbar.templ`
- `internal/templates/layouts/partials/footer.templ`
- `internal/templates/layouts/partials/progress.templ`
- Build validation run:
- `npm run build` (success)
- `go generate ./...` (success)
- `go build ./...` (success)
- Manual page verification is pending user browser checks on the running dev server.
