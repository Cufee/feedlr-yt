# TV Progress Sync Design (Go Lounge Client)

## Status

Implemented.

This document captures the implementation plan for:
- syncing playback progress between TV playback and Feedlr,
- auto-skipping SponsorBlock segments during TV playback,
- managing long-lived TV lounge connections at scale.

## Product Goal

When a paired TV starts playing a video we already know, resume from Feedlr's saved position.
While TV playback continues, keep Feedlr progress updated from TV state.
Automatically skip SponsorBlock segments on TV playback.

## Non-Negotiable Constraints

1. **Go-only implementation** (no Python sidecar, no external process per user).
2. **Sync decision logic stays in `internal/logic`** (database layer remains CRUD only).
3. **Connection lifecycle is bounded**:
   - disconnect users who have been inactive for days,
   - disconnect/stall-recover connections with no events for a configured interval,
   - expose this state in Settings UI.
4. **Consistency-first implementation**:
   - explore similar existing features and utilities before adding new functions,
   - reuse existing secrets/crypto/helpers when they fit,
   - do not force reuse of patterns that make the TV sync flow harder to understand or maintain.

## Codebase Alignment Principles

- Keep behavioral logic in `internal/logic`, keep `internal/database` focused on persistence operations.
- Prefer extending existing YouTube sync surfaces (settings, routes, service wiring) before creating parallel structures.
- Reuse existing utility patterns (hashing, crypto helpers, relative-time UI display) when they map cleanly to the problem.
- Keep naming explicit and local to feature scope (`tv sync` / `youtube lounge`) to avoid ambiguity with playlist sync.
- If a reused pattern introduces hidden coupling or unclear ownership, prefer a clearer TV-specific implementation.

## Problem

Current app behavior already supports:
- web player progress reporting (`/api/videos/:id/progress`),
- web player SponsorBlock skip (client-side).

But it does not maintain a TV lounge session that can:
- observe active TV playback,
- seek the TV playhead,
- keep app progress updated from TV playback.

## Scope

### In scope

- Lounge pairing and encrypted token storage.
- Single paired TV device per user.
- Long-lived per-user TV connection tasks in Go.
- Resume-on-start (seek TV to saved progress).
- TV -> Feedlr progress updates with the same simple write behavior as app progress reporting.
- SponsorBlock skip on TV playback (no ad mute/skip logic).
- UI status for connection health/inactivity pause reasons.

### Out of scope (v1)

- Multi-device support (not planned).
- Cross-process distributed coordination.
- Full remote-control feature set (dpad, captions, autoplay controls).
- Ad skip/mute behavior.

## Architecture

### 1) New package: `internal/api/youtube/lounge/`

Responsibilities:
- Pairing endpoints (`get_screen`, `get_lounge_token_batch`).
- Connect/bind stream management (`bc/bind` session handling).
- Command API (`seekTo`, `getNowPlaying`, optional `setPlaylist` only when explicitly requested).
- Event decoding for:
  - `nowPlaying`
  - `onStateChange`
  - `onPlaybackSpeedChanged`
  - `loungeScreenDisconnected`
  - `noop` / heartbeat events

Notes:
- Implement command serialization (mutex) to avoid RID/offset races.
- Implement watchdog based on "time since last event".
- Refresh now-playing state after playback-speed changes.
- Poll `getNowPlaying` periodically while connected and an active video is known, to ensure steady progress updates without waking idle TV app sessions.

### 2) New logic service: `internal/logic/youtube_tv_sync.go`

Responsibilities:
- Per-user session orchestration.
- Resume and progress sync decisions.
- SponsorBlock timing/skip decisions.
- Connection lifecycle policy (pause/disconnect/reconnect).

This service owns all behavioral rules and calls database interfaces only for storage/retrieval.

Service wiring should mirror current YouTube sync patterns:
- constructor in `internal/logic`,
- optional global default service reference for route usage,
- route-level helper that returns "disabled" when service is unavailable.

### 3) Database layer: CRUD-only interfaces

New DB interfaces under `internal/database/` should provide:
- paired device/token records,
- connection status persistence,
- last user activity lookup,
- progress row read/update.

No "merge progress" policy in DB client methods.

### 4) Background lifecycle integration

