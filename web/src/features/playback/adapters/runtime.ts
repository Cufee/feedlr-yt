import { request } from "@/lib/api/client";
import { keys, patchMedia, queryClient } from "@/lib/api/queries";
import type { MediaDetail, PodcastSegment } from "@/lib/api/types";
import playerStyleURL from "../../../../../assets/css/player.css?url";
import videoRuntimeURL from "../../../../../assets/js/feedlr-player.js?url";
import podcastRuntimeURL from "../../../../../assets/js/feedlr-podcast-player.js?url";
import shakaStyleURL from "../../../../../assets/vendor/controls.css?url";
import shakaRuntimeURL from "../../../../../assets/vendor/shaka-player.ui.js?url";

export interface VideoSnapshot {
  position: number;
  duration: number;
  playing: boolean;
  volume: number;
  muted: boolean;
  rate: number;
  mode: "native" | "iframe";
  ready: boolean;
  loading: boolean;
  switching: boolean;
  blocked: boolean;
  audioOnly: boolean;
  quality: string;
  qualities: readonly number[];
  tvOnline: boolean;
  tvSending: boolean;
  tvScreenName: string;
  notice: string;
  noticeID: number;
  error: string;
}
export interface PodcastSnapshot {
  position: number;
  duration: number;
  playing: boolean;
  volume: number;
  rate: number;
  loading: boolean;
  blocked: boolean;
  error: string;
  scanHold: boolean;
  segmentsEnabled: boolean;
  analysisStatus: string;
  analysisPhase: string;
  analysisError: string;
  validated: boolean;
  segments: readonly Readonly<PodcastSegment>[];
}
export interface VideoActions {
  play: () => void;
  pause: () => void;
  retry: () => void;
  sendToTV: () => void;
}
export interface PodcastActions {
  toggle: () => void;
  seekBy: (seconds: number) => void;
  scrub: (seconds: number) => void;
  finishScrub: () => void;
  skipScanWait: () => void;
  setRate: (rate: number) => void;
  setVolume: (volume: number) => void;
  retry: () => void;
}
interface RuntimeStore<T> {
  subscribe: (listener: () => void) => () => void;
  getSnapshot: () => T;
  cleanup: () => void | Promise<void>;
}
interface VideoRuntime extends RuntimeStore<VideoSnapshot> {
  playVideo: () => void | Promise<void>;
  pauseVideo: () => void;
  retry: () => Promise<void>;
  sendToTV: () => Promise<void>;
}
interface PodcastRuntime extends RuntimeStore<PodcastSnapshot>, PodcastActions {
  audio: HTMLAudioElement;
  play: () => void;
  pause: () => void;
}
interface VideoOptions {
  root: HTMLElement;
  externalUI: true;
  assetURLs: { playerStyle: string; shakaStyle: string; shakaScript: string };
  channel: string;
  video: string;
  progress: number;
  volume: number;
  withProgress: boolean;
  authenticated: boolean;
  segments: MediaDetail["skipSegments"];
  returnURL: string;
  title: string;
  channelTitle: string;
  persistProgress: (position: number, volume: number) => Promise<void>;
}
interface PodcastOptions {
  root: HTMLElement;
  audio: HTMLAudioElement;
  externalUI: true;
  episode: string;
  title: string;
  channel: string;
  progress: number;
  withProgress: boolean;
  initialSegments: MediaDetail["podcastSegments"];
  persistProgress: (position: number, volume: number) => Promise<void>;
}
declare global {
  interface Window {
    FeedlrPlayer?: { mount: (options: VideoOptions) => VideoRuntime };
    FeedlrPodcastPlayer?: { mount: (options: PodcastOptions) => PodcastRuntime };
  }
}

const scripts = new Map<string, Promise<void>>();
function loadRuntime(src: string): Promise<void> {
  const pending = scripts.get(src);
  if (pending) {
    return pending;
  }
  const script = document.createElement("script");
  script.src = src;
  script.async = true;
  const promise = new Promise<void>((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      script.remove();
      reject(new Error("The player took too long to load. Try again."));
    }, 15_000);
    script.onload = () => {
      clearTimeout(timeout);
      resolve();
    };
    script.onerror = () => {
      clearTimeout(timeout);
      script.remove();
      reject(new Error("The player could not load. Try again."));
    };
    document.head.append(script);
  }).catch((error: unknown) => {
    scripts.delete(src);
    throw error;
  });
  scripts.set(src, promise);
  return promise;
}
let playerStyle: Promise<void> | undefined;
function loadPlayerStyle(): Promise<void> {
  if (playerStyle) {
    return playerStyle;
  }
  const link = document.createElement("link");
  link.rel = "stylesheet";
  link.href = playerStyleURL;
  link.dataset.feedlrPlayer = "true";
  playerStyle = new Promise<void>((resolve, reject) => {
    const fail = (message: string) => {
      link.onload = null;
      link.onerror = null;
      link.remove();
      reject(new Error(message));
    };
    const timeout = window.setTimeout(
      () => fail("The player styles took too long to load. Try again."),
      15_000,
    );
    link.onload = () => {
      clearTimeout(timeout);
      link.dataset.feedlrLoaded = "true";
      resolve();
    };
    link.onerror = () => {
      clearTimeout(timeout);
      fail("The player styles could not load. Try again.");
    };
    document.head.append(link);
  }).catch((error: unknown) => {
    playerStyle = undefined;
    throw error;
  });
  return playerStyle;
}

