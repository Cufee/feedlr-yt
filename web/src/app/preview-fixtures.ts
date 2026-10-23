import { delay, HttpResponse, http } from "msw";
import type {
  Channel,
  ChannelDetail,
  Feed,
  MediaDetail,
  MediaItem,
  PageResult,
  Playlist,
  PlaylistDetail,
  Session,
  Settings,
} from "@/lib/api/types";

function illustration(background: string, foreground: string, motif: "hills" | "studio" | "waves") {
  const drawing =
    motif === "hills"
      ? `<circle cx="620" cy="155" r="66" fill="#f1dfbc"/><path d="M0 380 230 160 485 420 680 235 900 420V520H0Z" fill="${foreground}"/><path d="m0 470 300-160 270 210H0Zm400 50 280-185 220 140v45Z" fill="#142d2a" opacity=".65"/><path d="m440 520 95-190 75 190" fill="#d9c4a3" opacity=".5"/>`
      : motif === "studio"
        ? `<rect x="465" y="70" width="250" height="330" rx="125" fill="${foreground}"/><rect x="490" y="95" width="200" height="280" rx="100" fill="${background}"/><path d="M0 405H900V520H0Z" fill="#201d23" opacity=".35"/><path d="M250 260h130l-18 150h-93Z" fill="#eadac5"/><ellipse cx="315" cy="260" rx="65" ry="17" fill="#c2a68e"/><path d="M230 405h180v15H230Z" fill="${foreground}"/><path d="M310 260v-90m0 35-45-35m45 15 45-50" fill="none" stroke="${foreground}" stroke-width="12" stroke-linecap="round"/>`
        : `<circle cx="450" cy="240" r="155" fill="${foreground}" opacity=".75"/><path d="M80 300q140-200 280 0t280 0 280 0M-50 370q140-200 280 0t280 0 280 0" fill="none" stroke="#e4ddd0" stroke-width="24" opacity=".75"/><path d="M355 165v130m48-180v220m48-250v280m48-250v220m48-180v130" stroke="${background}" stroke-width="12" stroke-linecap="round"/>`;
  return `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="900" height="520" viewBox="0 0 900 520"><rect width="900" height="520" fill="${background}"/>${drawing}</svg>`)}`;
}

const covers = [
  illustration("#50645d", "#9cad8c", "hills"),
  illustration("#645264", "#bca0bc", "studio"),
  illustration("#715947", "#c6a675", "waves"),
  illustration("#444e62", "#8195b8", "hills"),
  illustration("#6b6650", "#b8b58e", "studio"),
  illustration("#455e64", "#8fb9ba", "waves"),
];

export const previewChannels: Channel[] = [
  {
    id: "field-notes",
    title: "Field Notes",
    description: "Small adventures, quiet landscapes, and a closer look at the world outside.",
    artworkUrl: covers[0] ?? "",
    isPodcast: false,
    source: "youtube",
    subscribed: true,
    videoFilter: "all",
  },
  {
    id: "form-and-field",
    title: "Form & Field",
    description: "Conversations with people who make things slowly and with care.",
    artworkUrl: covers[1] ?? "",
    isPodcast: false,
    source: "youtube",
    subscribed: true,
    videoFilter: "videos",
  },
  {
    id: "quiet-hours",
    title: "The Quiet Hours",
    description: "An independent podcast about attention, creativity, and the everyday.",
    artworkUrl: covers[2] ?? "",
    isPodcast: true,
    source: "podcasts",
    subscribed: true,
    videoFilter: "all",
    feedUrl: "https://quiet-hours.example/feed.xml",
  },
  {
    id: "off-the-clock",
    title: "Off the Clock",
    description: "A little perspective on a life beyond the to-do list.",
    artworkUrl: covers[5] ?? "",
    isPodcast: true,
    source: "podcasts",
    subscribed: false,
    videoFilter: "all",
    feedUrl: "https://off-the-clock.example/feed.xml",
  },
];

const mediaTitles = [
  "A small cabin, a long weekend, and no plans",
  "Inside a ceramic studio by the sea",
  "Making room for a more thoughtful day",
  "Walking the old coast road in autumn",
  "The useful beauty of everyday objects",
  "A conversation about doing a little less",
  "The forest after the rain",
  "How a good idea becomes something real",
];

export const previewMedia: MediaItem[] = mediaTitles.map((title, index) => {
  const channel = previewChannels[index % 3];
  if (!channel) {
    throw new Error("A preview channel is missing");
  }
  const common = {
    id: `preview-${index + 1}`,
    title,
    description: "A little time with people, places, and ideas worth paying attention to.",
    channel,
    artworkUrl: covers[index % covers.length] ?? "",
    publishedAt: `2026-10-${String(21 - index).padStart(2, "0")}T12:00:00Z`,
    durationSeconds: [1084, 1725, 2480, 934, 1450, 3120, 820, 1650][index] ?? 1200,
    progressSeconds: index === 2 ? 735 : index >= 6 ? 1800 : 0,
    hidden: false,
    inWatchLater: index === 0 || index === 2 || index === 4,
    completed: index >= 6,
  };
  return channel.isPodcast
    ? { ...common, kind: "podcast", type: "podcast_episode", streamUrl: "/preview-audio.mp3" }
    : { ...common, kind: "youtube", type: "video" };
});

const initialSettings: Settings = {
  volume: 70,
  passkeys: [
    { id: "laptop", label: "This laptop", createdAt: "2026-07-12T10:00:00Z" },
    { id: "phone", label: "Personal phone", createdAt: "2026-08-04T10:00:00Z" },
  ],
  youtubeSync: {
    available: true,
    connected: true,
    enabled: true,
    playlistId: "preview-library",
    lastError: "",
    lastSyncedAt: "2026-10-22T15:35:00Z",
  },
  tvSync: {
    available: true,
    connected: true,
    enabled: true,
    connectionState: "online",
    stateReason: "",
    screenName: "Living room",
    lastError: "",
    lastConnectedAt: "2026-10-22T14:00:00Z",
    lastEventAt: "2026-10-22T15:35:00Z",
    lastDisconnectAt: "",
    lastUserActivityAt: "2026-10-22T15:35:00Z",
  },
  sponsorBlock: {
    enabled: true,
    categories: ["sponsor", "selfpromo"],
    availableCategories: [
      { value: "sponsor", name: "Sponsors", description: "Paid promotions and sponsorships." },
      {
        value: "selfpromo",
        name: "Self promotion",
        description: "A creator promoting their work.",
      },
      {
        value: "interaction",
        name: "Interaction reminders",
        description: "Like and subscribe reminders.",
      },
      {
        value: "intro",
        name: "Introductions",
        description: "Opening sequences before the main content.",
      },
    ],
  },
  podcastSegments: {
    enabled: true,
    categories: ["sponsor", "selfpromo"],
    availableCategories: [
      { value: "sponsor", name: "Sponsors", description: "Ad reads and paid promotions." },
      {
        value: "selfpromo",
        name: "Self promotion",
        description: "The show’s own products and services.",
      },
      { value: "intro", name: "Introductions", description: "Recurring show introductions." },
      { value: "outro", name: "Closing remarks", description: "Credits and recurring sign-offs." },
    ],
  },
};

const initialPlaylists: Playlist[] = [
  {
    id: "a-slower-sunday",
    name: "A slower Sunday",
    description: "A few things to settle into when there’s nowhere else to be.",
    slug: "a-slower-sunday",
    videoCount: 3,
    progressPercent: 24,
    thumbnailUrl: covers[0] ?? "",
    system: false,
  },
  {
    id: "people-who-make",
    name: "People who make",
    description: "Good work, thoughtful spaces, and the people behind them.",
    slug: "people-who-make",
    videoCount: 2,
    progressPercent: 50,
    thumbnailUrl: covers[1] ?? "",
    system: false,
  },
  {
    id: "long-way-home",
    name: "The long way home",
    description: "Stories and conversations for a little time on the road.",
    slug: "long-way-home",
    videoCount: 2,
    progressPercent: 0,
    thumbnailUrl: covers[5] ?? "",
    system: false,
    youtubePlaylistId: "preview-import",
  },
];

export type PreviewMode = "ready" | "loading" | "error" | "empty" | "save-error";
export interface PreviewOptions {
  mode?: PreviewMode;
  guest?: boolean;
  longHistory?: boolean;
  failFirstRead?: boolean;
  sourceDelayMs?: number;
  holdSourceResponses?: boolean;
}
export interface PreviewRequest {
  method: string;
  path: string;
  search: string;
  body: Record<string, unknown>;
}

// Each story owns a resettable API instance. All mutations stay in this fixture;
// no request reaches an application database, passkey ceremony, or media provider.
export function createPreviewAPI(options: PreviewOptions = {}) {
  const requests: PreviewRequest[] = [];
  let items: MediaItem[] = [];
  let subscriptions = new Set<string>();
  let channelFilters = new Map<string, Channel["videoFilter"]>();
  let playlists: Playlist[] = [];
  let settings = structuredClone(initialSettings);
  let entries: Record<string, string[]> = {};
  let readFailures = 0;
  let sourceResponse: Promise<void> | undefined;
  let releaseSourceResponse: (() => void) | undefined;
  function reset() {
    releaseSourceResponse?.();
    releaseSourceResponse = undefined;
    sourceResponse = options.holdSourceResponses
      ? new Promise<void>((resolve) => {
          releaseSourceResponse = resolve;
        })
      : undefined;
    channelFilters = new Map();
    items = options.mode === "empty" ? [] : structuredClone(previewMedia);
    subscriptions = new Set(
      options.mode === "empty"
        ? []
        : previewChannels.filter((item) => item.subscribed).map((item) => item.id),
    );
    playlists = options.mode === "empty" ? [] : structuredClone(initialPlaylists);
    settings = structuredClone(initialSettings);
    entries = {
      "a-slower-sunday": ["preview-1", "preview-3", "preview-5"],
      "people-who-make": ["preview-2", "preview-8"],
      "long-way-home": ["preview-3", "preview-6"],
    };
    readFailures = options.failFirstRead ? 1 : 0;
    requests.length = 0;
  }
  reset();

  function currentChannel(channel: Channel): Channel {
    return {
      ...channel,
      subscribed: subscriptions.has(channel.id),
      videoFilter: channelFilters.get(channel.id) ?? channel.videoFilter,
    };
  }
  function currentItem(item: MediaItem): MediaItem {
    return { ...item, channel: currentChannel(item.channel) };
  }
  function paginate(values: MediaItem[], url: URL): PageResult {
    const limit = Number(url.searchParams.get("limit") ?? 24);
    const offset = Number(url.searchParams.get("offset") ?? 0);
    return {
      items: values.slice(offset, offset + limit).map(currentItem),
      page: { limit, offset, hasMore: values.length > offset + limit },
    };
  }
  const error = (message: string, status = 422) =>
    HttpResponse.json({ error: { code: "preview_error", message } }, { status });

  const handlers = [
    ...(options.holdSourceResponses
      ? [
          http.post("/__preview/release-source-responses", () => {
            releaseSourceResponse?.();
            return new HttpResponse(null, { status: 204 });
          }),
        ]
      : []),
    http.all("/api/v1/*", async ({ request }) => {
      const url = new URL(request.url);
      const path = url.pathname.slice("/api/v1".length);
      const method = request.method;
      const rawBody = await request.text();
      const body: Record<string, unknown> = rawBody ? JSON.parse(rawBody) : {};
      requests.push({ method, path, search: url.search, body });
      if (path === "/session") {
        return HttpResponse.json<Session>({
          authenticated: !options.guest,
          ...(options.guest ? {} : { userId: "preview-reader", username: "Morgan" }),
          capabilities: { podcastSearch: true, youtubeSync: true, tvSync: true },
        });
      }
      if (method === "GET") {
        if (options.mode === "loading") {
          await delay("infinite");
        }
        if (options.mode === "error") {
          return error("Your library is taking a little longer to respond. Please try again.", 503);
        }
        if (readFailures > 0) {
          await delay(40);
          if (request.signal.aborted) {
            return HttpResponse.error();
          }
          readFailures -= 1;
          return error("Your library is taking a little longer to respond. Please try again.", 503);
        }
        if (path === "/feed") {
          return HttpResponse.json<Feed>({
            new: items.filter((item) => !item.completed && !item.hidden).map(currentItem),
            watched: items.filter((item) => item.completed && !item.hidden).map(currentItem),
            watchLater: items.filter((item) => item.inWatchLater).map(currentItem),
            onboarding: items.length === 0,
          });
        }
        if (path === "/history" || path === "/watch-later") {
          if (path === "/history" && url.searchParams.get("source") !== "all") {
            if (sourceResponse) {
              await sourceResponse;
            } else if (options.sourceDelayMs) {
              await delay(options.sourceDelayMs);
            }
          }
          let values = items.filter((item) => path === "/history" || item.inWatchLater);
          if (options.longHistory && path === "/history") {
            values = Array.from({ length: 30 }, (_, index) => {
              const item = items[index % items.length];
              if (!item) {
                throw new Error("History preview needs media items");
              }
              return {
                ...item,
                id: `history-${index + 1}`,
                title: `${item.title} · ${Math.floor(index / items.length) + 1}`,
              };
            });
          }
          const source = url.searchParams.get("source");
          values = values.filter(
            (item) => !source || source === "all" || item.channel.source === source,
          );
          return HttpResponse.json<PageResult>(paginate(values, url));
        }
        if (path === "/subscriptions" || path === "/search") {
          const source = url.searchParams.get("source");
          const query = (url.searchParams.get("q") ?? "").toLocaleLowerCase();
          const channels = previewChannels
            .map(currentChannel)
            .filter((channel) => !source || source === "all" || channel.source === source)
            .filter((channel) =>
              path === "/subscriptions"
                ? channel.subscribed
                : `${channel.title} ${channel.description}`.toLocaleLowerCase().includes(query),
            );
          return HttpResponse.json<{ items: Channel[] }>({ items: channels });
        }
        if (path.startsWith("/channels/")) {
          const channel = previewChannels.find(
            (item) => item.id === decodeURIComponent(path.slice("/channels/".length)),
          );
          if (!channel) {
            return error("This channel is unavailable.", 404);
          }
          return HttpResponse.json<ChannelDetail>({
            channel: currentChannel(channel),
            items: items
              .filter((item) => item.channel.id === channel.id)
              .filter((item) => {
                const filter = currentChannel(channel).videoFilter;
                return filter === "all" || (filter === "streams") === item.type.includes("stream");
              })
              .map(currentItem),
            subscribed: subscriptions.has(channel.id),
            filter: currentChannel(channel).videoFilter,
          });
        }
        if (path === "/playlists") {
          return HttpResponse.json<{ items: Playlist[] }>({ items: playlists });
        }
        if (path.startsWith("/playlists/")) {
          const id = decodeURIComponent(path.slice("/playlists/".length));
          const playlist = playlists.find((item) => item.id === id);
          if (!playlist) {
            return error("This playlist is unavailable.", 404);
          }
          return HttpResponse.json<PlaylistDetail>({
            playlist,
            items: (entries[id] ?? [])
              .flatMap((id) => items.filter((item) => item.id === id))
              .map(currentItem),
          });
        }
        if (path.startsWith("/media/")) {
          const item = items.find(
            (item) => item.id === decodeURIComponent(path.slice("/media/".length)),
          );
          if (!item) {
            return error("This item is unavailable.", 404);
          }
          return HttpResponse.json<MediaDetail>({
            item: currentItem(item),
            authenticated: !options.guest,
            reportProgress: false,
            volume: settings.volume,
            skipSegments: [],
            podcastSegments: {
              enabled: true,
              status: "idle",
              phase: "",
              error: "",
              source: "",
              transcript_ready: false,
              has_segments: false,
              duration_ms: 0,
              segments: [],
            },
            playlists,
            membershipIds: playlists
              .filter((playlist) => entries[playlist.id]?.includes(item.id))
              .map((playlist) => playlist.id),
          });
        }
        if (path === "/settings") {
          return HttpResponse.json<Settings>(settings);
        }
        if (path.startsWith("/legal/")) {
          return HttpResponse.json({
            html: "<h2>Your time, your choices</h2><p>This preview uses fictional subscriptions and keeps every change inside your browser. Your real account is never contacted.</p>",
          });
        }
      } else {
        await delay(120);
        const parts = path.split("/").filter(Boolean).map(decodeURIComponent);
        if (parts[0] === "watch-later" && parts[1]) {
          if (options.mode === "save-error") {
            return error(
              "Watch Later could not be updated. Your previous choice has been restored.",
              409,
            );
          }
          items = items.map((item) =>
            item.id === parts[1] ? { ...item, inWatchLater: body.present === true } : item,
          );
          return HttpResponse.json({
            mediaId: parts[1],
            present: body.present === true,
            affectedIds: [parts[1]],
          });
        }
        if (parts[0] === "media" && parts[2] === "state") {
          if ("completed" in body && "progressSeconds" in body) {
            return error("Submit completion or progress, rather than both.");
          }
          items = items.map((item) =>
            item.id === parts[1]
              ? {
                  ...item,
                  ...(typeof body.hidden === "boolean" ? { hidden: body.hidden } : {}),
                  ...(typeof body.completed === "boolean"
                    ? {
                        completed: body.completed,
                        progressSeconds: body.completed ? (item.durationSeconds ?? 0) : 0,
                      }
                    : {}),
                }
              : item,
          );
          return new HttpResponse(null, { status: 204 });
        }
        if (parts[0] === "subscriptions") {
          if (
            method === "PATCH" &&
            parts[1] &&
            (body.filter === "all" || body.filter === "videos" || body.filter === "streams")
          ) {
            if (options.mode === "save-error") {
              return error("The channel filter could not be saved.", 503);
            }
            channelFilters.set(parts[1], body.filter);
          } else if (method === "DELETE" && parts[1]) {
            subscriptions.delete(parts[1]);
          } else if (method === "POST") {
            const channel = previewChannels.find(
              (channel) => channel.id === body.channelId || channel.feedUrl === body.feedUrl,
            );
            if (!channel) {
              return error("This preview has no matching feed. Try The Quiet Hours.");
            }
            subscriptions.add(channel.id);
          }
          return new HttpResponse(null, { status: 204 });
        }
        if (path === "/podcasts/open") {
          const channel = previewChannels.find((channel) => channel.feedUrl === body.feedUrl);
          return channel
            ? HttpResponse.json({ channelId: channel.id })
            : error("This podcast is unavailable.", 404);
        }
        if (parts[0] === "playlists") {
          if (parts.length === 1 || parts[1] === "import") {
            const playlist: Playlist = {
              id: "new-collection",
              name: String(body.name ?? "A new perspective"),
              description: String(body.description ?? ""),
              slug: "new-collection",
              videoCount: 0,
              progressPercent: 0,
              thumbnailUrl: covers[3] ?? "",
              system: false,
            };
            playlists = [...playlists, playlist];
            entries[playlist.id] = [];
            return HttpResponse.json<Playlist>(playlist, { status: 201 });
          }
          const id = parts[1] ?? "";
          if (parts.length === 2) {
            if (method === "DELETE") {
              playlists = playlists.filter((playlist) => playlist.id !== id);
              return new HttpResponse(null, { status: 204 });
            }
            playlists = playlists.map((playlist) =>
              playlist.id === id
                ? {
                    ...playlist,
                    name: String(body.name ?? playlist.name),
                    description: String(body.description ?? playlist.description),
                  }
                : playlist,
            );
            return HttpResponse.json<Playlist | undefined>(
              playlists.find((playlist) => playlist.id === id),
            );
          }
          if (parts[2] === "items") {
            const mediaId = parts[3] ?? "preview-2";
            const current = entries[id] ?? [];
            if (parts[4] === "move") {
              const index = current.indexOf(mediaId);
              const neighbor = body.direction === "up" ? index - 1 : index + 1;
              const otherId = current[neighbor];
              if (index >= 0 && otherId) {
                const next = [...current];
                next[index] = otherId;
                next[neighbor] = mediaId;
                entries[id] = next;
              }
            } else {
              entries[id] =
                body.present === false
                  ? current.filter((item) => item !== mediaId)
                  : [...new Set([...current, mediaId])];
              playlists = playlists.map((playlist) =>
                playlist.id === id
                  ? { ...playlist, videoCount: entries[id]?.length ?? 0 }
                  : playlist,
              );
            }
          }
          return new HttpResponse(null, { status: 204 });
        }
        if (path === "/settings") {
          for (const provider of ["sponsorBlock", "podcastSegments"] as const) {
            const patch = body[provider] as
              | { enabled?: boolean; categories?: string[] }
              | undefined;
            if (patch) {
              settings = { ...settings, [provider]: { ...settings[provider], ...patch } };
            }
          }
          return new HttpResponse(null, { status: 204 });
        }
        if (parts[0] === "settings" && parts[1] === "youtube") {
          settings.youtubeSync = {
            ...settings.youtubeSync,
            enabled: body.enabled === true,
            ...(parts[2] === "disconnect" ? { connected: false } : {}),
          };
          return new HttpResponse(null, { status: 204 });
        }
        if (parts[0] === "settings" && parts[1] === "tv") {
          settings.tvSync = {
            ...settings.tvSync,
            enabled: body.enabled === true,
            ...(method === "DELETE" ? { connected: false } : {}),
          };
          return new HttpResponse(null, { status: 204 });
        }
        if (parts[0] === "passkeys" && parts[1] && method === "DELETE") {
          settings.passkeys = settings.passkeys.filter((passkey) => passkey.id !== parts[1]);
          return new HttpResponse(null, { status: 204 });
        }
        if (parts[0] === "auth" || path === "/logout") {
          return error(
            "Account ceremonies are paused in this preview. Your real session is untouched.",
            409,
          );
        }
        if (parts[0] === "channels" && parts[2] === "refresh") {
          return new HttpResponse(null, { status: 204 });
        }
      }
      return error(`The preview has no response for ${method} ${path}.`, 501);
    }),
  ];
  return { handlers, requests, reset };
}
