# Playlist Sync (YouTube OAuth)

Feedlr exports the home feed and all Feedlr playlists to private YouTube playlists.

## Sources

| Feedlr source | YouTube destination | Contents |
| --- | --- | --- |
| Home feed | Existing `Feedlr Sync` | First 36 YouTube videos from `New + Watched` |
| Watch Later | `Feedlr: Watch Later` | All YouTube videos, newest additions first |
| User and imported playlists | `Feedlr: <name>` | All YouTube videos, in saved position order |

Podcast episodes are excluded. Local playlist exports also skip private videos
and unavailable local video records. Duplicate video IDs are removed.

Export is one-way from Feedlr. The separate import/refresh feature still uses
`playlists.youtube_playlist_id` as its source; exports never write to that ID.
Each imported playlist gets its own export destination.

Name and description changes update the export. Deleting a local playlist stops
exporting it and leaves its YouTube copy in place. Disconnecting removes credentials
and mappings, leaving YouTube copies; reconnecting creates new destinations.

## Credentials and storage

OAuth uses `https://www.googleapis.com/auth/youtube` and requires:

- `YOUTUBE_OAUTH_CLIENT_ID`
- `YOUTUBE_OAUTH_CLIENT_SECRET`
- `YOUTUBE_OAUTH_REDIRECT_URL`
- `YOUTUBE_SYNC_ENCRYPTION_SECRET`

Refresh tokens use AES-256-GCM encryption with a random nonce and user ID as
an authenticated context. A saved secret hash detects stale secrets; rotated refresh
tokens are persisted.

`youtube_sync_accounts` holds credentials, the legacy feed destination, enable
state, and account-level run timestamps/errors.

`youtube_sync_targets` holds one row per account/source, with:

- `source_id`: `feed` or `playlist:<local ID>`
- `playlist_id`: YouTube export destination
- `title` and `description`: last successfully exported metadata
- `last_attempt_at`: persistent scheduling order

Mappings cascade on account deletion. Apply migration
`20260915000000_add_youtube_sync_targets.sql` before running the updated worker.

## Sync algorithm

1. Build the feed and enumerate local playlists, including Watch Later.
2. Sort sources by oldest attempt, with source ID as a deterministic tie-break.
   New sources get a turn first.
3. Share one write budget across all destinations (currently four writes per
   user/run). Creation, metadata updates, item inserts/deletes/moves, and failed
   writes all count.
4. Persist the source attempt timestamp and load its desired contents.
5. Create a private destination if missing. Preserve the existing feed destination
   on upgrade. Recreate destinations deleted remotely and persist their new IDs.
6. Read every page of remote items. An incomplete read aborts that source before
   item mutations.
7. Use the delta planner to remove obsolete/duplicate items and insert missing
   videos. Insert/delete allocations can borrow unused capacity.
8. Once membership is correct, move existing items to match source order. After
   inserts/deletes, re-read positions on a later run before reordering.
9. Store account run status. Success may be partial when the budget is exhausted;
   large playlists converge over multiple runs.

Sources rotate between runs so a busy feed or failing playlist cannot monopolize
writes. Unchanged sources consume reads but no writes. Runs are serialized to
prevent callback/scheduler overlap from creating duplicate destinations.

A failed mutation stops that source; other sources can use the remaining budget.
Later insert positions are never used after an earlier insert fails. The existing
list retry handles temporary playlist-not-found responses.

## Scheduler and settings

The existing cron worker processes enabled accounts, up to 100 per tick, with a
one-minute context per account. Settings support connect/disconnect, pause/resume,
status, and a link to the feed destination. Other destinations appear in the
connected YouTube account's playlists. Pause applies to every export.

## Verification

Tests cover feed destination reuse, separate exports, Watch Later, imported
playlists, more than 50 items, ordering, renaming, emptying, no-op runs, shared
budgets, fairness, failed inserts, remote deletion, incomplete-page failures, and
database persistence/account isolation/disconnect cascade.

Key files:

- `internal/logic/youtube_sync.go`: OAuth, run status, feed source, delta planner
- `internal/logic/youtube_sync_playlists.go`: source enumeration and export
- `internal/database/youtube_sync.go`: credentials, mappings, run state
- `internal/templates/components/settings/youtube-sync.templ`: settings UI

API references: [pagination](https://developers.google.com/youtube/v3/docs/playlistItems/list),
[item ordering](https://developers.google.com/youtube/v3/docs/playlistItems/update),
[metadata updates](https://developers.google.com/youtube/v3/docs/playlists/update).
