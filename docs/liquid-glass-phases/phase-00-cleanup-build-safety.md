# Phase 00: Cleanup and Build Safety

## Objective

Establish a safe baseline for local/prod styling so generated assets are reproducible and not committed.

## Scope

- Remove runtime style CDNs.
- Ensure app uses local generated CSS.
- Make Docker and local build pipelines produce styles reliably.
- Add generated style output to gitignore and untrack it.

## Target Files

- `internal/templates/layouts/partials/head.templ`
- `.gitignore`
- `package.json`
- `Taskfile.yaml`
- `Dockerfile`
- `tailwind.css`
- `assets/css/style.css` (untracked output after migration)

## Work Checklist

- [x] Remove DaisyUI CDN stylesheet links from `internal/templates/layouts/partials/head.templ`.
- [x] Remove Tailwind browser runtime script from `internal/templates/layouts/partials/head.templ`.
- [x] Add local stylesheet link to `/assets/css/style.css` in `internal/templates/layouts/partials/head.templ`.
- [x] Ensure build command always generates `assets/css/style.css`.
- [x] Update Dockerfile to run frontend build before Go binary build.
- [x] Add `assets/css/style.css` to `.gitignore`.
- [x] Untrack generated CSS from git after confirming build pipeline works.
- [ ] Confirm app renders styled output in dev and production modes.

## Verification

- [x] `npm run build` succeeds.
- [ ] `task dev` renders styled pages.
- [ ] Production-style run (`npm run build && go run .`) renders styled pages.
- [x] Docker build generates CSS during image build.
- [x] No runtime dependency on style CDNs remains.

## Exit Criteria

- Styling is produced by local build pipeline only.
- Generated CSS is ignored by git and not manually maintained.
- Build/docs clearly reflect the new source-of-truth flow.

## Notes

- Keep Tailwind-first approach; avoid adding new custom CSS outside tokenized/reusable cases.
- Build verification run:
- `npm run build` (success)
- `go generate ./...` (success)
- `go build ./...` (success)
- `docker build --progress=plain -t feedlr-phase0-check .` (success; styles-builder executed `npm run build`)
