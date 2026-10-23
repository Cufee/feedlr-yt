import { ArrowPathIcon, CheckIcon, PlusIcon, RadioIcon } from "@heroicons/react/24/outline";
import { Link } from "@tanstack/react-router";
import type { SVGProps } from "react";
import { Page, PageLoading, RequestError } from "@/app/Page";
import { MediaGrid } from "@/components/media/MediaCard";
import { Button, EmptyState } from "@/components/ui";
import { api, mediaPath } from "@/lib/api/client";
import { useAction, useChannel, useSession } from "@/lib/api/queries";

function ClapperboardIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M20.2 6 3 11l-.9-2.4c-.3-1.1.3-2.2 1.3-2.5l13.5-4c1.1-.3 2.2.3 2.5 1.3Z" />
      <path d="m6.2 5.3 3.1 3.9m3.1-5.8 3.1 4M3 11h18v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" />
    </svg>
  );
}

export function ChannelPage({ channelId }: { channelId: string }) {
  const { data, isPending, error, refetch } = useChannel(channelId);
  const session = useSession();
  const authenticated = !!session.data?.authenticated;
  const subscription = useAction(() =>
    data?.subscribed
      ? api.delete(`/subscriptions/${mediaPath(channelId)}`)
      : api.post("/subscriptions", { channelId }),
  );
  const refresh = useAction(() => api.post(`/channels/${mediaPath(channelId)}/refresh`), {
    success: "Channel refreshed",
  });
  const filter = useAction((value: string) =>
    api.patch(`/subscriptions/${mediaPath(channelId)}`, { filter: value }),
  );
  if (isPending) {
    return <PageLoading />;
  }
  if (error) {
    return <RequestError error={error} retry={() => void refetch()} />;
  }
  return (
    <Page
      title={data.channel.title}
      description={data.channel.description}
      image={data.channel.artworkUrl}
      avatar={data.channel.artworkUrl}
      action={
        authenticated ? (
          <>
            <Button pending={refresh.isPending} onClick={() => refresh.mutate()}>
              <ArrowPathIcon />
              Refresh
            </Button>
            <Button
              variant={data.subscribed ? "secondary" : "primary"}
              pending={subscription.isPending}
              onClick={() => subscription.mutate()}
            >
              {data.subscribed ? <CheckIcon /> : <PlusIcon />}
              {data.subscribed ? "Following" : "Follow"}
            </Button>
          </>
        ) : (
          <Link to="/login" className="ui-button ui-button-primary">
            Sign in to follow
          </Link>
        )
      }
    >
      {!data.channel.isPodcast && authenticated && data.subscribed && (
        <fieldset className="segmented-navigation" aria-label="Channel media filter">
          {[
            { value: "all", label: "All" },
            { value: "videos", label: "Videos", icon: ClapperboardIcon },
            { value: "streams", label: "Streams", icon: RadioIcon },
          ].map((choice) => (
            <Button
              key={choice.value}
              variant="ghost"
              className={data.filter === choice.value ? "is-active" : ""}
              aria-pressed={data.filter === choice.value}
              aria-disabled={filter.isPending}
              pending={filter.isPending && filter.variables === choice.value}
              onClick={() => {
                if (!filter.isPending && data.filter !== choice.value) {
                  filter.mutate(choice.value);
                }
              }}
            >
              {choice.icon && <choice.icon aria-hidden="true" />}
              {choice.label}
            </Button>
          ))}
        </fieldset>
      )}
      {filter.error && (
        <p className="form-error" role="alert">
          {filter.error.message}
        </p>
      )}
      {data.items.length ? (
        <MediaGrid
          items={data.items}
          returnTo={`/channel/${channelId}`}
          authenticated={authenticated}
        />
      ) : (
        <EmptyState
          title={data.channel.isPodcast ? "No episodes to show" : "No videos to show"}
          description="Try a different filter or check back later."
        />
      )}
    </Page>
  );
}
