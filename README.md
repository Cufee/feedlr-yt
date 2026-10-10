# [Feedlr](https://feedlr.app)

Feedlr is a subscription reader for YouTube channels and podcasts. The main goal is to make following your favorite creators simpler and reduce doom scrolling.

## Approach

- No recommendation, play-next, or related videos
- No channel discovery; you need to know who you want to follow
- A simpler feed split into new and watched items
- Embedded sponsor segments can be skipped with SponsorBlock

## Current State

Core functionality is complete and working reliably. Implemented in this repository:

- Passkey auth (WebAuthn) with sessions
- YouTube and podcast RSS subscriptions
- Watch later playlist
- YouTube playlist sync via OAuth (home feed, Watch Later, and all user/imported playlists)
- YouTube TV lounge sync (pairing, progress sync, SponsorBlock skip, send video to an online TV)
- Native player through YouTube proxy
- YouTube chapter navigation and previews with a SponsorBlock-aware timeline
- Cached podcast sponsor scanning with optional OpenRouter transcript generation
- Background cron jobs for cache and sync tasks

## Setup Examples

### 1. Local development (recommended)

`task dev` runs:

- Go with `-tags dev` (mock auth middleware)
- Tailwind in watch mode
- Companion in Docker/Podman, with its local port taken from `COMPANION_URL`

Copy `.env.example` to `.env` and fill in the database paths and API credentials.
Set `COMPANION_URL="http://localhost:18282"` and `COMPANION_SECRET` to 16 random
alphanumeric characters (`openssl rand -hex 8`); both are required by `task dev`.
Set `NATIVE_PLAYBACK_ENABLED="true"` to use Companion for playback.
Docker Compose must be available;
Podman with a Compose provider also works.

Run locally:

```bash
mkdir -p tmp/database
npm install
task migrate-apply
task dev
```

Open `http://localhost:3000`.

`task dev` waits for Companion's HTTP health check before starting the app.
Companion stays running between dev sessions; `task dev:companion:stop` stops it
and preserves its cache. `task dev:companion` starts it independently.

Notes:

- YouTube playlist sync starts only when `YOUTUBE_OAUTH_CLIENT_ID`, `YOUTUBE_OAUTH_CLIENT_SECRET`, `YOUTUBE_OAUTH_REDIRECT_URL`, and `YOUTUBE_SYNC_ENCRYPTION_SECRET` are all set.
- PodcastIndex credentials enable catalog search. Users can subscribe to a direct RSS feed URL without them.
- The YouTube auth client may ask for device authentication on first run. Check the server logs.

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
# Development (Companion + Go + Tailwind watch)
task dev

# Stop the local Companion container
task dev:companion:stop

# Test suite
task test

# Generate templ/sqlboiler artifacts
task generate

# Apply DB migrations
task migrate-apply
```
