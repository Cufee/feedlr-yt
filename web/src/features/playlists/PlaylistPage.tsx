import {
  ArrowPathIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  PencilSquareIcon,
  PlusIcon,
  TrashIcon,
} from "@heroicons/react/24/outline";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Page, PageLoading, RequestError } from "@/app/Page";
import { MediaCard } from "@/components/media/MediaCard";
import { URLDialog } from "@/components/media/URLDialog";
import { Badge, Button, EmptyState, IconButton, Progress } from "@/components/ui";
import { ConfirmDialog } from "@/components/ui-compositions";
import { api, mediaPath } from "@/lib/api/client";
import { useAction, usePlaylist } from "@/lib/api/queries";
import { preserveRemovalFocus } from "@/lib/browser/removal-focus";
import { PlaylistEditor } from "./PlaylistEditor";

export function PlaylistPage({ playlistId }: { playlistId: string }) {
  const { data, isPending, error, refetch } = usePlaylist(playlistId);
  const [dialog, setDialog] = useState<"edit" | "delete" | "add" | null>(null);
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
  const itemAction = useAction(({ id, direction }: { id: string; direction?: "up" | "down" }) =>
    direction
      ? api.post(`${path}/items/${mediaPath(id)}/move`, { direction })
      : api.put(`${path}/items/${mediaPath(id)}`, { present: false }),
  );
  if (isPending) {
    return <PageLoading />;
  }
  if (error) {
    return <RequestError error={error} retry={() => void refetch()} />;
  }
  return (
    <Page
      title={data.playlist.name}
      description={data.playlist.description || undefined}
      action={
        <>
          {data.playlist.youtubePlaylistId && (
            <Button pending={sync.isPending} onClick={() => sync.mutate()}>
              <ArrowPathIcon />
              Refresh import
            </Button>
          )}
          {!data.playlist.system && (
            <>
              <Button onClick={() => setDialog("edit")}>
                <PencilSquareIcon />
                Edit
              </Button>
              <IconButton label="Delete playlist" onClick={() => setDialog("delete")}>
                <TrashIcon />
              </IconButton>
            </>
          )}
          <Button variant="primary" onClick={() => setDialog("add")}>
            <PlusIcon />
            Add a video
          </Button>
        </>
      }
    >
      <div className="playlist-summary">
        <Badge>{data.items.length} items</Badge>
        <Progress value={data.playlist.progressPercent} label="Watched" />
      </div>
      {data.items.length ? (
        <div className="media-grid">
          {data.items.map((item, index) => (
            <div className="playlist-entry" key={item.id}>
              <MediaCard item={item} returnTo={`/app/playlist/${playlistId}`} />
              <div className="playlist-entry-actions">
                <span className="muted">{index + 1}</span>
                <div>
                  <IconButton
                    label={`Move ${item.title} up`}
                    size="sm"
                    disabled={index === 0 || itemAction.isPending}
                    onClick={() => itemAction.mutate({ id: item.id, direction: "up" })}
                  >
                    <ChevronUpIcon />
                  </IconButton>
                  <IconButton
                    label={`Move ${item.title} down`}
                    size="sm"
                    disabled={index === data.items.length - 1 || itemAction.isPending}
                    onClick={() => itemAction.mutate({ id: item.id, direction: "down" })}
                  >
                    <ChevronDownIcon />
                  </IconButton>
                  <IconButton
                    label={`Remove ${item.title} from playlist`}
                    size="sm"
                    disabled={itemAction.isPending}
                    onClick={(event) => {
                      const stop = preserveRemovalFocus(
                        event.currentTarget.closest(".playlist-entry"),
                      );
                      void itemAction.mutateAsync({ id: item.id }).catch(stop);
                    }}
                  >
                    <TrashIcon />
                  </IconButton>
                </div>
              </div>
            </div>
          ))}
        </div>
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
  );
}
