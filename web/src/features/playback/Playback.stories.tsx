import type { Meta, StoryObj } from "@storybook/react-vite";
import { createMemoryHistory, RouterContextProvider } from "@tanstack/react-router";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { createAppRouter } from "@/app/router";
import { Button } from "@/components/ui";
import type {
  PodcastActions,
  PodcastSnapshot,
  VideoActions,
  VideoSnapshot,
} from "./adapters/runtime";
import { initialPodcastSnapshot, initialVideoSnapshot } from "./adapters/runtime";
import { playbackFixture, podcastFixture } from "./fixtures";
import { PlayerToolbarView } from "./PlayerToolbar";
import { PodcastPlayerView } from "./PodcastPlayer";
import { YouTubePlayerView } from "./YouTubePlayer";

interface ScenarioProps {
  kind: "youtube" | "podcast";
  video?: Partial<VideoSnapshot>;
  podcast?: Partial<PodcastSnapshot>;
}
function PlaybackScenario({
  kind,
  video: videoOverrides,
  podcast: podcastOverrides,
}: ScenarioProps) {
  const [router] = useState(() => createAppRouter(createMemoryHistory()));
  const [video, setVideo] = useState({
    ...initialVideoSnapshot(playbackFixture),
    ...videoOverrides,
  });
  const [podcast, setPodcast] = useState({
    ...initialPodcastSnapshot(podcastFixture),
    loading: false,
    validated: true,
    ...podcastOverrides,
  });
  const [saved, setSaved] = useState(false);
  const [attempts, setAttempts] = useState(0);
  const videoActions: VideoActions = {
    play: () => setVideo((current) => ({ ...current, playing: true, blocked: false })),
    pause: () => setVideo((current) => ({ ...current, playing: false })),
    retry: () => setVideo((current) => ({ ...current, error: "", loading: false, ready: true })),
    sendToTV: () => {
      setVideo((current) => ({ ...current, tvSending: true }));
      void Promise.resolve().then(() => {
        setVideo((current) => ({
          ...current,
          tvSending: false,
          playing: attempts > 0 ? false : current.playing,
          notice: attempts > 0 ? "Sent to TV" : "Could not send video to TV. Try again.",
          noticeID: current.noticeID + 1,
        }));
        setAttempts(attempts + 1);
      });
    },
  };
  const podcastActions: PodcastActions = {
    toggle: () => setPodcast((current) => ({ ...current, playing: !current.playing })),
    seekBy: (seconds) =>
      setPodcast((current) => ({
        ...current,
        position: Math.max(0, Math.min(current.duration, current.position + seconds)),
      })),
    scrub: (position) => setPodcast((current) => ({ ...current, position })),
    finishScrub: () => undefined,
    skipScanWait: () =>
      setPodcast((current) => ({ ...current, scanHold: false, loading: false, playing: true })),
    setRate: (rate) => setPodcast((current) => ({ ...current, rate })),
    setVolume: (volume) => setPodcast((current) => ({ ...current, volume })),
    retry: () =>
      setPodcast((current) => ({ ...current, error: "", blocked: false, loading: false })),
  };
  const detail = kind === "youtube" ? playbackFixture : podcastFixture;
  return (
    <RouterContextProvider router={router}>
      <article
        className={`playback-page ${kind === "youtube" ? "playback-video" : "playback-podcast"}`}
      >
        <PlayerToolbarView
          detail={detail}
          saved={saved}
          onSave={() => setSaved(!saved)}
          onPlaylists={() => undefined}
          onShare={() => undefined}
          tv={video}
          onSendToTV={videoActions.sendToTV}
        />
        {kind === "youtube" ? (
          <YouTubePlayerView title={detail.item.title} snapshot={video} actions={videoActions} />
        ) : (
          <PodcastPlayerView detail={detail} snapshot={podcast} actions={podcastActions} />
        )}
      </article>
    </RouterContextProvider>
  );
}
const meta = {
  title: "Playback/Player",
  component: PlaybackScenario,
  tags: ["autodocs"],
  parameters: { layout: "fullscreen" },
  args: { kind: "youtube" },
} satisfies Meta<typeof PlaybackScenario>;
export default meta;
type Story = StoryObj<typeof meta>;

