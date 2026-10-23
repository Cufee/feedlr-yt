import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, RouterProvider, useRouterState } from "@tanstack/react-router";
import { HttpResponse, http } from "msw";
import { StrictMode, useState } from "react";
import { expect, waitFor, within } from "storybook/test";
import { NotificationProvider } from "@/app/Notifications";
import { createPreviewAPI } from "@/app/preview-fixtures";
import { createAppRouter } from "@/app/router";
import { Button } from "@/components/ui";
import type { MediaDetail } from "@/lib/api/types";
import { playbackFixture, podcastFixture } from "./fixtures";
import audioURL from "./preview/audio.m4a";
import manifest from "./preview/manifest.mpd?raw";
import "@/app/app.css";

const dashAssets = import.meta.glob<string>("./preview/*.m4s", {
  eager: true,
  query: "?url",
  import: "default",
});

const videoDetail: MediaDetail = {
  ...playbackFixture,
  item: {
    ...playbackFixture.item,
    title: "A small cabin in the mountains",
    durationSeconds: 60,
    progressSeconds: 12,
  },
};
const podcastDetail: MediaDetail = {
  ...podcastFixture,
  item: {
    ...podcastFixture.item,
    title: "How we build better habits",
    descriptionHtml:
      "<p>A conversation with Dr. Maya Chen about the routines that stick, and how to make a change that lasts.</p><p>We discuss getting started, setting realistic goals, and making time for what matters.</p>",
    durationSeconds: 60,
    progressSeconds: 12,
    streamUrl: "/api/preview/audio.m4a",
  },
};

// Keep the shipped engines and their DOM intact. Only API/provider responses
// and media bytes are local fixtures; no YouTube, podcast or account is used.
async function localMedia(request: Request, sourceURL: string, contentType = "video/mp4") {
  const source = await fetch(sourceURL);
  const bytes = await source.arrayBuffer();
  const range = /^bytes=(\d+)-(\d*)$/.exec(request.headers.get("Range") ?? "");
  const start = range ? Number(range[1]) : 0;
  const end = range?.[2] ? Math.min(Number(range[2]), bytes.byteLength - 1) : bytes.byteLength - 1;
  return new HttpResponse(request.method === "HEAD" ? null : bytes.slice(start, end + 1), {
    status: range ? 206 : 200,
    headers: {
      "Content-Type": contentType,
      "Content-Length": String(end - start + 1),
      "Accept-Ranges": "bytes",
      ...(range ? { "Content-Range": `bytes ${start}-${end}/${bytes.byteLength}` } : {}),
    },
  });
}

function PlaybackPagePreview({
  kind,
  returnTo = "/app",
  historyTools = false,
}: {
  kind: "youtube" | "podcast";
  returnTo?: string;
  historyTools?: boolean;
}) {
  const detail = kind === "youtube" ? videoDetail : podcastDetail;
  const [preview] = useState(() => ({
    client: new QueryClient({
      defaultOptions: {
        queries: { retry: false, staleTime: 20_000, gcTime: 0 },
        mutations: { retry: false },
      },
    }),
    router: createAppRouter(
      createMemoryHistory({
        initialEntries: [`/video/${detail.item.id}?return=${encodeURIComponent(returnTo)}`],
      }),
    ),
  }));
  const location = useRouterState({
    router: preview.router,
    select: (state) => state.location.href,
  });
  return (
    <StrictMode>
      <QueryClientProvider client={preview.client}>
        <NotificationProvider>
          {historyTools && (
            <nav
              aria-label="Preview history"
              className="relative z-50 flex flex-wrap items-center gap-3 p-4"
            >
              <Button onClick={() => preview.router.history.back()}>Back in preview</Button>
              <Button onClick={() => preview.router.history.forward()}>Forward in preview</Button>
              <output aria-label="Preview route" className="w-full break-all">
                {location}
              </output>
            </nav>
          )}
          <RouterProvider router={preview.router} />
        </NotificationProvider>
      </QueryClientProvider>
    </StrictMode>
  );
}

const meta = {
  title: "Playback/Screens",
  component: PlaybackPagePreview,
  parameters: { layout: "fullscreen", controls: { disable: true } },
  args: { kind: "youtube" },
} satisfies Meta<typeof PlaybackPagePreview>;
export default meta;
type Story = StoryObj<typeof meta>;

