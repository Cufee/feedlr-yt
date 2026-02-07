# Playlist Sync Design (YouTube OAuth)

## Status

Implemented.

This document captures the agreed direction:
- use **OAuth** (not per-user API keys),
- define feed as what the user sees in the app today,
- sync by applying a **delta** (diff), not rewriting the whole playlist.

## Problem

Some devices (for example TVs) cannot use the Feedlr web UI.  
We need to mirror a user’s Feedlr feed into a YouTube playlist so they can watch on those devices.

## Product Decisions

### 1) Auth model: OAuth

We will use per-user YouTube OAuth tokens for playlist write operations.

Why:
- YouTube playlist write endpoints require authorized user credentials.
- API keys are not sufficient for playlist mutation operations.
- For sync writes, use standard Google OAuth credentials we control (not scraped TV client credentials).

### 2) Feed definition: current app feed

The sync source must reflect what the user currently sees in the app feed.

Concrete source contract:
- source = `props.New + props.Watched` from `logic.GetUserVideosProps`,
- order = existing UI order (new section first, then watched section),
- excludes hidden videos (already filtered by current logic),
- excludes watch-later carousel (`props.WatchLater`) because it is a separate user-curated list.

Reference points:
- `internal/logic/videos.go` (`GetUserVideosProps`)
- `internal/server/routes/app/index.go`
- `internal/templates/pages/app/index.templ`

### 3) Sync strategy: diff-only

We will not clear/rebuild playlist contents.

Each run:
- read current remote playlist items,
- compute desired top 24 items from feed source,
- apply only missing inserts and obsolete deletes.

## Scope

### In scope
- Per-user OAuth connect/disconnect.
- Encrypted storage of per-user OAuth refresh token.
- Secret hash persisted with ciphertext for stale-secret detection.
- Scheduled background sync for all enabled users.
- Auto-create and persist a single target playlist per user.
- Keep exactly the last 24 source videos in that playlist.

### Out of scope
- Encryption key rotation workflow implementation.
- Multi-playlist sync.
- Syncing watch-later carousel.
- Full backfill beyond current feed behavior.

## Data Model

Add a new table for sync credentials and state (example name: `youtube_sync_accounts`):

- `id` (`text`, primary key)
- `created_at` (`date`, not null)
- `updated_at` (`date`, not null)
- `user_id` (`text`, not null, unique, FK `users.id` cascade delete)
- `refresh_token_enc` (`blob`, not null)  
  Encrypted refresh token payload.
- `enc_secret_hash` (`text`, not null)  
  Hash of runtime encryption secret used to encrypt `refresh_token_enc`.
- `playlist_id` (`text`, null)  
  YouTube playlist ID created/owned for this sync.
- `sync_enabled` (`boolean`, not null, default `true`)
- `last_feed_video_published_at` (`date`, null)  
  Source watermark of the latest feed item seen by sync.
- `last_synced_at` (`date`, null)
- `last_sync_attempt_at` (`date`, null)
- `last_error` (`text`, not null, default `''`)

Recommended indexes:
- unique (`user_id`)
- (`sync_enabled`, `last_synced_at`)

## Encryption & Secret Handling

Environment:
- `YOUTUBE_SYNC_ENCRYPTION_SECRET` (required in runtime environments)

Encryption approach:
- AEAD (`AES-256-GCM`) with random nonce per encryption.
- ciphertext payload format: `version || nonce || ciphertext` (stored as bytes/base64).
- key derivation from env secret using deterministic KDF.
- include stable context as AAD (for example user ID + version).

Secret hash:
- store `sha256(secret)` (hex) in `enc_secret_hash`.
- on read, compare current hash with stored hash:
  - if mismatch, mark record invalid/unavailable for sync,
  - do not attempt decryption or sync.

Notes:
- never log tokens or decrypted payloads,
- clear sensitive byte slices when practical.

## OAuth Integration

### Required OAuth scope

- `https://www.googleapis.com/auth/youtube`

### OAuth env/config

- `YOUTUBE_OAUTH_CLIENT_ID`
- `YOUTUBE_OAUTH_CLIENT_SECRET`
- `YOUTUBE_OAUTH_REDIRECT_URL`

### Flow

1. User starts connect flow from `/app/settings`.
2. App redirects to Google consent.
3. Callback exchanges code for tokens.
4. Persist encrypted refresh token in `youtube_sync_accounts`.
5. Kick off an immediate best-effort background sync for that user.
6. Keep access tokens in-memory/short-lived; refresh from stored refresh token.

State validation detail:
- The callback validates OAuth `state` against a short-lived HTTP-only cookie (`youtube_sync_state`) to protect against CSRF/session mixups.

### Rate-limit and reliability constraints

- Device flow has explicit rate-limiting behavior:
  - if polling too frequently: `slow_down` / HTTP 403,
  - if device-code requests exceed quota: `rate_limit_exceeded` / HTTP 403.
- The sync subsystem must be **isolated** from critical app startup:
  - sync auth/setup failures must not panic or stop the server,
  - sync worker should be best-effort and independently restartable.

