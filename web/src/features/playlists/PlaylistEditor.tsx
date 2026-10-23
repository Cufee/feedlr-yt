import { type FormEvent, useState } from "react";
import { Button, Dialog, Field, Input, Textarea } from "@/components/ui";
import { api, mediaPath } from "@/lib/api/client";
import { useAction } from "@/lib/api/queries";
import type { Playlist } from "@/lib/api/types";

export function PlaylistEditor({
  playlist,
  onClose,
  onCreated,
}: {
  playlist?: Playlist;
  onClose: () => void;
  onCreated?: (playlist: Playlist) => void;
}) {
  const [name, setName] = useState(playlist?.name ?? "");
  const [description, setDescription] = useState(playlist?.description ?? "");
  const mutation = useAction(
    () =>
      playlist
        ? api.patch<Playlist>(`/playlists/${mediaPath(playlist.id)}`, { name, description })
        : api.post<Playlist>("/playlists", { name, description }),
    {
      success: playlist ? undefined : "Playlist created",
      onSuccess: (result) => {
        onClose();
        if (!playlist) {
          onCreated?.(result);
        }
      },
    },
  );
  function submit(event: FormEvent) {
    event.preventDefault();
    mutation.mutate();
  }
  return (
    <Dialog open onClose={onClose} title={playlist ? "Edit playlist" : "Create a playlist"}>
      <form className="form-stack" onSubmit={submit}>
        <Field label="Name" htmlFor="playlist-name">
          <Input
            id="playlist-name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            autoFocus
            required
            maxLength={120}
          />
        </Field>
        <Field label="Description (optional)" htmlFor="playlist-description">
          <Textarea
            id="playlist-description"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            rows={3}
            maxLength={1000}
          />
        </Field>
        {mutation.error && (
          <p className="form-error" role="alert">
            {mutation.error.message}
          </p>
        )}
        <div className="form-actions">
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" pending={mutation.isPending}>
            {playlist ? "Save changes" : "Create playlist"}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
