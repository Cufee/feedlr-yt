import type { Meta, StoryObj } from "@storybook/react-vite";
import { createMemoryHistory, RouterContextProvider } from "@tanstack/react-router";
import { useState } from "react";
import { createAppRouter } from "@/app/router";
import { Progress, Toast } from "@/components/ui";
import type { MediaItem } from "@/lib/api/types";
import { MediaCardView } from "./MediaCard";
import "@/app/app.css";

function cover(background: string, foreground: string) {
  return `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360" viewBox="0 0 640 360"><rect width="640" height="360" fill="${background}"/><circle cx="490" cy="85" r="45" fill="${foreground}"/><path d="M0 300 150 120 300 290 450 185 640 330V360H0Z" fill="${foreground}"/></svg>`)}`;
}
const base: MediaItem = {
  id: "progress-preview",
  kind: "youtube",
  type: "video",
  title: "A walk through the hills",
  description: "",
  artworkUrl: cover("#f7f1df", "#b8a48b"),
  channel: {
    id: "field-notes",
    title: "Field Notes",
    description: "",
    artworkUrl: "",
    subscribed: true,
    isPodcast: false,
    source: "youtube",
    videoFilter: "all",
  },
  publishedAt: "2026-10-20T12:00:00Z",
  durationSeconds: 1200,
  progressSeconds: 420,
  hidden: false,
  completed: false,
  inWatchLater: false,
};
const items: MediaItem[] = [
  base,
  {
    ...base,
    id: "completed-without-position",
    title: "The forest after the rain",
    artworkUrl: cover("#18222e", "#456159"),
    completed: true,
    progressSeconds: 0,
  },
  {
    ...base,
    id: "unplayed",
    title: "Inside the workshop",
    artworkUrl: cover("#49415c", "#a18caf"),
    progressSeconds: 0,
  },
  { ...base, id: "just-started", title: "The coast road", progressSeconds: 1 },
  { ...base, id: "hidden", title: "A hidden episode", hidden: true },
];

function MediaProgressExamples() {
  const [router] = useState(() => createAppRouter(createMemoryHistory()));
  return (
    <RouterContextProvider router={router}>
      <main className="page-stack p-5">
        <div className="media-grid">
          {items.map((item) => (
            <MediaCardView key={item.id} item={item} />
          ))}
        </div>
        <div className="panel space-y-5 p-5">
          <Progress label="Playlist completion" value={35} />
          <Progress label="Completed playlist" value={100} />
          <Progress label="Loading playlist" />
        </div>
      </main>
    </RouterContextProvider>
  );
}
function GlassToastExamples() {
  const [visible, setVisible] = useState(true);
  return (
    <div
      className="grid min-h-dvh place-items-center p-5"
      style={{
        backgroundImage: `url("${base.artworkUrl}")`,
        backgroundSize: "cover",
        backgroundPosition: "center",
      }}
    >
      {visible && (
        <div className="flex w-full max-w-110 flex-col gap-4">
          <Toast tone="info" onDismiss={() => setVisible(false)}>
            Link copied
          </Toast>
          <Toast tone="danger" onDismiss={() => setVisible(false)}>
            The playlist could not be updated. Try again.
          </Toast>
        </div>
      )}
    </div>
  );
}
const meta = {
  title: "Media/Progress",
  component: MediaProgressExamples,
  parameters: { layout: "fullscreen", controls: { disable: true } },
} satisfies Meta<typeof MediaProgressExamples>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Desktop: Story = { globals: { viewport: { value: "desktop" } } };
export const Mobile: Story = { globals: { viewport: { value: "phone" } } };
export const GlassToastsDesktop: Story = {
  globals: { viewport: { value: "desktop" } },
  render: () => <GlassToastExamples />,
};
export const GlassToastsMobile: Story = {
  globals: { viewport: { value: "phone" } },
  render: () => <GlassToastExamples />,
};
