# Podcasts

Feedlr stores podcast shows as channels and podcast episodes as videos. This keeps progress, watch later, playlists, and feed cards shared between YouTube and podcasts.

## Subscribe to a podcast

Open **Subscriptions**. You can search the PodcastIndex catalog or add a podcast RSS feed URL.

Podcast catalog search needs both environment variables:

```bash
PODCASTINDEX_API_KEY=your-key
PODCASTINDEX_API_SECRET=your-secret
```

RSS feed URL subscriptions do not need PodcastIndex credentials. Feedlr fetches the feed when the user subscribes, then stores the show and its most recent episodes.

## Refresh feeds

`PODCAST_CACHE_UPDATE_CRON` controls the background RSS refresh schedule. The default is every six hours.

Podcast feeds use ETag and Last-Modified headers when the publisher provides them. Feedlr also refreshes a show when a user selects **Refresh** on the show page. Manual refreshes use the same one-hour throttle as YouTube channels.

## Playback

Episode pages use a custom HTML audio player. Feedlr reports progress through the existing video-progress endpoint. The media route redirects the browser to the episode enclosure URL. Feedlr does not proxy audio bytes.
