import {
  ArrowPathIcon,
  BookmarkIcon,
  CheckIcon,
  EyeSlashIcon,
  MusicalNoteIcon,
  PlayIcon,
  PlusIcon,
} from "@heroicons/react/24/outline";
import { BookmarkIcon as BookmarkSolidIcon } from "@heroicons/react/24/solid";
import { Link as RouterLink } from "@tanstack/react-router";
import { type MouseEvent, useState } from "react";
import { Badge, IconButton, Menu, type MenuAction } from "@/components/ui";
import { useMediaState, useWatchLater } from "@/lib/api/queries";
import type { MediaItem } from "@/lib/api/types";
import { preserveRemovalFocus } from "@/lib/browser/removal-focus";
import { dateLabel, duration, relativeDate } from "./format";
import { PlaylistPicker } from "./PlaylistPicker";

export interface MediaCardViewProps {
  item: MediaItem;
  returnTo?: string;
  authenticated?: boolean;
  pending?: boolean;
  onWatchLater?: (event: MouseEvent<HTMLButtonElement>) => void;
  onState?: (state: { hidden?: boolean; completed?: boolean; progressSeconds?: number }) => void;
  onPlaylist?: () => void;
  contextActions?: MenuAction[];
}
export function MediaCardView({
  item,
  returnTo = "/app",
  authenticated = true,
  pending = false,
  onWatchLater,
  onState,
  onPlaylist,
  contextActions = [],
}: MediaCardViewProps) {
  const progress = item.completed
    ? 100
    : item.durationSeconds && item.durationSeconds > 0
      ? Math.max(0, Math.min(100, (item.progressSeconds / item.durationSeconds) * 100))
      : 0;
  const playingLabel = item.kind === "podcast" ? "Listen to" : "Watch";
  return (
    <article
      className={`media-card ${item.hidden ? "media-card-hidden" : ""} ${item.completed ? "media-card-completed" : ""}`}
      data-media-id={item.id}
    >
      <div className="media-artwork">
        <RouterLink
          to="/video/$mediaId"
          params={{ mediaId: item.id }}
          search={{ return: returnTo }}
          className="media-artwork-link"
          aria-label={`${playingLabel} ${item.title}`}
        >
          <img src={item.artworkUrl} alt="" loading="lazy" decoding="async" />
          <span className="media-play-hint" aria-hidden="true">
            {item.kind === "podcast" ? <MusicalNoteIcon /> : <PlayIcon />}
          </span>
        </RouterLink>
        {authenticated && (
          <IconButton
            className="media-watch-later"
            label={item.inWatchLater ? "Remove from Watch Later" : "Save to Watch Later"}
            aria-pressed={item.inWatchLater}
            variant="ghost"
            pending={pending}
            onClick={onWatchLater}
          >
            {item.inWatchLater ? <BookmarkSolidIcon /> : <BookmarkIcon />}
          </IconButton>
        )}
        {item.hidden && (
          <div className="media-artwork-badges">
            <Badge>Hidden</Badge>
          </div>
        )}
        {!item.hidden && (item.completed || item.progressSeconds > 0) && (
          <div className="media-watched-overlay">Watched</div>
        )}
        {duration(item.durationSeconds) && (
          <span className="media-duration">{duration(item.durationSeconds)}</span>
        )}
        {item.type.includes("live") && !item.type.includes("upcoming") && (
          <Badge tone="danger">Live</Badge>
        )}
        {item.type.includes("upcoming") && <Badge tone="accent">Upcoming</Badge>}
        {!item.hidden && progress > 0 && (
          <div
            className="media-progress"
            role="progressbar"
            aria-label={item.completed ? "Completed" : "Played"}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(progress)}
          >
            <span style={{ width: `${progress}%` }} />
          </div>
        )}
      </div>
      <div className="media-card-body">
        <div className="media-card-heading">
          <RouterLink
            to="/video/$mediaId"
            params={{ mediaId: item.id }}
            search={{ return: returnTo }}
            className="media-title"
          >
            {item.title}
          </RouterLink>
          {authenticated && (
            <Menu
              label={`Actions for ${item.title}`}
              iconOnly
              variant="ghost"
              size="sm"
              buttonClassName="media-actions"
              items={[
                {
                  id: "playlist",
                  label: "Add to playlist",
                  icon: <PlusIcon />,
                  onSelect: onPlaylist,
                },
                {
                  id: "completed",
                  label: item.completed ? "Mark as unwatched" : "Mark as watched",
                  icon: item.completed ? <ArrowPathIcon /> : <CheckIcon />,
                  onSelect: () =>
                    onState?.(item.completed ? { completed: false } : { completed: true }),
                },
                {
                  id: "hidden",
                  label: item.hidden ? "Show in feed" : "Hide from feed",
                  icon: <EyeSlashIcon />,
                  onSelect: () => onState?.({ hidden: !item.hidden }),
                },
                ...contextActions,
              ]}
            />
          )}
        </div>
        <RouterLink
          to="/channel/$channelId"
          params={{ channelId: item.channel.id }}
          className="media-channel"
        >
          {item.channel.title}
        </RouterLink>
        <div className="media-card-foot">
          <time
            className="media-detail"
            dateTime={item.publishedAt}
            title={dateLabel(item.publishedAt)}
          >
            {relativeDate(item.publishedAt)}
          </time>
        </div>
      </div>
    </article>
  );
}

export function MediaCard({
  item,
  returnTo,
  authenticated = true,
  contextActions,
}: {
  item: MediaItem;
  returnTo?: string;
  authenticated?: boolean;
  contextActions?: MenuAction[];
}) {
  const [pickerOpen, setPickerOpen] = useState(false);
  const { setPresent, pending } = useWatchLater(item.id);
  const state = useMediaState(item.id);
  function toggleWatchLater(event: MouseEvent<HTMLButtonElement>) {
    const article = event.currentTarget.closest("article");
    const removing = item.inWatchLater && returnTo?.startsWith("/app/watch-later");
    const stop = removing ? preserveRemovalFocus(article) : () => undefined;
    void setPresent(!item.inWatchLater).catch(stop);
  }
  return (
    <>
      <MediaCardView
        item={item}
        returnTo={returnTo}
        authenticated={authenticated}
        contextActions={contextActions}
        pending={pending || state.isPending}
        onWatchLater={toggleWatchLater}
        onState={(value) => state.mutate(value)}
        onPlaylist={() => setPickerOpen(true)}
      />
      {pickerOpen && <PlaylistPicker mediaId={item.id} open onClose={() => setPickerOpen(false)} />}
    </>
  );
}

export function MediaGrid({
  items,
  returnTo,
  authenticated = true,
  busy = false,
}: {
  items: MediaItem[];
  returnTo?: string;
  authenticated?: boolean;
  busy?: boolean;
}) {
  return (
    <div className="media-grid" aria-busy={busy}>
      {items.map((item) => (
        <MediaCard key={item.id} item={item} returnTo={returnTo} authenticated={authenticated} />
      ))}
    </div>
  );
}
