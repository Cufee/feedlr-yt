import { PlayIcon } from "@heroicons/react/24/solid";
import type { Ref } from "react";
import { Button } from "@/components/ui";
import type { MediaDetail } from "@/lib/api/types";
import { usePlaybackNotifications } from "@/lib/browser/usePlaybackNotifications";
import type { VideoActions, VideoSnapshot } from "./adapters/runtime";
import { useVideoPlayback } from "./adapters/usePlayback";
import { PlayerToolbar } from "./PlayerToolbar";
import "./playback.css";

export function YouTubePlayerView({
  title,
  snapshot,
  actions,
  rootRef,
}: {
  title: string;
  snapshot: VideoSnapshot;
  actions: VideoActions;
  rootRef?: Ref<HTMLFieldSetElement>;
}) {
  usePlaybackNotifications(snapshot);
  return (
    <div className="playback-video-body">
      <h1 className="sr-only">{title}</h1>
      <section
        className="playback-video-stage"
        aria-label="Video player"
        aria-busy={snapshot.loading || undefined}
      >
        <fieldset
          ref={rootRef}
          className="playback-video-engine"
          aria-label="Playback controls"
          tabIndex={-1}
        />
        {snapshot.blocked && !snapshot.loading && (
          <div className="playback-stage-prompt">
            <Button
              variant="primary"
              size="lg"
              onClick={(event) => {
                event.currentTarget
                  .closest(".playback-video-stage")
                  ?.querySelector<HTMLElement>(".playback-video-engine")
                  ?.focus();
                actions.play();
              }}
            >
              <PlayIcon aria-hidden="true" />
              Play video
            </Button>
          </div>
        )}
        {snapshot.error && (
          <div className="playback-stage-prompt">
            <p role="alert">{snapshot.error}</p>
            <Button
              onClick={(event) => {
                event.currentTarget
                  .closest(".playback-video-stage")
                  ?.querySelector<HTMLElement>(".playback-video-engine")
                  ?.focus();
                actions.retry();
              }}
            >
              Try again
            </Button>
          </div>
        )}
      </section>
    </div>
  );
}

export function YouTubePlayer({ detail, returnTo }: { detail: MediaDetail; returnTo?: string }) {
  const playback = useVideoPlayback(detail);
  return (
    <article className="playback-page playback-video">
      <PlayerToolbar
        detail={detail}
        returnTo={returnTo}
        tv={playback.snapshot}
        onSendToTV={playback.actions.sendToTV}
      />
      <YouTubePlayerView
        title={detail.item.title}
        snapshot={playback.snapshot}
        actions={playback.actions}
        rootRef={playback.root}
      />
    </article>
  );
}
