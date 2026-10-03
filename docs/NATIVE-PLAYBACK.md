# Native YouTube playback

Feedlr plays prerecorded YouTube videos through a private Invidious Companion and locally bundled Shaka Player. When native playback is enabled, signed-in viewers use it by default. Guests, live/upcoming videos, unsupported browsers, and playback failures use the YouTube iframe. Podcasts keep their existing player. No full Invidious installation or database migration is required.

## Player behavior

Each page load requests native playback for signed-in viewers; the server selects iframe if its playback checks fail. The native settings menu includes **Use YouTube player**, which switches to iframe while preserving position, playing/paused state, volume, mute, and supported playback rate. Both manual and automatic fallback stay active for the current page. Refreshing retries native playback, subject to the server's health checks and circuit retry interval. Player mode is not persisted between page loads.

Quality lives in Shaka's settings menu: **Auto**, supported resolutions, and **Audio only**. Resolutions use familiar labels such as **1080p**, **2K**, and **4K**, including normalized portrait/ultrawide labels. The choice persists across videos and reloads in `localStorage` under `feedlr-player-quality`:

- A manual resolution selects the highest supported quality at or below the saved ceiling. If every available quality is higher, it selects the lowest available. Playing a lower-resolution video does not overwrite the saved ceiling.
- Auto clears manual restrictions and enables adaptive quality. Automatic adaptation never overwrites the preference.
- Audio only requests an audio-only manifest and session resource list. The viewer downloads no video media, and sees a static translucent accent waveform on black without a visible text label. Switching back to a resolution or Auto restores video at the current position. Shared instance health probes still sample both audio and video.

The iframe manages its own quality; returning to Feedlr restores the native preference. Both players share progress reporting, hotkeys, SponsorBlock, and teardown through `assets/js/feedlr-player.js`.

On mobile devices, the native player hides mute and volume controls and uses 100% player volume, leaving loudness to the device's volume buttons. Desktop keeps the volume slider visible without hover expansion and retains its saved volume preference.

Desktop uses a single click to play/pause and a double click to toggle fullscreen. Mobile uses taps to show/hide controls, a central play/pause button, and double taps on the sides to seek backward/forward by 10 seconds. Mobile fullscreen uses its button; device rotation does not trigger fullscreen. These gestures use Shaka's built-in controls. In the pinned Shaka version, the first tap with hidden controls only reveals them and does not count toward double-tap seeking. Hold for temporary 2× playback is not implemented.

## Deployment

Docker Compose enables native playback by default. `.env.example` sets it to **off** for deployment acceptance; `NATIVE_PLAYBACK_ENABLED=false` remains the global iframe rollback switch.

1. Generate a secret with `openssl rand -hex 8` and set `COMPANION_SECRET` in `.env`. Companion requires exactly 16 alphanumeric characters.
2. Set `COMPANION_URL=http://companion:8282`. Compose passes the shared secret to both services and enables signed manifest requests.
3. Deploy the stack with `docker compose up -d --build`. Companion starts with the other services; no Compose profile is required, including in Dokploy.
4. On the acceptance deployment, set `NATIVE_PLAYBACK_ENABLED=true` and recreate Feedlr with `docker compose up -d --build feedlr-service`.

Companion has no published ports or public proxy route. It shares the dedicated `playback` network only with Feedlr and uses the writable `companion-cache` volume at `/var/tmp/youtubei.js`; its root filesystem is read-only. The image's `/healthz` check establishes process liveness, not successful YouTube playback. Companion being unavailable does not prevent Feedlr from starting.

Keep extraction and streaming on the same stable outbound IP. Feedlr routes both through Companion; avoid rotating egress. Playback sessions are in memory, so multiple Go instances require session affinity. A restart invalidates sessions and triggers browser renewal or fallback.

### Dependency versions

Compose tracks Companion's `:latest` tag with Watchtower updates enabled. Shaka is pinned in npm. The exact Companion build used for local validation is recorded below for reproducibility.

