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

- [ ] Remove DaisyUI CDN stylesheet links from `internal/templates/layouts/partials/head.templ`.
- [ ] Remove Tailwind browser runtime script from `internal/templates/layouts/partials/head.templ`.
- [ ] Add local stylesheet link to `/assets/css/style.css` in `internal/templates/layouts/partials/head.templ`.
- [ ] Ensure build command always generates `assets/css/style.css`.
- [ ] Update Dockerfile to run frontend build before Go binary build.
- [ ] Add `assets/css/style.css` to `.gitignore`.
- [ ] Untrack generated CSS from git after confirming build pipeline works.
- [ ] Confirm app renders styled output in dev and production modes.

## Verification

- [ ] `npm run build` succeeds.
- [ ] `task dev` renders styled pages.
- [ ] Production-style run (`npm run build && go run .`) renders styled pages.
- [ ] Docker build generates CSS during image build.
- [ ] No runtime dependency on style CDNs remains.

## Exit Criteria

- Styling is produced by local build pipeline only.
- Generated CSS is ignored by git and not manually maintained.
- Build/docs clearly reflect the new source-of-truth flow.

## Notes

- Keep Tailwind-first approach; avoid adding new custom CSS outside tokenized/reusable cases.

