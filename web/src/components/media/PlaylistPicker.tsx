import { Button, Checkbox, Dialog, EmptyState, Skeleton } from "@/components/ui";
import { api, mediaPath } from "@/lib/api/client";
import { useAction, useMediaDetail, usePlaylists } from "@/lib/api/queries";

export function PlaylistPicker({
  mediaId,
  open,
  onClose,
  membershipIds,
  portalTarget,
}: {
  mediaId: string;
  open: boolean;
  onClose: () => void;
  membershipIds?: string[];
  portalTarget?: HTMLElement | null;
}) {
  const playlists = usePlaylists();
  const detail = useMediaDetail(mediaId);
  const membership = membershipIds ?? detail.data?.membershipIds ?? [];
  const mutation = useAction(({ playlistId, present }: { playlistId: string; present: boolean }) =>
    api.put(`/playlists/${mediaPath(playlistId)}/items/${mediaPath(mediaId)}`, { present }),
  );
  const items = playlists.data?.items.filter((item) => !item.system) ?? [];
  return (
    <Dialog
      open={open}
      onClose={onClose}
      portalTarget={portalTarget}
      title="Add to playlist"
      footer={<Button onClick={onClose}>Done</Button>}
    >
      {playlists.isPending || detail.isPending ? (
        <Skeleton className="h-24" />
      ) : playlists.error || detail.error ? (
        <div role="alert">
          <p>{(playlists.error ?? detail.error)?.message}</p>
          <Button
            onClick={() => {
              void playlists.refetch();
              void detail.refetch();
            }}
          >
            Try again
          </Button>
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          title="No playlists"
          description="Create a playlist to add items."
          action={
            <a className="ui-link" href="/app/playlists">
              Go to playlists
            </a>
          }
        />
      ) : (
        <div className="settings-list">
          {items.map((item) => (
            <Checkbox
              key={item.id}
              label={item.name}
              description={`${item.videoCount} items`}
              checked={membership.includes(item.id)}
              disabled={mutation.isPending}
              onChange={(present) => mutation.mutate({ playlistId: item.id, present })}
            />
          ))}
        </div>
      )}
      {mutation.error && (
        <p className="form-error" role="alert">
          {mutation.error.message}
        </p>
      )}
    </Dialog>
  );
}
