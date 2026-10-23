import { useNavigate } from "@tanstack/react-router";
import { Metadata } from "@/app/Metadata";
import { Button, EmptyState, Skeleton } from "@/components/ui";
import { safeReturn } from "@/features/auth/passkeys";
import { useMediaDetail } from "@/lib/api/queries";
import { usePlayerEscape } from "@/lib/browser/usePlayerEscape";
import { PodcastPlayer } from "./PodcastPlayer";
import { YouTubePlayer } from "./YouTubePlayer";

export function PlayerPage({ mediaId, returnTo = "/app" }: { mediaId: string; returnTo?: string }) {
  const detail = useMediaDetail(mediaId);
  const navigate = useNavigate();
  usePlayerEscape(() => {
    void navigate({ to: safeReturn(returnTo) });
  });
  if (detail.isPending) {
    return (
      <div className="playback-page" role="status" aria-label="Loading player">
        <Skeleton className="h-16" />
        <Skeleton className="min-h-96" />
      </div>
    );
  }
  if (!detail.data) {
    return (
      <EmptyState
        title="This item could not load"
        description={detail.error?.message ?? "Try loading this item again."}
        action={
          <Button
            onClick={() => {
              void detail.refetch();
            }}
          >
            Try again
          </Button>
        }
      />
    );
  }
  const { item } = detail.data;
  return (
    <>
      <Metadata
        title={item.title}
        description={item.description}
        image={item.artworkUrl}
        author={item.channel.title}
        durationSeconds={item.durationSeconds}
        kind="video.episode"
      />
      {item.kind === "podcast" ? (
        <PodcastPlayer key={mediaId} detail={detail.data} returnTo={returnTo} />
      ) : (
        <YouTubePlayer key={mediaId} detail={detail.data} returnTo={returnTo} />
      )}
    </>
  );
}