// A replacement never mounts while Shaka is still destroying its predecessor.
let lifecycle: Promise<void> = Promise.resolve();
function serialize(operation: () => Promise<void> | void) {
  const next = lifecycle.then(operation);
  lifecycle = next.catch(() => undefined);
  return next;
}
const progressQueues = new Map<string, { tail: Promise<void>; version: number }>();

abstract class PlaybackSession<
  T extends { loading: boolean; error: string },
  R extends RuntimeStore<T>,
> {
  protected runtime?: R;
  protected readonly detail: MediaDetail;
  private snapshot: T;
  private readonly listeners = new Set<() => void>();
  private generation = 0;
  private unsubscribe?: () => void;
  private readonly progressWrites = new Set<Promise<void>>();
  private retryLoad?: () => void;
  constructor(detail: MediaDetail, initial: T) {
    this.detail = detail;
    this.snapshot = Object.freeze(initial);
  }
  readonly subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };
  readonly getSnapshot = () => this.snapshot;
  protected update(next: T) {
    if (Object.is(next, this.snapshot)) {
      return;
    }
    this.snapshot = next;
    for (const listener of this.listeners) {
      listener();
    }
  }
  protected readonly persistProgress = (position: number, volume: number) => {
    const mediaId = this.detail.item.id;
    const queue = progressQueues.get(mediaId) ?? { tail: Promise.resolve(), version: 0 };
    progressQueues.set(mediaId, queue);
    const version = ++queue.version;
    const pending = queue.tail
      .catch(() => undefined)
      .then(() =>
        request(`/media/${encodeURIComponent(mediaId)}/state`, {
          method: "PATCH",
          body: JSON.stringify({ progressSeconds: position, volume }),
          keepalive: true,
        }),
      )
      .then(() => {
        if (version === queue.version) {
          queryClient.setQueriesData({ queryKey: keys.all }, (value: unknown) =>
            patchMedia(value, mediaId, { progressSeconds: position }),
          );
        }
      });
    queue.tail = pending;
    this.progressWrites.add(pending);
    void pending
      .finally(() => {
        this.progressWrites.delete(pending);
        if (queue.tail === pending) {
          progressQueues.delete(mediaId);
        }
      })
      .catch(() => undefined);
    return pending;
  };
  protected abstract prepareRuntime(): Promise<void>;
  protected abstract mountRuntime(
    root: HTMLElement,
    current: () => boolean,
  ): Promise<R | undefined>;
  protected abstract loadFailed(message: string): void;
  protected retryMount() {
    this.retryLoad?.();
  }
  readonly connect = (root: HTMLElement) => {
    const generation = ++this.generation;
    const current = () => generation === this.generation && root.isConnected;
    const start = () => {
      this.update(Object.freeze({ ...this.getSnapshot(), loading: true, error: "" }));
      void this.prepareRuntime()
        .then(() =>
          serialize(async () => {
            if (!current()) {
              return;
            }
            const runtime = await this.mountRuntime(root, current);
            if (!runtime) {
              return;
            }
            if (!current()) {
              await runtime.cleanup();
              return;
            }
            this.runtime = runtime;
            this.unsubscribe = runtime.subscribe(() => this.update(runtime.getSnapshot()));
            this.update(runtime.getSnapshot());
          }),
        )
        .catch((error: unknown) => {
          if (generation === this.generation) {
            this.loadFailed(
              error instanceof Error ? error.message : "The player could not load. Try again.",
            );
          }
        });
    };
    this.retryLoad = start;
    start();
    return () => {
      if (generation !== this.generation) {
        return;
      }
      this.generation++;
      this.retryLoad = undefined;
      this.unsubscribe?.();
      this.unsubscribe = undefined;
      const runtime = this.runtime;
      this.runtime = undefined;
      // Abort immediately; serialize the asynchronous adapter teardown.
      const stopped = Promise.resolve(runtime?.cleanup());
      void serialize(() => stopped).catch(() => undefined);
      // Network persistence must not delay playback on the next page.
      void Promise.allSettled([stopped, ...this.progressWrites]).then(() => {
        void queryClient.invalidateQueries({ queryKey: keys.all });
      });
    };
  };
}

