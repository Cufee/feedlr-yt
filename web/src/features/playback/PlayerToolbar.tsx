import {
  ArrowLeftIcon,
  BookmarkIcon,
  ListBulletIcon,
  ShareIcon,
  TvIcon,
} from "@heroicons/react/24/outline";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { PlaylistPicker } from "@/components/media/PlaylistPicker";
import { Button, Dialog, Field, Input, Menu, type MenuAction } from "@/components/ui";
import { useWatchLater } from "@/lib/api/queries";
import type { MediaDetail } from "@/lib/api/types";
import type { VideoSnapshot } from "./adapters/runtime";
import { useFullscreenElement } from "./adapters/usePlayback";

export interface PlayerToolbarProps {
  detail: MediaDetail;
  returnTo?: string;
  saved: boolean;
  saving?: boolean;
  onSave: () => void;
  onPlaylists: () => void;
  onShare: () => void;
  tv?: Pick<VideoSnapshot, "tvOnline" | "tvSending" | "tvScreenName" | "ready" | "switching">;
  onSendToTV?: () => void;
  portalTarget?: HTMLElement | null;
}
export function PlayerToolbarView({
  detail,
  returnTo = "/app",
  saved,
  saving = false,
  onSave,
  onPlaylists,
  onShare,
  tv,
  onSendToTV,
  portalTarget,
}: PlayerToolbarProps) {
  const { item, authenticated } = detail;
  const back = returnTo.startsWith("/") && !returnTo.startsWith("//") ? returnTo : "/app";
  const items: MenuAction[] = [
    {
      id: "watch-later",
      label: saving ? "Saving…" : saved ? "Remove from Watch Later" : "Save to Watch Later",
      disabled: !authenticated || saving,
      checked: saved,
      pending: saving,
      icon: <BookmarkIcon aria-hidden="true" />,
      onSelect: onSave,
    },
    {
      id: "playlist",
      label: "Add to playlist",
      disabled: !authenticated,
      icon: <ListBulletIcon aria-hidden="true" />,
      onSelect: onPlaylists,
    },
    { id: "share", label: "Share", icon: <ShareIcon aria-hidden="true" />, onSelect: onShare },
  ];
  if (item.kind === "youtube" && tv?.tvOnline && authenticated) {
    items.push({
      id: "tv",
      label: tv.tvSending
        ? "Sending to TV…"
        : `Send to TV${tv.tvScreenName ? ` (${tv.tvScreenName})` : ""}`,
      disabled: tv.tvSending || !tv.ready || tv.switching,
      pending: tv.tvSending,
      icon: <TvIcon aria-hidden="true" />,
      onSelect: onSendToTV,
    });
  }
  return (
    <header className="playback-toolbar">
      <div className="playback-identity">
        <Link className="playback-back" to={back} aria-label="Close player">
          <ArrowLeftIcon aria-hidden="true" />
        </Link>
        <div className="playback-identity-copy">
          {item.kind === "youtube" && (
            <span className="playback-toolbar-title" title={item.title}>
              {item.title}
            </span>
          )}
          <Link
            className="playback-channel"
            to="/channel/$channelId"
            params={{ channelId: item.channel.id }}
            title={item.channel.title}
          >
            {item.channel.title}
          </Link>
        </div>
      </div>
      <Menu
        label="Player actions"
        items={items}
        iconOnly
        className="playback-actions"
        buttonClassName="playback-menu-trigger"
        portalTarget={portalTarget}
      />
    </header>
  );
}

export function PlayerToolbar({
  detail,
  returnTo,
  tv,
  onSendToTV,
}: Pick<PlayerToolbarProps, "detail" | "returnTo" | "tv" | "onSendToTV">) {
  const [playlistsOpen, setPlaylistsOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  const [copyError, setCopyError] = useState("");
  const membership = useWatchLater(detail.item.id);
  const fullscreen = useFullscreenElement();
  const link = new URL(`/video/${encodeURIComponent(detail.item.id)}`, window.location.origin).href;
  async function share() {
    try {
      if (navigator.share) {
        await navigator.share({ title: detail.item.title, url: link });
      } else {
        await navigator.clipboard.writeText(link);
        toast.success("Link copied");
      }
    } catch (error) {
      if (error instanceof Error && error.name === "AbortError") {
        return;
      }
      setCopyError("");
      setShareOpen(true);
    }
  }
  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
      setCopyError("");
      toast.success("Link copied");
      setShareOpen(false);
    } catch {
      setCopyError("Select the link and copy it with your browser.");
    }
  }
  return (
    <>
      <PlayerToolbarView
        detail={detail}
        returnTo={returnTo}
        saved={detail.item.inWatchLater}
        saving={membership.pending}
        onSave={() => {
          void membership.setPresent(!detail.item.inWatchLater).catch(() => undefined);
        }}
        onPlaylists={() => setPlaylistsOpen(true)}
        onShare={() => {
          void share();
        }}
        tv={tv}
        onSendToTV={onSendToTV}
        portalTarget={fullscreen}
      />
      {detail.authenticated && (
        <PlaylistPicker
          mediaId={detail.item.id}
          open={playlistsOpen}
          onClose={() => setPlaylistsOpen(false)}
          portalTarget={fullscreen}
        />
      )}
      <Dialog
        open={shareOpen}
        onClose={() => setShareOpen(false)}
        title="Share"
        portalTarget={fullscreen}
        footer={
          <Button
            onClick={() => {
              void copy();
            }}
          >
            Copy link
          </Button>
        }
      >
        <Field label="Link" error={copyError || undefined}>
          <Input readOnly value={link} onFocus={(event) => event.currentTarget.select()} />
        </Field>
      </Dialog>
    </>
  );
}
