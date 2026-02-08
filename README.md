# [Feedlr](https://feedlr.app)

Feedlr is an alternative frontend for YouTube. The main goal is to make following your favorite creators simpler and reduce doom scrolling.

## Approach

- No recommendation, play-next, or related videos
- No channel discovery; you need to know who you want to follow
- A simpler feed split into new and watched videos
- Embedded sponsor segments can be skipped with SponsorBlock

## Current State

Core functionality is complete and working reliably. Implemented in this repository:

- Passkey auth (WebAuthn) with sessions
- Subscriptions flow (search, subscribe/unsubscribe, per-channel filters)
- Feed pages (`/app`, `/app/recent`, `/app/watch-later`, onboarding)
- Watch later playlist and cleanup task
- YouTube playlist sync via OAuth (`Feedlr Sync` playlist)
- YouTube TV lounge sync (pairing, progress sync, SponsorBlock skip)
- Background cron jobs for cache and sync tasks
- Prometheus metrics endpoint (`METRICS_PORT` / `METRICS_PATH`)

For implementation details, see `docs/`.

## Stack

- Go 1.25
- Fiber
- Templ + HTMX + Hyperscript
- Tailwind CSS v4 + DaisyUI v5
- SQLite + SQLBoiler

## Setup Examples

### 1. Local development (recommended)

`task dev` runs:
- Go with `-tags dev` (mock auth middleware)
- Tailwind in watch mode

Prerequisites:
- Go 1.25+
- Node.js + npm
- `task` (Taskfile runner)
- `air`
- `atlas` (for migrations)
- `sqlboiler` (used by `task generate` / migration flow)

Example `.env`:

```bash
cat <<EOF > .env
PORT=3000
YOUTUBE_API_KEY=replace-me
DATABASE_PATH=$(pwd)/tmp/database/local.db
DATABASE_DIR=$(pwd)/tmp/database
SPONSORBLOCK_API_URL=https://sponsor.ajay.app/api
VIDEO_CACHE_UPDATE_CRON=0 0 * * *
YOUTUBE_SYNC_ENCRYPTION_SECRET=replace-me
YOUTUBE_OAUTH_CLIENT_ID=replace-me
YOUTUBE_OAUTH_CLIENT_SECRET=replace-me
YOUTUBE_OAUTH_REDIRECT_URL=http://localhost:3000/api/settings/youtube-sync/connect/callback
PLAYLIST_SYNC_CRON=*/30 * * * *
PLAYLIST_SYNC_MAX_EXPENSIVE_CALLS=4
PLAYLIST_SYNC_MAX_USERS_PER_TICK=100
MAINTENANCE_MODE=false
COOKIE_DOMAIN=localhost:3000
METRICS_PORT=9090
METRICS_PATH=/metrics
EOF
```

Run locally:

```bash
mkdir -p tmp/database
npm install
task migrate-apply
task dev
```

Open `http://localhost:3000`.

Notes:
- `YOUTUBE_OAUTH_CLIENT_ID` / `YOUTUBE_OAUTH_CLIENT_SECRET` must be non-empty at startup.
- The YouTube auth client may ask for device authentication on first run (check server logs).

### 2. Production-style local run

```bash
npm install
go generate ./...
npm run build
go run .
```

Use this mode when you want the non-dev runtime behavior (passkeys + production middleware path).

## Common Commands

```bash
# Development (Go + Tailwind watch)
task dev

# Test suite
task test

# Generate templ/sqlboiler artifacts
task generate

# Apply DB migrations
task migrate-apply
```

## Documentation

- `docs/ARCHITECTURE.md`
- `docs/FRONTEND.md`
- `docs/DATABASE.md`
- `docs/YOUTUBE-API.md`
- `docs/PLAYLIST-SYNC.md`
- `docs/TV-PROGRESS-SYNC.md`
- `docs/OBSERVABILITY.md`