| Dependency | Pin |
|---|---|
| Tested Companion revision | `bb3b37ff40c69475e45785d16eb7da8876b80089` |
| Tested Companion image | `quay.io/invidious/invidious-companion@sha256:f15bc688aedd4bbb76cc636c7cc0cfff9b2b9f0734a4b831efbda7253b180525` |
| Shaka Player | `5.2.12`, with npm lockfile integrity |

The tested image index supports amd64 and arm64; local playback validation used amd64. Its `quay.expires-after=12w` label makes a retained registry mirror useful for reproducing that build. Repeat playback acceptance after Companion or Shaka upgrades.

Run `npm ci && npm run build` before building Go locally. The build copies pinned Shaka scripts, CSS, and license into ignored `assets/vendor/`, removes the optional remote font, and embeds the assets in the binary. Docker performs these steps automatically. Shaka loads locally when native playback is needed; the iframe API loads only when iframe playback is needed.

See the upstream [installation guide](https://docs.invidious.io/installation/), pinned [configuration parser](https://github.com/iv-org/invidious-companion/blob/bb3b37ff40c69475e45785d16eb7da8876b80089/src/lib/helpers/config.ts), and [HTTP API](https://github.com/iv-org/invidious-companion/wiki/How-to-communicate-with-Invidious-companion-with-any-client-%28HTTP-API%29).

## Server and recovery

`POST /api/videos/:id/playback` reads current saved progress and returns player mode, reason, and, for native playback, a same-origin manifest URL and expiry. Explicit iframe requests bypass Companion. The optional `audioOnly` request selects the audio-only session. Expiring upstream URLs and Companion credentials remain server-side.

Authenticated manifest and media routes bind opaque sessions to users. Native Fiber handlers stream ranges directly on the application port, bypassing the buffering adapter and generic API limiter. Stream concurrency is bounded; upstream bodies close and requests cancel when streaming ends. Read-only session authentication avoids database refresh writes for each range.

Shared health checks validate 64 KiB each of audio and video, alternating initial/interior ranges. Success is cached for two minutes, concurrent checks coalesce, and periodic checks stop ten minutes after the last playback resolution request. Continuous probing transfers about 90 MiB/day per instance, excluding metadata and retries. Failure of both public probe videos opens a circuit for new playback requests; recovery requires both probes to pass, with retry backoff from one to five minutes. Individual video failures do not establish a global outage, and health failures do not stop functioning players. Initial resolution is bounded to eight seconds.

The browser renews URLs within two minutes of expiry during active playback and rechecks on resume/history restoration. Renewal preserves current position and rejects already-expired URLs returned from Companion's cache. Fatal errors, unsupported playback, missing startup frames, and sustained stalls fall back to iframe; plausible expiry errors get one fresh resolution attempt first. Startup/stall timers pause while paused, hidden, offline, or autoplay-blocked.

See [Observability](OBSERVABILITY.md) for reason-coded playback events and startup latency. Keep signed URLs and secrets out of shared diagnostics. Companion logs its configuration, including its secret, at startup; restrict access to those logs and redact before sharing.

## Verification and rollout

```sh
npm ci
npm run build
npm test
go generate ./...
go test ./...
go vet ./...
docker compose config --quiet
```

Automated checks cover controller state/lifecycle behavior, manifest rewriting, authorization, health recovery, and streaming with fake Companion and socket tests. Local browser verification exercised the integrated page in Chromium and Firefox, including native-to-iframe switching. Audio-only playback was verified with zero video media bytes. Local playback checks also covered seeking, quality selection, state preservation, and simulated expiry recovery.

Remaining production acceptance checks:

- Production-host egress and reverse-proxy behavior, concurrent viewing, and a seek well into a multi-hour video.
- Natural URL expiration, paused/stale tabs, and real saved-progress persistence across application sessions.
- Safari/macOS, Safari/iOS, unsupported-device fallback, and arm64 deployment if used.

Keep iframe fallback available permanently.
