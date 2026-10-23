import { PauseIcon, PlayIcon, SpeakerWaveIcon } from "@heroicons/react/24/solid";
import { type Ref, useId } from "react";
import { Badge, Button, IconButton, Select, Tabs } from "@/components/ui";
import type { MediaDetail } from "@/lib/api/types";
import type { PodcastActions, PodcastSnapshot } from "./adapters/runtime";
import { usePodcastPlayback } from "./adapters/usePlayback";
import { PlayerToolbar } from "./PlayerToolbar";
import "./playback.css";

export function formatPlaybackTime(seconds: number) {
  const value = Math.max(0, Math.floor(Number.isFinite(seconds) ? seconds : 0));
  const hours = Math.floor(value / 3600),
    minutes = Math.floor(value / 60) % 60;
  return `${hours ? `${hours}:${String(minutes).padStart(2, "0")}` : minutes}:${String(value % 60).padStart(2, "0")}`;
}
function SkipIcon({ forward = false }: { forward?: boolean }) {
  return (
    <svg viewBox="0 0 32 32" fill="none" aria-hidden="true">
      <g transform={forward ? "translate(32 0) scale(-1 1)" : undefined}>
        <path
          d="M8.5 7.5a11 11 0 1 1-3.4 12.9M8.5 7.5H2.7m5.8 0V1.7"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </g>
      <text
        x="16"
        y="17"
        fill="currentColor"
        textAnchor="middle"
        dominantBaseline="central"
        fontSize="10"
        fontWeight="650"
      >
        15
      </text>
    </svg>
  );
}
function analysisMessage(snapshot: PodcastSnapshot) {
  if (snapshot.analysisStatus === "failed") {
    return snapshot.segments.length
      ? "Scan incomplete. Available segments are shown below."
      : "Segments could not load.";
  }
  if (snapshot.analysisStatus === "unavailable") {
    return "No segments available.";
  }
  if (snapshot.analysisStatus === "idle") {
    return "Play to check for segments.";
  }
  if (["pending", "running"].includes(snapshot.analysisStatus)) {
    if (snapshot.analysisPhase === "validating") {
      return "Checking audio…";
    }
    if (snapshot.analysisPhase === "scanning") {
      return "Finding sponsors…";
    }
    return "Preparing transcript…";
  }
  return snapshot.segments.length ? "" : "No segments found.";
}
export function PodcastSegments({ snapshot }: { snapshot: PodcastSnapshot }) {
  const message = analysisMessage(snapshot);
  const active = ["pending", "running"].includes(snapshot.analysisStatus);
  return (
    <div className="playback-segments">
      {message && (
        <p className="playback-analysis-status" role="status">
          {active && <span className="ui-spinner" aria-hidden="true" />}
          {message}
        </p>
      )}
      {snapshot.segments.length > 0 && !snapshot.validated && (
        <p className="playback-help">Segments will be checked when playback starts.</p>
      )}
      {snapshot.segments.map((segment) => (
        <section
          key={`${segment.category}:${segment.start_ms}:${segment.end_ms}:${segment.brand}`}
          className="playback-segment"
        >
          <div>
            <Badge tone="accent">{segment.category}</Badge>
            <span>
              {formatPlaybackTime(segment.start_ms / 1000)}–
              {formatPlaybackTime(segment.end_ms / 1000)}
            </span>
          </div>
          <p>
            {segment.start_text}
            {segment.end_text !== segment.start_text ? ` … ${segment.end_text}` : ""}
          </p>
        </section>
      ))}
    </div>
  );
}
export function PodcastPlayerView({
  detail,
  snapshot,
  actions,
  rootRef,
}: {
  detail: MediaDetail;
  snapshot: PodcastSnapshot;
  actions: PodcastActions;
  rootRef?: Ref<HTMLDivElement>;
}) {
  const progressID = useId(),
    volumeID = useId();
  const { item } = detail;
  const busy = snapshot.loading || snapshot.scanHold;
  const notes = item.descriptionHtml ? (
    // biome-ignore lint/security/noDangerouslySetInnerHtml: The v1 DTO sanitizes podcast notes with shared.SanitizePodcastHTML (internal/server/webapi/dto.go).
    <div className="playback-prose" dangerouslySetInnerHTML={{ __html: item.descriptionHtml }} />
  ) : (
    <p className="playback-prose">{item.description || "No episode notes provided."}</p>
  );
  const panels = [{ id: "notes", label: "Notes", content: notes }];
  if (snapshot.segmentsEnabled) {
    panels.push({
      id: "segments",
      label: `Segments${snapshot.segments.length ? ` (${snapshot.segments.length})` : ""}`,
      content: <PodcastSegments snapshot={snapshot} />,
    });
  }
  return (
    <div className="playback-podcast-body">
      <div ref={rootRef} className="playback-audio-engine" />
      <div className="playback-listening">
        {item.artworkUrl && (
          <div className="playback-artwork-panel">
            <img className="playback-artwork" src={item.artworkUrl} alt="" />
          </div>
        )}
        <div className="playback-podcast-main">
          <h1>{item.title}</h1>
          <section className="playback-podcast-controls" aria-label="Podcast playback controls">
            {snapshot.scanHold && (
              <div className="playback-scan-hold">
                <p role="status">Checking for sponsors…</p>
                <Button
                  size="sm"
                  onClick={(event) => {
                    event.currentTarget
                      .closest("section")
                      ?.querySelector<HTMLElement>(".playback-play")
                      ?.focus();
                    actions.skipScanWait();
                  }}
                >
                  Skip wait
                </Button>
              </div>
            )}
            {snapshot.error && (
              <div className="playback-media-error">
                <p role="alert">{snapshot.error}</p>
                <Button onClick={actions.retry}>Try again</Button>
              </div>
            )}
            <div className="playback-timeline">
              <label htmlFor={progressID} className="sr-only">
                Episode progress
              </label>
              <div className="playback-range-wrap">
                <div className="playback-range-track" aria-hidden="true">
                  <div
                    className="playback-range-progress"
                    style={{
                      width: `${snapshot.duration > 0 ? Math.max(0, Math.min(100, (snapshot.position / snapshot.duration) * 100)) : 0}%`,
                    }}
                  />
                  <div className="playback-segment-markers">
                    {snapshot.segmentsEnabled &&
                      snapshot.duration > 0 &&
                      snapshot.segments
                        .filter((segment) => segment.skippable)
                        .map((segment) => {
                          const left = Math.max(
                            0,
                            Math.min(100, (segment.start_ms / 1000 / snapshot.duration) * 100),
                          );
                          const right = Math.max(
                            left,
                            Math.min(100, (segment.end_ms / 1000 / snapshot.duration) * 100),
                          );
                          return (
                            <span
                              key={`${segment.category}:${segment.start_ms}:${segment.end_ms}`}
                              style={{ left: `${left}%`, width: `${right - left}%` }}
                            />
                          );
                        })}
                  </div>
                </div>
                <input
                  id={progressID}
                  className="playback-range"
                  type="range"
                  min="0"
                  max={snapshot.duration || 1}
                  step="1"
                  value={Math.min(snapshot.position, snapshot.duration || 0)}
                  disabled={!snapshot.duration}
                  aria-valuetext={`${formatPlaybackTime(snapshot.position)} of ${formatPlaybackTime(snapshot.duration)}`}
                  onChange={(event) => actions.scrub(Number(event.currentTarget.value))}
                  onPointerUp={actions.finishScrub}
                  onPointerCancel={actions.finishScrub}
                  onKeyUp={actions.finishScrub}
                  onBlur={actions.finishScrub}
                />
              </div>
              <div className="playback-time">
                <span>{formatPlaybackTime(snapshot.position)}</span>
                <span>−{formatPlaybackTime(snapshot.duration - snapshot.position)}</span>
              </div>
            </div>
            <div className="playback-transport">
              <IconButton
                label="Skip back 15 seconds"
                variant="ghost"
                onClick={() => actions.seekBy(-15)}
              >
                <SkipIcon />
              </IconButton>
              <Button
                variant="primary"
                size="lg"
                className="playback-play"
                onClick={actions.toggle}
                aria-label={snapshot.playing ? "Pause episode" : "Play episode"}
                aria-busy={busy}
                disabled={snapshot.loading && !snapshot.scanHold && !snapshot.blocked}
              >
                {busy ? (
                  <span className="ui-spinner" aria-hidden="true" />
                ) : snapshot.playing ? (
                  <PauseIcon aria-hidden="true" />
                ) : (
                  <PlayIcon aria-hidden="true" />
                )}
              </Button>
              <IconButton
                label="Skip forward 15 seconds"
                variant="ghost"
                onClick={() => actions.seekBy(15)}
              >
                <SkipIcon forward />
              </IconButton>
            </div>
            <div className="playback-preferences">
              <Select
                aria-label="Playback speed"
                value={snapshot.rate}
                onChange={(event) => actions.setRate(Number(event.currentTarget.value))}
              >
                {[0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3].map((rate) => (
                  <option key={rate} value={rate}>
                    {rate}×
                  </option>
                ))}
              </Select>
              <div className="playback-volume">
                <SpeakerWaveIcon aria-hidden="true" />
                <label htmlFor={volumeID} className="sr-only">
                  Volume
                </label>
                <input
                  id={volumeID}
                  className="playback-range"
                  type="range"
                  min="0"
                  max="100"
                  value={snapshot.volume}
                  onChange={(event) => actions.setVolume(Number(event.currentTarget.value))}
                />
              </div>
            </div>
          </section>
        </div>
      </div>
      <section className="playback-notes" aria-label="Episode details">
        {panels.length > 1 ? <Tabs label="Episode details" items={panels} /> : notes}
      </section>
    </div>
  );
}

export function PodcastPlayer({ detail, returnTo }: { detail: MediaDetail; returnTo?: string }) {
  const playback = usePodcastPlayback(detail);
  return (
    <article className="playback-page playback-podcast">
      <PlayerToolbar detail={detail} returnTo={returnTo} />
      <PodcastPlayerView
        detail={detail}
        snapshot={playback.snapshot}
        actions={playback.actions}
        rootRef={playback.root}
      />
    </article>
  );
}
