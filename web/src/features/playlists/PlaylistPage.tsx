import {
  ArrowPathIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  PencilSquareIcon,
  PlusIcon,
  TrashIcon,
} from "@heroicons/react/24/outline";
import { useNavigate } from "@tanstack/react-router";
import { useRef, useState } from "react";
import { Page, PageLoading, RequestError } from "@/app/Page";
import { MediaCard } from "@/components/media/MediaCard";
import { URLDialog } from "@/components/media/URLDialog";
import { Badge, Button, EmptyState, Menu, type MenuAction, Progress } from "@/components/ui";
import { ConfirmDialog } from "@/components/ui-compositions";
import { api, mediaPath } from "@/lib/api/client";
import { useAction, usePlaylist } from "@/lib/api/queries";
import type { MediaItem } from "@/lib/api/types";
import { preserveRemovalFocus } from "@/lib/browser/removal-focus";
import { PlaylistEditor } from "./PlaylistEditor";
import "./playlists.css";

type ItemAction = { id: string; direction?: "up" | "down" };

function restoreMenuFocus(trigger: HTMLButtonElement | null | undefined) {
  requestAnimationFrame(() => {
    if (
      trigger?.isConnected &&
      (document.activeElement === document.body || document.activeElement === trigger)
    ) {
      trigger.focus({ preventScroll: true });
    }
  });
}

function PlaylistEntry({
  item,
  index,
  count,
  playlistId,
  editable,
  pending,
  onAction,
}: {
  item: MediaItem;
  index: number;
  count: number;
  playlistId: string;
  editable: boolean;
  pending: boolean;
  onAction: (action: ItemAction) => Promise<unknown>;
}) {
  const entry = useRef<HTMLLIElement>(null);
  function move(direction: "up" | "down") {
    const trigger = entry.current?.querySelector<HTMLButtonElement>('[aria-haspopup="menu"]');
    void onAction({ id: item.id, direction })
      .then(() => {
        // Moving a keyed row can detach its focused trigger during the refetch.
        // Restore it after the commit only if focus has not moved elsewhere.
        restoreMenuFocus(trigger);
      })
      .catch(() => undefined);
  }
  return (
    <li className="playlist-entry" ref={entry}>
      <MediaCard
        item={item}
        returnTo={`/app/playlist/${playlistId}`}
        contextActions={
          editable
            ? [
                {
                  id: "move-up",
                  label: "Move up",
                  icon: <ChevronUpIcon />,
                  disabled: index === 0 || pending,
                  onSelect: () => move("up"),
                },
                {
                  id: "move-down",
                  label: "Move down",
                  icon: <ChevronDownIcon />,
                  disabled: index === count - 1 || pending,
                  onSelect: () => move("down"),
                },
                {
                  id: "remove",
                  label: "Remove from playlist",
                  icon: <TrashIcon />,
                  danger: true,
                  disabled: pending,
                  onSelect: () => {
                    const stop = preserveRemovalFocus(entry.current);
                    void onAction({ id: item.id }).catch(stop);
                  },
                },
              ]
            : []
        }
      />
    </li>
  );
}

export function PlaylistPage({ playlistId }: { playlistId: string }) {
  const { data, isPending, error, refetch } = usePlaylist(playlistId);
  const [dialog, setDialog] = useState<"edit" | "delete" | "add" | null>(null);
  const actionsButton = useRef<HTMLButtonElement>(null);
  const navigate = useNavigate();
  const path = `/playlists/${mediaPath(playlistId)}`;
  const remove = useAction(() => api.delete(path), {
    success: "Playlist deleted",
    onSuccess: () => {
      setDialog(null);
      void navigate({ to: "/app/playlists" });
    },
  });
  const sync = useAction(() => api.post(`${path}/sync`), { success: "Playlist refreshed" });
  const itemAction = useAction(({ id, direction }: ItemAction) =>
    direction
      ? api.post(`${path}/items/${mediaPath(id)}/move`, { direction })
      : api.put(`${path}/items/${mediaPath(id)}`, { present: false }),
  );
  function refreshImport() {
    const trigger = actionsButton.current;
    void sync
      .mutateAsync()
      .finally(() => restoreMenuFocus(trigger))
      .catch(() => undefined);
  }
  if (isPending) {
    return <PageLoading />;
  }
  if (error) {
    return <RequestError error={error} retry={() => void refetch()} />;
  }
  const actions: MenuAction[] = data.playlist.system
    ? []
    : [
        ...(data.playlist.youtubePlaylistId
          ? [
              {
                id: "sync",
                label: "Refresh import",
                icon: <ArrowPathIcon />,
                pending: sync.isPending,
                onSelect: refreshImport,
              },
            ]
          : []),
        {
          id: "edit",
          label: "Edit playlist",
          icon: <PencilSquareIcon />,
          onSelect: () => setDialog("edit"),
        },
        {
          id: "delete",
          label: "Delete playlist",
          icon: <TrashIcon />,
          danger: true,
          onSelect: () => setDialog("delete"),
        },
      ];
  return (
    <div className="playlist-page">
      <Page
        title={data.playlist.name}
        description={data.playlist.description || undefined}
        action={
          !data.playlist.system && (
            <div className="playlist-header-actions">
              <Button size="sm" variant="primary" onClick={() => setDialog("add")}>
                <PlusIcon />
                Add a video
              </Button>
              <Menu ref={actionsButton} label="Playlist actions" iconOnly items={actions} />
            </div>
          )
        }
      >
        <div className="playlist-summary">
          <Badge>{data.items.length} items</Badge>
          <Progress value={data.playlist.progressPercent} label="Watched" />
        </div>
        {data.items.length ? (
          <ol className="media-grid playlist-items" aria-label="Playlist items">
            {data.items.map((item, index) => (
              <PlaylistEntry
                key={item.id}
                item={item}
                index={index}
                count={data.items.length}
                playlistId={playlistId}
                editable={!data.playlist.system}
                pending={itemAction.isPending}
                onAction={itemAction.mutateAsync}
              />
            ))}
          </ol>
        ) : (
          <EmptyState
            title="No items"
            description="Add a video link, or use Add to playlist on any video or episode."
          />
        )}
        {dialog === "edit" && (
          <PlaylistEditor playlist={data.playlist} onClose={() => setDialog(null)} />
        )}
        {dialog === "delete" && (
          <ConfirmDialog
            open
            onClose={() => setDialog(null)}
            title={`Delete “${data.playlist.name}”?`}
            description="The playlist will be removed. Your subscriptions and viewing history will be kept."
            pending={remove.isPending}
            error={remove.error}
            onConfirm={() => remove.mutate()}
          />
        )}
        {dialog === "add" && (
          <URLDialog
            open
            onClose={() => setDialog(null)}
            title="Add a video"
            label="YouTube video link"
            submitLabel="Add to playlist"
            onSubmit={(url) => api.post(`${path}/items`, { url })}
          />
        )}
      </Page>
    </div>
  );
}