Extend `internal/logic/background/cron.go` with:
- manager tick for connection orchestration,
- stale session cleanup tick.

All tasks run in the main Go process.

## Data Model

Add table `youtube_tv_sync_accounts`:

- `id` (`text`, primary key)
- `created_at` (`date`, not null)
- `updated_at` (`date`, not null)
- `user_id` (`text`, not null, unique, FK `users.id` cascade delete)
- `screen_id` (`text`, not null)
- `screen_name` (`text`, not null default `''`)
- `lounge_token_enc` (`blob`, not null)
- `enc_secret_hash` (`text`, not null)
- `sync_enabled` (`boolean`, not null, default `true`)
- `connection_state` (`text`, not null, default `'disconnected'`)
- `state_reason` (`text`, not null, default `''`)
- `last_connected_at` (`date`, null)
- `last_event_at` (`date`, null)
- `last_disconnect_at` (`date`, null)
- `last_user_activity_at` (`date`, null)
- `last_video_id` (`text`, null)
- `last_error` (`text`, not null, default `''`)

Recommended indexes:
- unique (`user_id`)
- (`sync_enabled`, `connection_state`)
- (`last_event_at`)

`views` table remains source-of-truth for progress. No schema change required for v1.

## Security and Secret Handling

Environment:
- `YOUTUBE_SYNC_ENCRYPTION_SECRET` (required, shared with playlist sync)

Use the same encryption implementation as playlist sync:
- AES-GCM with random nonce,
- persisted secret hash for stale-secret detection,
- never log decrypted token material.

Implementation note:
- do not introduce a TV-specific encryption secret,
- extract/reuse the existing `youtubeSyncCrypto` logic so both playlist sync and TV sync share one crypto path.

All non-secret behavior/tuning values in this feature should be code-level constants in `internal/logic/youtube_tv_sync.go`.
For this project size, rebuilding for changes is acceptable and keeps runtime configuration simpler.

## Connection Lifecycle Policy

### User inactivity cutoff (days scale)

If user has no active session usage (`sessions.last_used`) for configured days:
- close lounge subscription,
- mark `connection_state = 'paused_inactive_user'`,
- keep pairing credentials stored.

Default constant:
- `tvSyncUserInactiveDays = 14`

### Event inactivity cutoff

If no lounge events for configured duration:
- cancel subscription task,
- mark `connection_state = 'paused_no_events'`,
- schedule reconnect with backoff.

Default constants:
- `tvSyncNoEventTimeout = 60 * time.Second`
- `tvSyncReconnectMin = 10 * time.Second`
- `tvSyncReconnectMax = 5 * time.Minute`

### Manual and fatal states

Other states:
- `connected`
- `connecting`
- `disabled_by_user`
- `error`
- `disconnected`

All states should be shown in Settings UI with human-readable reason and relative timestamp.

## Sync Algorithm

### Resume-on-start

Trigger:
- new video where current TV position is within the start window,
- transition into playing state (`state=1`) for an already-known video in the connection session.

Steps:
1. Load saved progress from `views.progress`.
2. Determine whether this event represents playback start (new video near start, or play-state transition).
3. If saved progress is > `0` and current position is within start window, send `seekTo(saved)`.
4. If current position is beyond start window, seek only when saved progress is ahead by a minimum threshold.
5. Mark a single in-memory resume flag for the active video so this resume logic is evaluated once per active video.
6. Skip persisting progress for that same event to avoid writing pre-seek timestamps.

### Progress ingest (TV -> Feedlr)

Process events while playing:
1. Normalize candidate progress:
   - floor seconds,
   - clamp to `[0, duration]` when duration known.
2. Ensure the video exists in local cache before writing progress (fetch/cache when missing).
3. Persist at most once per `write_interval` using the same view write path used by the app.
4. After write, run existing "remove watch-later if fully watched" rule.

Default constants:
- `tvSyncProgressWriteIntervalSec = 10`
- `tvSyncResumeStartWindowSec = 90`
- `tvSyncResumeAheadThresholdSec = 8`
- `tvSyncNowPlayingPollIntervalSec = 5`
- `tvSyncVideoCacheRetryIntervalSec = 60`

### SponsorBlock skip on TV

For each new `video_id`:
1. Fetch SponsorBlock segments (reuse current categories/settings behavior).
2. Normalize:
   - merge overlapping/adjacent segments,
   - ignore segments shorter than minimum length.
