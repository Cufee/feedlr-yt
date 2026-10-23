import { Link } from "@tanstack/react-router";
import { Page, PageLoading, RequestError, Section } from "@/app/Page";
import { MediaGrid } from "@/components/media/MediaCard";
import { EmptyState } from "@/components/ui";
import { useFeed } from "@/lib/api/queries";

export function FeedPage() {
  const { data, isPending, error, refetch } = useFeed();
  if (isPending) {
    return <PageLoading />;
  }
  if (error) {
    return <RequestError error={error} retry={() => void refetch()} />;
  }
  return (
    <Page
      title="Your feed"
      description="New videos and podcast episodes from your subscriptions."
      showDescription={false}
    >
      {data.onboarding ? (
        <EmptyState
          title="No subscriptions"
          description="Follow a YouTube channel or podcast to see new releases."
          action={
            <Link
              to="/app/subscriptions"
              search={{ source: "all" }}
              className="ui-button ui-button-primary"
            >
              Add subscriptions
            </Link>
          }
        />
      ) : (
        <>
          {data.watchLater.length > 0 && (
            <Section
              title="Saved for later"
              action={
                <Link className="ui-link" to="/app/watch-later" search={{ page: 1 }}>
                  View all
                </Link>
              }
            >
              <div className="media-shelf">
                <MediaGrid items={data.watchLater} returnTo="/app" />
              </div>
            </Section>
          )}
          <Section title="New releases">
            {data.new.length ? (
              <MediaGrid items={data.new} returnTo="/app" />
            ) : (
              <EmptyState
                title="You’re all caught up"
                description="New releases from your subscriptions will appear here."
              />
            )}
          </Section>
          {data.watched.length > 0 && (
            <Section
              title="Recently played"
              action={
                <Link className="ui-link" to="/app/recent" search={{ source: "all" }}>
                  View history
                </Link>
              }
            >
              <MediaGrid items={data.watched} returnTo="/app" />
            </Section>
          )}
        </>
      )}
    </Page>
  );
}
