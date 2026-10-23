import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { request } from "@/lib/api/client";
import { queryClient } from "@/lib/api/queries";
import { playbackFixture } from "../fixtures";
import type { VideoSnapshot } from "./runtime";
import { initialVideoSnapshot, VideoSession } from "./runtime";

vi.mock("@/lib/api/client", () => ({ request: vi.fn() }));
// Vitest's Node environment replaces CSS imports, including ?url, with an empty module.
vi.mock("../../../../../assets/css/player.css?url", () => ({
  default: "/ui-assets/assets/player-fixture.css",
}));
vi.mock("../../../../../assets/vendor/controls.css?url", () => ({
  default: "/ui-assets/assets/controls-fixture.css",
}));
vi.mock("@/lib/api/queries", () => ({
  keys: { all: ["feedlr"] },
  patchMedia: vi.fn((value) => value),
  queryClient: { setQueriesData: vi.fn(), invalidateQueries: vi.fn().mockResolvedValue(undefined) },
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
function runtime(cleanup: () => Promise<void> = async () => undefined) {
  const snapshot: VideoSnapshot = Object.freeze({
    ...initialVideoSnapshot(playbackFixture),
    ready: true,
    loading: false,
  });
  return {
    subscribe: vi.fn(() => () => undefined),
    getSnapshot: () => snapshot,
    cleanup: vi.fn(cleanup),
    playVideo: vi.fn(),
    pauseVideo: vi.fn(),
    retry: vi.fn().mockResolvedValue(undefined),
    sendToTV: vi.fn().mockResolvedValue(undefined),
  };
}
const root = () => ({ isConnected: true }) as HTMLElement;
beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("window", { setTimeout });
  vi.stubGlobal("document", {
    createElement: () => ({ dataset: {}, remove: vi.fn() }),
    head: {
      append: vi.fn((link: HTMLLinkElement) => {
        queueMicrotask(() => link.onload?.call(link, new Event("load")));
      }),
    },
  });
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("browser player lifecycle", () => {
  it("waits for asynchronous teardown before mounting a StrictMode replacement", async () => {
    const teardown = deferred<void>();
    const first = runtime(() => teardown.promise),
      second = runtime();
    const mount = vi.fn().mockReturnValueOnce(first).mockReturnValue(second);
    window.FeedlrPlayer = { mount };
    const session = new VideoSession(playbackFixture);
    const stop = session.connect(root());
    await vi.waitFor(() => expect(mount).toHaveBeenCalledTimes(1));
    const ready = session.getSnapshot();
    expect(session.getSnapshot()).toBe(ready);
    stop();
    expect(first.cleanup).toHaveBeenCalledOnce();
    const stopAgain = session.connect(root());
    await Promise.resolve();
    expect(mount).toHaveBeenCalledTimes(1);
    teardown.resolve();
    await vi.waitFor(() => expect(mount).toHaveBeenCalledTimes(2));
    expect(session.getSnapshot()).toBe(second.getSnapshot());
    stopAgain();
    await vi.waitFor(() => expect(second.cleanup).toHaveBeenCalledOnce());
  });

  it("cancels an obsolete mount while its external script is loading", async () => {
    const script: Partial<HTMLScriptElement> = { remove: vi.fn() };
    const style: Partial<HTMLLinkElement> = { dataset: {}, remove: vi.fn() };
    vi.stubGlobal("window", { setTimeout: vi.fn(() => 1) });
    vi.stubGlobal("document", {
      createElement: (tag: string) => (tag === "link" ? style : script),
      head: {
        append: vi.fn((element: HTMLLinkElement | HTMLScriptElement) => {
          if (element === style) {
            queueMicrotask(() => style.onload?.call(style as HTMLLinkElement, new Event("load")));
          }
        }),
      },
    });
    const session = new VideoSession(playbackFixture);
    const stop = session.connect(root());
    await vi.waitFor(() => expect(document.head.append).toHaveBeenLastCalledWith(script));
    stop();
    const mount = vi.fn(() => runtime());
    window.FeedlrPlayer = { mount };
    script.onload?.call(script as HTMLScriptElement, new Event("load"));
    await vi.waitFor(() => expect(queryClient.invalidateQueries).toHaveBeenCalled());
    expect(mount).not.toHaveBeenCalled();
  });

  it("serializes progress writes and only updates caches for the newest intent", async () => {
    const first = deferred<void>(),
      second = deferred<void>();
    vi.mocked(request).mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    let persist!: (position: number, volume: number) => Promise<void>;
    window.FeedlrPlayer = {
      mount: (options) => {
        persist = options.persistProgress;
        return runtime();
      },
    };
    const session = new VideoSession(playbackFixture);
    const stop = session.connect(root());
    await vi.waitFor(() => expect(persist).toBeTypeOf("function"));
    const earlier = persist(130, 50),
      latest = persist(70, 40);
    await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(1));
    expect(request).toHaveBeenLastCalledWith(
      "/media/sample-video/state",
      expect.objectContaining({
        body: JSON.stringify({ progressSeconds: 130, volume: 50 }),
        keepalive: true,
      }),
    );
    first.resolve();
    await earlier;
    await vi.waitFor(() => expect(request).toHaveBeenCalledTimes(2));
    expect(queryClient.setQueriesData).not.toHaveBeenCalled();
    second.resolve();
    await latest;
    expect(queryClient.setQueriesData).toHaveBeenCalledOnce();
    stop();
    await vi.waitFor(() => expect(queryClient.invalidateQueries).toHaveBeenCalled());
  });

  it("starts the next player while the previous page is still saving progress", async () => {
    const response = deferred<void>();
    vi.mocked(request).mockReturnValueOnce(response.promise);
    let persist!: (position: number, volume: number) => Promise<void>;
    const mount = vi.fn((options: Parameters<NonNullable<Window["FeedlrPlayer"]>["mount"]>[0]) => {
      persist = options.persistProgress;
      return runtime();
    });
    window.FeedlrPlayer = { mount };
    const first = new VideoSession(playbackFixture);
    const stopFirst = first.connect(root());
    await vi.waitFor(() => expect(mount).toHaveBeenCalledOnce());
    const saved = persist(150, 70);
    stopFirst();
    const next = new VideoSession({
      ...playbackFixture,
      item: { ...playbackFixture.item, id: "next-video" },
    });
    const stopNext = next.connect(root());
    await vi.waitFor(() => expect(mount).toHaveBeenCalledTimes(2));
    response.resolve();
    await saved;
    stopNext();
    await vi.waitFor(() => expect(queryClient.invalidateQueries).toHaveBeenCalled());
  });
});

describe("player stylesheet recovery", () => {
  for (const failure of ["error", "timeout"] as const) {
    it(`retries the same pinned stylesheet after a ${failure}`, async () => {
      vi.resetModules();
      const { VideoSession: Session } = await import("./runtime");
      const links: HTMLLinkElement[] = [];
      vi.stubGlobal("document", {
        createElement: () => ({ dataset: {}, remove: vi.fn() }),
        head: { append: vi.fn((link: HTMLLinkElement) => links.push(link)) },
      });
      vi.useFakeTimers();
      const mount = vi.fn<NonNullable<Window["FeedlrPlayer"]>["mount"]>(() => runtime());
      vi.stubGlobal("window", { setTimeout, FeedlrPlayer: { mount } });
      const session = new Session(playbackFixture);
      const stop = session.connect(root());
      const first = links[0];
      expect(first).toBeDefined();
      if (failure === "timeout") {
        await vi.advanceTimersByTimeAsync(15_000);
      } else {
        first?.onerror?.call(first, new Event("error"));
      }
      await vi.waitFor(() => expect(session.getSnapshot().error).toMatch(/player styles/));
      expect(first?.remove).toHaveBeenCalledOnce();
      expect(mount).not.toHaveBeenCalled();
      session.retry();
      const next = links[1];
      expect(next).toBeDefined();
      expect(next?.href).toBe(first?.href);
      next?.onload?.call(next, new Event("load"));
      await vi.waitFor(() => expect(mount).toHaveBeenCalledOnce());
      expect(session.getSnapshot().ready).toBe(true);
      const options = mount.mock.calls[0]?.[0] as
        | Parameters<NonNullable<Window["FeedlrPlayer"]>["mount"]>[0]
        | undefined;
      expect(options?.assetURLs.playerStyle).toBe(next?.href);
      expect(options?.assetURLs.shakaScript).toContain("shaka-player.ui");
      expect(options?.assetURLs.shakaStyle).toContain("controls");
      stop();
      await vi.waitFor(() => expect(queryClient.invalidateQueries).toHaveBeenCalled());
    });
  }
});
