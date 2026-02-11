# Liquid Glass Checkpoints

This branch (`ai-slop/tailwind-liquid-glass`) is the rollout base.

Use annotated git tags for rollback checkpoints after each completed phase.

## Naming Convention

- `checkpoint/liquid-glass-00-base`
- `checkpoint/liquid-glass-01-phase-00`
- `checkpoint/liquid-glass-02-phase-01`
- `checkpoint/liquid-glass-03-phase-02`
- `checkpoint/liquid-glass-04-phase-025`
- `checkpoint/liquid-glass-04-phase-025-polish`
- `checkpoint/liquid-glass-05-phase-03-pass1`
- `checkpoint/liquid-glass-05-phase-03-polish-v2`
- `checkpoint/liquid-glass-05-phase-03-polish-v3`
- `checkpoint/liquid-glass-05-phase-03-polish-v4`
- `checkpoint/liquid-glass-05-phase-03-polish-v5`
- `checkpoint/liquid-glass-05-phase-03-polish-v6`
- `checkpoint/liquid-glass-06-phase-04-pass1`
- `checkpoint/liquid-glass-06-phase-04-polish-v2`
- `checkpoint/liquid-glass-06-phase-04-polish-v3`
- `checkpoint/liquid-glass-06-phase-04-polish-v4`
- `checkpoint/liquid-glass-06-phase-04-polish-v5`
- `checkpoint/liquid-glass-07-phase-05-pass1`
- `checkpoint/liquid-glass-07-phase-05-pass2`
- `checkpoint/liquid-glass-05-phase-03`
- `checkpoint/liquid-glass-06-phase-04`
- `checkpoint/liquid-glass-07-phase-05`
- `checkpoint/liquid-glass-08-phase-06`

## Commands

Create checkpoint tag for current commit:

```bash
git tag -a checkpoint/liquid-glass-XX-name -m "Liquid Glass checkpoint: XX-name"
```

List checkpoints:

```bash
git tag --list "checkpoint/liquid-glass-*"
```

Rollback working tree to checkpoint (detached HEAD):

```bash
git switch --detach checkpoint/liquid-glass-XX-name
```

Rollback branch hard to checkpoint:

```bash
git reset --hard checkpoint/liquid-glass-XX-name
```

## Registry

- `checkpoint/liquid-glass-00-base` -> branch baseline before implementation phases
- `checkpoint/liquid-glass-01-phase-00` -> local CSS runtime, generated CSS untracked, Docker builds styles in image
- `checkpoint/liquid-glass-02-phase-01` -> tokenized Tailwind foundation + `ui` primitive component package scaffold
- `checkpoint/liquid-glass-03-phase-02` -> layout shell migrated to `ui` chrome primitives (page shell/navbar/footer/progress)
- `checkpoint/liquid-glass-03-phase-02-polish` -> phase 02 cohesion polish (logo alignment, nav icon stroke/color, nav typography)
- `checkpoint/liquid-glass-03-phase-02-polish-v2` -> phase 02 visual polish iteration 2 (smaller nav logo, reduced icon framing, no hover/click flash)
- `checkpoint/liquid-glass-03-phase-02-polish-v3` -> phase 02 visual polish iteration 3 (delayed nav progress reveal to avoid fast-request flashing)
- `checkpoint/liquid-glass-03-phase-02-polish-v4` -> phase 02 visual polish iteration 4 (footer overlap/layout flow fix)
- `checkpoint/liquid-glass-04-phase-025` -> native motion phase (CSS-first HTMX swap/dialog/toast animations + reduced-motion support)
- `checkpoint/liquid-glass-04-phase-025-polish` -> delayed HTMX spinner reveal to avoid fast-request flash + docs update; includes limiter max bump to 100
- `checkpoint/liquid-glass-05-phase-03-pass1` -> phase 03 pass 1 (feed/video surface styling migration to `ui-*` classes, app dividers/empty states, video rail + toast refresh)
- `checkpoint/liquid-glass-05-phase-03-polish-v2` -> phase 03 visual polish (lighter feed/overlay typography, roomier action buttons, inset blurred in-card progress bar)
- `checkpoint/liquid-glass-05-phase-03-polish-v3` -> phase 03 visual polish (softer title contrast/weight + more translucent glass progress track/fill)
- `checkpoint/liquid-glass-05-phase-03-polish-v4` -> phase 03 visual polish (radius token scale + larger media radius + reduced progress-fill opacity)
- `checkpoint/liquid-glass-05-phase-03-polish-v5` -> phase 03 visual polish (consistent video inset/size token system + radius/button/padding alignment)
- `checkpoint/liquid-glass-05-phase-03-polish-v6` -> phase 03 visual polish (corner artifact fix + wider progress x-inset + action-button blur)
- `checkpoint/liquid-glass-06-phase-04-pass1` -> phase 04 pass 1 (subscriptions/channel/onboarding surface migration to channel/search/filter `ui-*` patterns)
- `checkpoint/liquid-glass-06-phase-04-polish-v2` -> phase 04 polish (channel action icon padding balance + mobile search overlay behavior + empty-search spacing + no-progress duration-chip alignment)
- `checkpoint/liquid-glass-06-phase-04-polish-v3` -> phase 04 polish (channel filter-tab mode switch now OOB-swaps a single centered wrapper to prevent left-align flicker)
- `checkpoint/liquid-glass-06-phase-04-polish-v4` -> phase 04 polish (subscribe/unsubscribe refresh-mode swap flicker fix + consistent blurred "Open" hover overlay across subscribed/search channel cards)
- `checkpoint/liquid-glass-06-phase-04-polish-v5` -> phase 04 polish (full-width integrated subscriptions search input + search section no-card layout + search results aligned to subscriptions grid behavior)
- `checkpoint/liquid-glass-07-phase-05-pass1` -> phase 05 pass 1 (supporting/fallback pages migrated to shared `ui-*` primitives; behavior/scripts unchanged)
- `checkpoint/liquid-glass-07-phase-05-pass2` -> phase 05 pass 2 (settings account/sync/sponsorblock surfaces and controls migrated to `ui-*` primitives while preserving endpoints/actions)
