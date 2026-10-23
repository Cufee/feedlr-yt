import { Page, PageLoading, Pagination, RequestError, SourceNavigation } from "@/app/Page";
import { MediaGrid } from "@/components/media/MediaCard";
import { EmptyState } from "@/components/ui";
import { useHistory } from "@/lib/api/queries";
import type { MediaSource } from "@/lib/api/types";

export function HistoryPage({ source, page }: { source: MediaSource; page: number }) {
  const { data, isPending, isPlaceholderData, error, refetch } = useHistory(source, page);
  return (
    <Page
      title="Recently played"
      description="Your recently played videos and podcast episodes."
      showDescription={false}
    >
      <SourceNavigation source={source} to="/app/recent" />
      {isPending ? (
        <PageLoading />
      ) : error ? (
        <RequestError error={error} retry={() => void refetch()} />
      ) : data.items.length ? (
        <MediaGrid
          items={data.items}
          returnTo={`/app/recent?source=${source}&page=${page}`}
          busy={isPlaceholderData}
        />
      ) : (
        <EmptyState
          title="No history"
          description="Videos and episodes you play will appear here."
        />
      )}
      <Pagination page={page} hasMore={!!data?.page.hasMore} to="/app/recent" source={source} />
    </Page>
  );
}
