# Observability Plan (Prometheus + Grafana)

This document defines the metrics exposure model and concrete rollout plan for visibility across app, user actions, YouTube integrations, and background refresh work.

## Metrics Endpoint

- Dedicated listener port: `METRICS_PORT` (default: `9090`)
- Endpoint path: `METRICS_PATH` (default: `/metrics`)
- Auth: none (intended to be reachable only on internal/private network paths)
- Format: standard Prometheus text format (`promhttp`).

## Metrics Added

All metrics use the `feedlr` namespace.

- `feedlr_http_requests_total{method,route,status_class}`
  - Global request volume and status distribution per route.

- `feedlr_users_events_total{event,outcome}`
  - Session lifecycle and user auth/account events.

- `feedlr_user_actions_total{action,outcome}`
  - Product actions from API/auth routes (watch-later, progress, subscriptions, sync toggles, passkey flows, login/register, etc.).

- `feedlr_youtube_api_calls_total{client,operation,outcome}`
  - YouTube Data API + player API operations (playlist/channel/search/player fetch).

- `feedlr_youtube_oauth_calls_total{operation,outcome}`
  - Device flow/token refresh/context fetch operations.

- `feedlr_youtube_tv_calls_total{operation,outcome}`
  - Lounge API connect/subscribe/command/pair/token refresh operations.

- `feedlr_video_refresh_operations_total{operation,outcome}`
  - Channel/video cache refresh operation outcomes.

- `feedlr_video_refresh_items_total{operation}`
  - Number of videos processed by refresh/cache jobs.

- `feedlr_background_tasks_total{task,outcome}`
  - Cron execution outcomes.

- `feedlr_youtube_tv_sync_events_total{event,outcome}`
  - TV sync connects/disconnects/reconnects/progress/sponsor skip events.

## Concrete Rollout Plan

1. Enable secure scraping:
   - Ensure only private/internal network access reaches `METRICS_PORT`.
   - Set `METRICS_PATH` if your scrape path is non-default.
2. Configure Prometheus scrape job:
   - Target app service on `METRICS_PORT`.
3. Build Grafana dashboards:
   - API error-rate by route (`feedlr_http_requests_total`).
   - User auth funnel (`feedlr_users_events_total`, `feedlr_user_actions_total`).
   - YouTube dependency health (`feedlr_youtube_api_calls_total`, `feedlr_youtube_oauth_calls_total`, `feedlr_youtube_tv_calls_total`).
   - Background/video refresh throughput (`feedlr_background_tasks_total`, `feedlr_video_refresh_*`).
   - TV sync reliability (`feedlr_youtube_tv_sync_events_total`).
4. Add alert rules:
   - YouTube API/OAuth error ratio spikes.
   - TV connect/reconnect failure spikes.
   - Repeated background task failures.
5. Follow-up improvements:
   - Add histograms for latency-sensitive external calls.
   - Add gauges for connected TV workers and sync queue depth.