Practical decision:
- Keep existing TV auth usage scoped to current playback metadata behavior.
- For playlist sync, use normal user OAuth login/callback from settings (no startup-time blocking auth).

## Sync Algorithm

Constants:
- `max_items = 24`
- `max_expensive_calls_per_sync` (configurable; default `4`)
  - Expensive calls are YouTube methods with quota cost `50`:
    `playlists.insert`, `playlistItems.insert`, `playlistItems.update`, `playlistItems.delete`.
- `insert_budget = floor(max_expensive_calls_per_sync / 2)`
- `delete_budget = max_expensive_calls_per_sync - insert_budget`

Per user sync:
1. Load user sync record; skip if disabled or invalid secret hash.
2. Build source list from current feed contract (`new + watched`) in current order.
3. Trim to `max_items` and derive ordered `desired_video_ids`.
4. Ensure target playlist exists:
   - if `playlist_id` missing or invalid, create playlist and persist ID.
5. Fetch current remote playlist items (first page up to 50; enough for 24-target set).
6. Compute delta:
   - `to_add = desired - remote`
   - `to_remove = remote - desired` (including overflow >24)
7. Apply mutations with a strict split budget:
   - process `to_add` in desired-order (newest first), up to `insert_budget`,
   - process `to_remove` from oldest/stalest remote items, up to `delete_budget`,
   - optional borrowing rule: if one side has no work, remaining calls can be used by the other side,
   - stop when the run budget is exhausted.
8. Update sync state:
   - `last_feed_video_published_at`,
   - `last_synced_at`,
   - clear/set `last_error`.

Recovery/robustness details:
- If stored `playlist_id` returns `playlistNotFound`, recreate playlist, persist new ID, and retry once.
- If there are no diff operations, `last_synced_at` still updates.
- If any mutation fails, sync stores `last_error` and does not advance `last_synced_at`.

Important:
- preserve deterministic ordering to avoid oscillating diffs.
- do not issue unnecessary updates/deletes if set is already correct.
- full convergence to 24 items may take multiple scheduler runs by design.
- keep behavior simple in v1: use insert/delete only (no `playlistItems.update` path).

## Scheduler

Add new cron env:
- `PLAYLIST_SYNC_CRON` (for example every 10–30 minutes)
- `PLAYLIST_SYNC_MAX_EXPENSIVE_CALLS` (default `4`)
- `PLAYLIST_SYNC_MAX_USERS_PER_TICK` (bounded worker intake)

Execution model:
- single scheduler process in app (same as current background tasks),
- bounded worker concurrency,
- per-user timeout,
- retry with backoff for transient failures,
- partial-failure tolerant (one user failure does not block others).
- global quota protection:
  - when quota/rate-limit errors spike, reduce worker throughput,
  - optionally stop writes until next quota window and continue read-only checks.

Integration point:
- extend `internal/logic/background/cron.go`.

## API & UI Plan

Settings page additions:
- connect/disconnect YouTube sync,
- enable/disable sync,
- show status (`connected`, `last_synced_at`, `last_error`),
- show a link to open the target playlist when available.

Server route additions (example):
- `POST /api/settings/youtube-sync/connect/begin`
- `GET /api/settings/youtube-sync/connect/callback`
- `POST /api/settings/youtube-sync/disconnect`
- `POST /api/settings/youtube-sync/toggle`

## Quota and Performance

Key point:
- Playlist mutations are relatively expensive versus list/read calls.

Optimizations:
- diff-only mutation strategy,
- cap source to 24 before diff,
- low concurrency with jitter to smooth spikes,
- cache short-lived per-user access tokens during one scheduler run.
- hard per-user write budget per run (`max_expensive_calls_per_sync`) with insert/delete split.

Operational guidance:
- Estimate upper bound:  
  `daily_write_units ≈ users_synced_per_day * avg_expensive_calls_per_sync * 50`
- Keep this bounded against project daily quota and reserve headroom for existing app traffic.

## Error Handling & Edge Cases

- Token revoked/expired:
  - mark error, disable sync or require reconnect.
- Playlist deleted externally:
  - recreate and persist new `playlist_id`.
- Secret hash mismatch:
  - mark “reconnect required”; skip sync.
- OAuth/device-flow rate-limit responses:
  - exponentially back off,
  - do not fail app startup,
  - persist error and retry later.
- Private/unavailable videos:
  - ignore failed insert for specific video, continue with rest.
- Empty feed:
  - remove remote items not in desired set (resulting playlist can become empty).

## Rollout Plan

1. Schema + model generation.
2. Encryption utility + unit tests.
3. OAuth connect/disconnect endpoints + settings UI.
4. Sync planner/diff engine + unit tests.
5. Cron worker integration + operational logs.
6. Gradual enablement (feature flag or admin-only first).

## Test Plan

- Crypto roundtrip + secret-hash mismatch tests.
- DB CRUD for sync account records.
- Feed-to-desired-list deterministic ordering tests.
- Diff planner tests:
  - no-op,
  - add-only,
  - remove-only,
  - mixed add/remove,
  - trim-to-24 behavior.
- Worker tests for timeout/retry/error-state transitions.