export function initialVideoSnapshot(detail: MediaDetail): VideoSnapshot {
  return {
    position: detail.item.progressSeconds,
    duration: detail.item.durationSeconds ?? 0,
    playing: false,
    volume: detail.volume,
    muted: false,
    rate: 1,
    mode: detail.authenticated ? "native" : "iframe",
    ready: false,
    loading: true,
    switching: false,
    blocked: false,
    audioOnly: false,
    quality: "auto",
    qualities: Object.freeze([]),
    tvOnline: false,
    tvSending: false,
    tvScreenName: "",
    notice: "",
    noticeID: 0,
    error: "",
  };
}
export class VideoSession
  extends PlaybackSession<VideoSnapshot, VideoRuntime>
  implements VideoActions
{
  constructor(detail: MediaDetail) {
    super(detail, initialVideoSnapshot(detail));
  }
  protected async prepareRuntime() {
    await loadPlayerStyle();
    if (!window.FeedlrPlayer) {
      await loadRuntime(videoRuntimeURL);
    }
  }
  protected async mountRuntime(root: HTMLElement, current: () => boolean) {
    if (!current()) {
      return;
    }
    if (!window.FeedlrPlayer) {
      throw new Error("The video player is unavailable. Try again.");
    }
    return window.FeedlrPlayer.mount({
      root,
      externalUI: true,
      assetURLs: {
        playerStyle: playerStyleURL,
        shakaStyle: shakaStyleURL,
        shakaScript: shakaRuntimeURL,
      },
      channel: this.detail.item.channel.id,
      video: this.detail.item.id,
      progress: this.detail.item.progressSeconds,
      volume: this.detail.volume,
      withProgress: this.detail.reportProgress,
      authenticated: this.detail.authenticated,
      segments: this.detail.skipSegments ?? [],
      returnURL: "/app",
      title: this.detail.item.title,
      channelTitle: this.detail.item.channel.title,
      persistProgress: this.persistProgress,
    });
  }
  protected loadFailed(message: string) {
    this.update(Object.freeze({ ...this.getSnapshot(), loading: false, error: message }));
  }
  readonly play = () => {
    void this.runtime?.playVideo();
  };
  readonly pause = () => {
    this.runtime?.pauseVideo();
  };
  readonly retry = () => {
    if (this.runtime) {
      void this.runtime.retry();
    } else {
      this.retryMount();
    }
  };
  readonly sendToTV = () => {
    void this.runtime?.sendToTV();
  };
}
export function initialPodcastSnapshot(detail: MediaDetail): PodcastSnapshot {
  return {
    position: detail.item.progressSeconds,
    duration: detail.item.durationSeconds ?? 0,
    playing: false,
    volume: detail.volume,
    rate: 1,
    loading: true,
    blocked: false,
    error: "",
    scanHold: false,
    segmentsEnabled: detail.podcastSegments?.enabled ?? false,
    analysisStatus: detail.podcastSegments?.status ?? "disabled",
    analysisPhase: detail.podcastSegments?.phase ?? "",
    analysisError: detail.podcastSegments?.error ?? "",
    validated: false,
    segments: Object.freeze(
      (detail.podcastSegments?.segments ?? []).map((segment) => Object.freeze({ ...segment })),
    ),
  };
}
export class PodcastSession
  extends PlaybackSession<PodcastSnapshot, PodcastRuntime>
  implements PodcastActions
{
  constructor(detail: MediaDetail) {
    super(detail, initialPodcastSnapshot(detail));
  }
  protected async prepareRuntime() {
    if (!window.FeedlrPodcastPlayer) {
      await loadRuntime(podcastRuntimeURL);
    }
  }
  protected async mountRuntime(root: HTMLElement, current: () => boolean) {
    if (!current()) {
      return;
    }
    if (!window.FeedlrPodcastPlayer) {
      throw new Error("The podcast player is unavailable. Try again.");
    }
    if (!this.detail.item.streamUrl) {
      throw new Error("This episode has no playable audio.");
    }
    const audio = document.createElement("audio");
    audio.dataset.feedlrPodcastPlayer = "true";
    audio.preload = "metadata";
    root.replaceChildren(audio);
    const runtime = window.FeedlrPodcastPlayer.mount({
      root,
      audio,
      externalUI: true,
      episode: this.detail.item.id,
      title: this.detail.item.title,
      channel: this.detail.item.channel.title,
      progress: this.detail.item.progressSeconds,
      withProgress: this.detail.reportProgress,
      initialSegments: this.detail.podcastSegments,
      persistProgress: this.persistProgress,
    });
    audio.src = this.detail.item.streamUrl;
    runtime.play();
    return runtime;
  }
  protected loadFailed(message: string) {
    this.update(Object.freeze({ ...this.getSnapshot(), loading: false, error: message }));
  }
  readonly toggle = () => {
    this.runtime?.toggle();
  };
  readonly seekBy = (seconds: number) => {
    this.runtime?.seekBy(seconds);
  };
  readonly scrub = (seconds: number) => {
    this.runtime?.scrub(seconds);
  };
  readonly finishScrub = () => {
    this.runtime?.finishScrub();
  };
  readonly skipScanWait = () => {
    this.runtime?.skipScanWait();
  };
  readonly setRate = (rate: number) => {
    this.runtime?.setRate(rate);
  };
  readonly setVolume = (volume: number) => {
    this.runtime?.setVolume(volume);
  };
  readonly retry = () => {
    if (this.runtime) {
      this.runtime.retry();
    } else {
      this.retryMount();
    }
  };
}
