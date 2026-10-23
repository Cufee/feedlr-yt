import type { MediaDetail } from "@/lib/api/types";

const cover = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="600" height="600" viewBox="0 0 600 600"><rect width="600" height="600" fill="#27242f"/><circle cx="300" cy="275" r="150" fill="#b69ad7"/><path d="M255 230v90m45-130v170m45-130v90" stroke="#27242f" stroke-width="20" stroke-linecap="round"/><text x="300" y="505" font-family="sans-serif" font-size="35" fill="#f1edf4" text-anchor="middle">THE QUIET HOURS</text></svg>')}`;
export const playbackFixture: MediaDetail = {
  item: {
    id: "sample-video",
    kind: "youtube",
    type: "video",
    title: "A slower way to discover what matters",
    description:
      "A conversation about thoughtful tools, good habits, and making room for things worth your attention.",
    channel: {
      id: "sample-channel",
      title: "The Quiet Hours",
      description: "",
      artworkUrl: cover,
      isPodcast: false,
      source: "youtube",
      subscribed: true,
      videoFilter: "all",
    },
    artworkUrl: cover,
    publishedAt: "2026-10-20T12:00:00Z",
    durationSeconds: 1800,
    progressSeconds: 120,
    hidden: false,
    inWatchLater: false,
    completed: false,
  },
  authenticated: true,
  reportProgress: false,
  volume: 70,
  skipSegments: [],
  podcastSegments: {
    enabled: true,
    status: "ready",
    phase: "",
    error: "",
    source: "publisher",
    transcript_ready: true,
    has_segments: true,
    duration_ms: 1_800_000,
    segments: [
      {
        category: "sponsor",
        start_ms: 10_000,
        end_ms: 20_000,
        start_text: "A word from our sponsor",
        end_text: "Back to the conversation",
        brand: "",
        reason: "",
        skippable: true,
      },
    ],
  },
  playlists: [],
  membershipIds: [],
};
export const podcastFixture: MediaDetail = {
  ...playbackFixture,
  item: {
    ...playbackFixture.item,
    id: "sample-episode",
    kind: "podcast",
    type: "podcast_episode",
    title: "Making room for a more thoughtful day",
    channel: { ...playbackFixture.item.channel, isPodcast: true, source: "podcasts" },
    streamUrl: "/api/videos/sample-episode/audio",
  },
};