3. During playback, if current time is inside an unskipped segment:
   - seek to segment end,
   - mark segment as skipped for this video/session,
   - apply cooldown to avoid skip loops.

Default constants:
- `tvSyncMinSkipLengthSec = 1`
- `tvSyncSkipCooldown = 1200 * time.Millisecond`

## API and UI Plan

### Settings/API endpoints (new)

- `POST /api/settings/youtube-sync/tv/connect` (pair via TV code)
- `POST /api/settings/youtube-sync/tv/disconnect`
- `POST /api/settings/youtube-sync/tv/toggle`

This keeps TV sync grouped under existing YouTube sync settings routes.

### Settings UI component

Extend existing YouTube sync settings UI with a TV sync subsection:
- Pair/Disconnect controls.
- Enable/Disable toggle.
- Current state badge (`Connected`, `Paused: inactive`, `Paused: no events`, `Error`, etc.).
- `last_event_at`, `last_connected_at`, `last_user_activity_at`, `last_error`.

This explicitly surfaces inactivity-based disconnections and auto-recovery state.

## Quota / Request Cost Expectations

Per connected user:
- one long-lived bind subscription stream,
- occasional reconnect attempts,
- one SponsorBlock fetch per new video,
- low-frequency command writes (`seekTo`, `getNowPlaying`),
- one DB progress write every ~10s while actively playing.

This is mainly connection-concurrency cost, not high request-volume cost.
Use inactivity policies above to bound idle connections.

## Implementation Plan

### Phase 1: Protocol + storage foundation

1. Add migration for `youtube_tv_sync_accounts`.
2. Add DB CRUD interfaces and models.
3. Review existing YouTube sync and utility helpers; reuse only where it improves clarity.
4. Reuse/extract existing YouTube sync crypto helper for token encryption/decryption.
5. Implement `internal/api/youtube/lounge` connect/subscribe/command primitives.
6. Unit tests for event parsing and command sequencing.

### Phase 2: Core sync logic

1. Implement `TVSyncService` in `internal/logic`.
2. Add resume-on-start logic.
3. Add direct progress update writes aligned with app behavior.
4. Add SponsorBlock skip scheduler with loop protection.
5. Keep sync state in-memory only (no additional persistence table for v1).
6. Unit tests for resume trigger and skip-decision logic.

### Phase 3: Lifecycle + UI integration

1. Add manager ticks in `internal/logic/background/cron.go`.
2. Add inactivity/no-event disconnect policies.
3. Add Settings API + templ subsection under existing YouTube sync settings.
4. Add state/status props to `internal/types`.
5. Add integration tests for lifecycle transitions.

### Phase 4: Rollout hardening

1. Add structured logs + metrics counters:
   - connect/disconnect counts,
   - reconnect reason,
   - progress update counts,
   - sponsor skip count.
2. Validate with staging users.
3. Tune thresholds based on observed behavior.

## Testing Plan

- Unit:
  - lounge chunk/event parser,
  - progress write interval behavior,
  - resume decision thresholds,
  - skip-loop prevention.
- Integration:
  - fake lounge server for connect/subscribe/reconnect flows,
  - inactivity transition scenarios,
  - settings status rendering.
- Regression:
  - ensure web player progress path still works unchanged.

## Decisions

1. Device model: single paired TV device per user; multi-device support is not planned.
2. Activity source for idle disconnect: use `sessions.last_used` as the sole signal in v1.
3. Keep sync state in-memory only; do not add extra tables for v1.

## File Reference (expected)

- `internal/api/youtube/lounge/*` (new)
- `internal/logic/youtube_tv_sync.go` (new)
- `internal/logic/background/cron.go` (update)
- `internal/database/youtube_tv_sync.go` (new)
- `internal/database/migrations/*_add_youtube_tv_sync_accounts.sql` (new)
- `internal/server/routes/api/youtube_sync.go` (update, add TV sync handlers) or `internal/server/routes/api/youtube_tv_sync.go` (new if file grows too large)
- `internal/templates/components/settings/youtube-sync.templ` (update with TV subsection) or `internal/templates/components/settings/youtube-tv-sync.templ` (new if separation improves clarity)
- `internal/types/props.go` (update)