function screen(kind: "youtube" | "podcast", viewport: "desktop" | "phone"): Story {
  const preview = createPreviewAPI();
  const initial = kind === "youtube" ? videoDetail : podcastDetail;
  let detail = structuredClone(initial);
  return {
    args: { kind },
    globals: { viewport: { value: viewport } },
    parameters: {
      msw: {
        handlers: [
          http.get("/api/v1/media/:id", () => HttpResponse.json(detail)),
          http.get(
            "/api/preview/manifest.mpd",
            () =>
              new HttpResponse(manifest, { headers: { "Content-Type": "application/dash+xml" } }),
          ),
          http.all("/api/preview/audio.m4a", ({ request }) =>
            localMedia(request, audioURL, "audio/mp4"),
          ),
          http.all("/api/preview/:file", ({ request, params }) => {
            const source = dashAssets[`./preview/${params.file}`];
            return source ? localMedia(request, source) : new HttpResponse(null, { status: 404 });
          }),
          http.all("https://www.youtube.com/*", () => HttpResponse.error()),
          http.post("/api/videos/:id/playback", () =>
            HttpResponse.json({
              mode: "native",
              manifestUrl: "/api/preview/manifest.mpd",
              expiresAt: new Date(Date.now() + 3_600_000).toISOString(),
              qualities: [360],
              progress: 12,
            }),
          ),
          http.get("/api/videos/:id/chapters", () => HttpResponse.json({ chapters: [] })),
          http.post(
            "/api/videos/:id/playback/events",
            () => new HttpResponse(null, { status: 204 }),
          ),
          http.get("/api/v1/tv/status", () => HttpResponse.json({ online: false })),
          http.get(
            "/thumb/video/:id/maxresdefault",
            () =>
              new HttpResponse(decodeURIComponent(initial.item.artworkUrl.split(",")[1] ?? ""), {
                headers: { "Content-Type": "image/svg+xml" },
              }),
          ),
          http.all("/api/v1/media/:id/segments", () => HttpResponse.json(initial.podcastSegments)),
          http.put("/api/v1/watch-later/:id", () => {
            detail = { ...detail, item: { ...detail.item, inWatchLater: true } };
            return new HttpResponse(null, { status: 204 });
          }),
          http.delete("/api/v1/watch-later/:id", () => {
            detail = { ...detail, item: { ...detail.item, inWatchLater: false } };
            return new HttpResponse(null, { status: 204 });
          }),
          ...preview.handlers,
        ],
      },
    },
    beforeEach: () => {
      preview.reset();
      detail = structuredClone(initial);
      localStorage.removeItem("feedlr-player-quality");
      localStorage.removeItem("podcast-playback-rate");
    },
    render: (args) => <PlaybackPagePreview key={`${kind}:${viewport}`} {...args} />,
    play: async ({ canvasElement }) => {
      const canvas = within(canvasElement);
      await expect(
        canvas.findByRole("heading", { name: initial.item.title, level: 1 }),
      ).resolves.toBeInTheDocument();
      if (kind === "podcast") {
        await waitFor(() =>
          expect(canvas.getByRole("slider", { name: "Episode progress" })).toHaveAttribute(
            "max",
            "60",
          ),
        );
      } else {
        await waitFor(() =>
          expect(canvasElement.querySelector("video")?.readyState).toBeGreaterThanOrEqual(2),
        );
        await expect(
          canvas.findByRole("button", { name: "More settings" }),
        ).resolves.toBeInTheDocument();
      }
    },
  };
}

export const VideoDesktop = screen("youtube", "desktop");
export const VideoMobile = screen("youtube", "phone");
export const PodcastDesktop = screen("podcast", "desktop");
export const PodcastMobile = screen("podcast", "phone");

export const PodcastReturnToHistory: Story = {
  ...screen("podcast", "desktop"),
  args: { kind: "podcast", returnTo: "/app/recent?source=podcasts&page=2", historyTools: true },
};
export const UnsafeReturn: Story = {
  ...screen("youtube", "desktop"),
  args: { kind: "youtube", returnTo: "https://untrusted.example/app", historyTools: true },
};
