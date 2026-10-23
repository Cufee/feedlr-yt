import { PlayIcon } from "@heroicons/react/24/solid";
import { type Ref, useState } from "react";
import { Button, Toast } from "@/components/ui";
import type { MediaDetail } from "@/lib/api/types";
import { useToastTimer } from "@/lib/browser/useToastTimer";
import type { VideoActions, VideoSnapshot } from "./adapters/runtime";
import { useVideoPlayback } from "./adapters/usePlayback";
import { PlayerToolbar } from "./PlayerToolbar";
import "./playback.css";

function PlaybackStatus({ message, sending }: { message: string; sending: boolean }) {
  const [visible, setVisible] = useState(true);
  const dismiss = () => setVisible(false);
  const lifetime = useToastTimer(6_000, dismiss);
  if (sending) {
    return (
      <div className="playback-video-status" role="status" aria-live="polite">
        Sending to TV…
      </div>
    );
  }
  return visible && message ? (
    <Toast {...lifetime} className="playback-video-status" tone="info" onDismiss={dismiss}>
      {message}
    </Toast>
  ) : null;
}

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
      {(snapshot.notice || snapshot.tvSending) && (
        <PlaybackStatus
          key={snapshot.noticeID}
          message={snapshot.notice}
          sending={snapshot.tvSending}
        />
      )}
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
