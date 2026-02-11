# Liquid Glass Checkpoints

This branch (`ai-slop/tailwind-liquid-glass`) is the rollout base.

Use annotated git tags for rollback checkpoints after each completed phase.

## Naming Convention

- `checkpoint/liquid-glass-00-base`
- `checkpoint/liquid-glass-01-phase-00`
- `checkpoint/liquid-glass-02-phase-01`
- `checkpoint/liquid-glass-03-phase-02`
- `checkpoint/liquid-glass-04-phase-025`
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
