import { ArrowDownTrayIcon, PlusIcon, QueueListIcon } from "@heroicons/react/24/outline";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Page, PageLoading, RequestError } from "@/app/Page";
import { URLDialog } from "@/components/media/URLDialog";
import { Badge, Button, EmptyState, Progress } from "@/components/ui";
import { api } from "@/lib/api/client";
import { usePlaylists } from "@/lib/api/queries";
import { PlaylistEditor } from "./PlaylistEditor";

export function PlaylistsPage() {
  const { data, isPending, error, refetch } = usePlaylists();
  const [dialog, setDialog] = useState<"create" | "import" | null>(null);
  const navigate = useNavigate();
  const items = data?.items.filter((item) => !item.system) ?? [];
  return (
    <Page
      title="Playlists"
      description="Your playlists of videos and podcast episodes."
      showDescription={false}
      action={
        <>
          <Button onClick={() => setDialog("import")}>
            <ArrowDownTrayIcon />
            Import
          </Button>
          <Button variant="primary" onClick={() => setDialog("create")}>
            <PlusIcon />
            Create playlist
          </Button>
        </>
      }
    >
      {isPending ? (
        <PageLoading />
      ) : error ? (
        <RequestError error={error} retry={() => void refetch()} />
      ) : items.length === 0 ? (
        <EmptyState
          title="No playlists"
          description="Create a playlist or import one from YouTube."
        />
      ) : (
        <div className="playlist-grid">
          {items.map((playlist) => (
            <Link
              className="playlist-card panel"
              key={playlist.id}
              to="/app/playlist/$playlistId"
              params={{ playlistId: playlist.id }}
            >
              <div className="playlist-artwork">
                {playlist.thumbnailUrl ? (
                  <img src={playlist.thumbnailUrl} alt="" loading="lazy" />
                ) : (
                  <QueueListIcon />
                )}
                <Badge>{playlist.videoCount} items</Badge>
              </div>
              <div className="playlist-card-copy">
                <h2>{playlist.name}</h2>
                {playlist.description && <p>{playlist.description}</p>}
                {playlist.youtubePlaylistId && <Badge tone="accent">YouTube import</Badge>}
                <Progress value={playlist.progressPercent} label="Watched" />
              </div>
            </Link>
          ))}
        </div>
      )}
      {dialog === "create" && (
        <PlaylistEditor
          onClose={() => setDialog(null)}
          onCreated={(playlist) =>
            void navigate({ to: "/app/playlist/$playlistId", params: { playlistId: playlist.id } })
          }
        />
      )}
      {dialog === "import" && (
        <URLDialog
          open
          onClose={() => setDialog(null)}
          title="Import a YouTube playlist"
          label="YouTube playlist link"
          submitLabel="Import playlist"
          onSubmit={(url) => api.post("/playlists/import", { url })}
        />
      )}
    </Page>
  );
}