export const LoadingWithStableActions: Story = {
  args: { video: { loading: true } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const menu = canvas.getByRole("button", { name: "Player actions" });
    const document = within(canvasElement.ownerDocument.body);
    await userEvent.click(menu);
    await userEvent.click(document.getByRole("menuitemcheckbox", { name: "Save to Watch Later" }));
    await waitFor(() => {
      expect(menu).toHaveAttribute("aria-expanded", "false");
      expect(document.queryByRole("menu")).not.toBeInTheDocument();
      expect(menu).toHaveFocus();
    });
    await userEvent.click(menu);
    await expect(
      document.getByRole("menuitemcheckbox", { name: "Remove from Watch Later" }),
    ).toBeChecked();
    await userEvent.keyboard("{Escape}");
    await waitFor(() => {
      expect(menu).toHaveAttribute("aria-expanded", "false");
      expect(document.queryByRole("menu")).not.toBeInTheDocument();
      expect(menu).toHaveFocus();
    });
    await expect(canvas.getByRole("region", { name: "Video player" })).toHaveAttribute(
      "aria-busy",
      "true",
    );
    await expect(canvas.queryByText("Loading video…")).not.toBeInTheDocument();
  },
};
export const AutoplayNeedsUserGesture: Story = {
  args: { video: { loading: false, ready: true, blocked: true } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Play video" }));
    await expect(canvas.queryByRole("button", { name: "Play video" })).not.toBeInTheDocument();
  },
};
export const TVFailureAndRetry: Story = {
  args: {
    video: {
      loading: false,
      ready: true,
      playing: true,
      tvOnline: true,
      tvScreenName: "Living room",
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const menu = canvas.getByRole("button", { name: "Player actions" });
    const document = within(canvasElement.ownerDocument.body);
    await userEvent.click(menu);
    await userEvent.click(document.getByRole("menuitem", { name: "Send to TV (Living room)" }));
    await waitFor(() =>
      expect(canvas.getByText("Could not send video to TV. Try again.")).toBeVisible(),
    );
    await waitFor(() => {
      expect(menu).toHaveAttribute("aria-expanded", "false");
      expect(document.queryByRole("menu")).not.toBeInTheDocument();
      expect(menu).toHaveFocus();
    });
    await userEvent.click(menu);
    await userEvent.click(document.getByRole("menuitem", { name: "Send to TV (Living room)" }));
    await waitFor(() => expect(canvas.getByText("Sent to TV")).toBeVisible());
    await waitFor(() => {
      expect(menu).toHaveAttribute("aria-expanded", "false");
      expect(document.queryByRole("menu")).not.toBeInTheDocument();
      expect(menu).toHaveFocus();
    });
  },
};
export const PodcastScanHold: Story = {
  args: {
    kind: "podcast",
    podcast: {
      scanHold: true,
      playing: true,
      analysisStatus: "running",
      analysisPhase: "preparing",
      validated: false,
      segments: [],
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const pause = canvas.getByRole("button", { name: "Pause episode" });
    await expect(pause).toHaveAttribute("aria-busy", "true");
    await userEvent.click(canvas.getByRole("button", { name: "Skip wait" }));
    await expect(pause).not.toHaveAttribute("aria-busy", "true");
    await expect(pause).toHaveFocus();
    await expect(canvas.queryByRole("button", { name: "Skip wait" })).not.toBeInTheDocument();
  },
};
export const PodcastKeyboardControls: Story = {
  args: { kind: "podcast", podcast: { playing: true } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const notes = canvas.getByRole("tab", { name: "Notes" });
    await userEvent.click(notes);
    await userEvent.keyboard("{ArrowRight}");
    await expect(canvas.getByRole("tab", { name: "Segments (1)" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(
      canvas.getByText("A word from our sponsor … Back to the conversation"),
    ).toBeVisible();
    const progress = canvas.getByRole("slider", { name: "Episode progress" });
    progress.focus();
    // Native range defaults require a trusted browser key, which the preview's
    // synthetic userEvent cannot supply. The browser runner drives Playwright.
    const nativeKeyboard = import.meta.env.MODE === "test";
    if (nativeKeyboard) {
      const { userEvent: browserUserEvent } = await import("vitest/browser");
      await browserUserEvent.keyboard("{ArrowLeft}");
      await expect(progress).toHaveValue("119");
    }
    await userEvent.click(canvas.getByRole("button", { name: "Skip back 15 seconds" }));
    await expect(progress).toHaveValue(nativeKeyboard ? "104" : "105");
    await userEvent.selectOptions(canvas.getByRole("combobox", { name: "Playback speed" }), "1.5");
    await expect(canvas.getByRole("combobox", { name: "Playback speed" })).toHaveValue("1.5");
  },
};
export const PodcastMediaError: Story = {
  args: { kind: "podcast", podcast: { error: "This episode could not load. Try again." } },
};

function PlaybackNoticeScenario() {
  const [snapshot, setSnapshot] = useState({
    ...initialVideoSnapshot(playbackFixture),
    loading: false,
    ready: true,
  });
  const notify = (notice: string) =>
    setSnapshot((current) => ({ ...current, notice, noticeID: current.noticeID + 1 }));
  const actions: VideoActions = {
    play: () => setSnapshot((current) => ({ ...current, position: current.position + 1 })),
    pause: () => undefined,
    retry: () => undefined,
    sendToTV: () => setSnapshot((current) => ({ ...current, tvSending: !current.tvSending })),
  };
  return (
    <article className="playback-page playback-video">
      <nav className="flex flex-wrap gap-2 p-2" aria-label="Preview playback events">
        <Button onClick={() => notify("SponsorBlock skipped a video segment")}>Skip sponsor</Button>
        <Button onClick={() => notify("Switched to YouTube player")}>Use YouTube player</Button>
        <Button onClick={actions.play}>Advance playback</Button>
        <Button onClick={actions.sendToTV}>Toggle TV progress</Button>
      </nav>
      <YouTubePlayerView title={playbackFixture.item.title} snapshot={snapshot} actions={actions} />
    </article>
  );
}

export const PlaybackNotices: Story = { render: () => <PlaybackNoticeScenario /> };
