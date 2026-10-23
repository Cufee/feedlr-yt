import { Link } from "@tanstack/react-router";
import { Page, PageLoading, Pagination, RequestError } from "@/app/Page";
import { MediaGrid } from "@/components/media/MediaCard";
import { EmptyState } from "@/components/ui";
import { useWatchLaterPage } from "@/lib/api/queries";

export function WatchLaterPage({ page }: { page: number }) {
  const { data, isPending, error, refetch } = useWatchLaterPage(page);
  return (
    <Page
      title="Watch Later"
      description="Your saved videos and podcast episodes."
      showDescription={false}
    >
      {isPending ? (
        <PageLoading />
      ) : error ? (
        <RequestError error={error} retry={() => void refetch()} />
      ) : data.items.length ? (
        <MediaGrid items={data.items} returnTo={`/app/watch-later?page=${page}`} />
      ) : (
        <EmptyState
          title={page > 1 ? "End of Watch Later" : "No saved items"}
          description="Use the bookmark on a video or episode to keep it here."
          action={
            <Link className="ui-link" to="/app">
              Browse your feed
            </Link>
          }
        />
      )}
      <Pagination page={page} hasMore={!!data?.page.hasMore} to="/app/watch-later" />
    </Page>
  );
}
