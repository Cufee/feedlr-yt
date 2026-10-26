import { CheckIcon, MagnifyingGlassIcon, PlusIcon, RssIcon } from "@heroicons/react/24/outline";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";
import { Page, PageLoading, RequestError, SourceNavigation } from "@/app/Page";
import { Button, Dialog, EmptyState, Field, Input, Tabs } from "@/components/ui";
import { api, mediaPath } from "@/lib/api/client";
import { keys, useAction, useSession, useSubscriptions } from "@/lib/api/queries";
import type { Channel, MediaSource } from "@/lib/api/types";
import { preserveRemovalFocus } from "@/lib/browser/removal-focus";

export function ChannelTile({
  channel,
  discovery = false,
}: {
  channel: Channel;
  discovery?: boolean;
}) {
  const navigate = useNavigate();
  const open = useAction(
    () => api.post<{ channelId: string }>("/podcasts/open", { feedUrl: channel.feedUrl }),
    {
      onSuccess: ({ channelId }) => {
        void navigate({ to: "/channel/$channelId", params: { channelId } });
      },
    },
  );
  const subscribe = useAction(() =>
    channel.subscribed
      ? api.delete(`/subscriptions/${mediaPath(channel.id)}`)
      : api.post(
          "/subscriptions",
          channel.isPodcast && channel.feedUrl
            ? { feedUrl: channel.feedUrl }
            : { channelId: channel.id },
        ),
  );
  return (
    <article className="channel-tile panel">
      <img className="channel-avatar" src={channel.artworkUrl} alt="" loading="lazy" />
      <div className="channel-tile-copy">
        <h2>
          {discovery && channel.isPodcast && !channel.subscribed ? (
            <button
              className="channel-title-action channel-tile-target"
              type="button"
              aria-busy={open.isPending}
              disabled={open.isPending}
              onClick={() => open.mutate()}
            >
              {channel.title}
            </button>
          ) : (
            <Link
              className="channel-tile-target"
              to="/channel/$channelId"
              params={{ channelId: channel.id }}
            >
              {channel.title}
            </Link>
          )}
        </h2>
        {discovery && channel.description && <p className="line-clamp-2">{channel.description}</p>}
      </div>
      <Button
        size="sm"
        variant={channel.subscribed ? "secondary" : "primary"}
        pending={subscribe.isPending}
        onClick={(event) => {
          const stop =
            channel.subscribed && !discovery
              ? preserveRemovalFocus(event.currentTarget.closest(".channel-tile"))
              : () => undefined;
          void subscribe.mutateAsync().catch(stop);
        }}
        aria-label={
          channel.subscribed ? `Following ${channel.title}. Unfollow` : `Follow ${channel.title}`
        }
        aria-pressed={channel.subscribed}
      >
        {channel.subscribed ? <CheckIcon /> : <PlusIcon />}
        <span className="channel-follow-label">{channel.subscribed ? "Following" : "Follow"}</span>
      </Button>
    </article>
  );
}

function DiscoverySearch({ source }: { source: "youtube" | "podcasts" }) {
  const [input, setInput] = useState("");
  const [query, setQuery] = useState("");
  const { data, isFetching, error } = useQuery({
    queryKey: [...keys.all, "search", source, query],
    queryFn: ({ signal }) =>
      api.get<{ items: Channel[] }>(
        `/search?source=${source}&q=${encodeURIComponent(query)}`,
        signal,
      ),
    enabled: query.length >= 3,
    staleTime: 60_000,
  });
  function search(event: FormEvent) {
    event.preventDefault();
    setQuery(input.trim());
  }
  return (
    <div className="section-stack">
      <form className="search-form" onSubmit={search}>
        <Field
          label={source === "youtube" ? "Channel name or handle" : "Podcast title or creator"}
          htmlFor={`discovery-${source}`}
        >
          <Input
            id={`discovery-${source}`}
            type="search"
            value={input}
            onChange={(event) => setInput(event.target.value)}
            minLength={3}
            maxLength={100}
            required
          />
        </Field>
        <Button type="submit" pending={isFetching}>
          <MagnifyingGlassIcon />
          Search
        </Button>
      </form>
      {error && (
        <p className="form-error" role="alert">
          {error.message}
        </p>
      )}
      {isFetching ? (
        <p role="status" className="muted">
          Finding {source === "youtube" ? "channels" : "podcasts"}…
        </p>
      ) : data?.items.length ? (
        <div className="channel-grid">
          {data.items.map((channel) => (
            <ChannelTile key={channel.id || channel.feedUrl} channel={channel} discovery />
          ))}
        </div>
      ) : query && data ? (
        <EmptyState
          title="No matches yet"
          description="Try a different name or use a direct RSS feed below."
        />
      ) : null}
    </div>
  );
}

function RSSSubscription() {
  const [feedUrl, setFeedUrl] = useState("");
  const subscribe = useAction(() => api.post("/subscriptions", { feedUrl }), {
    onSuccess: () => setFeedUrl(""),
  });
  return (
    <form
      className="search-form"
      onSubmit={(event) => {
        event.preventDefault();
        subscribe.mutate();
      }}
    >
      <Field label="Podcast RSS feed" htmlFor="podcast-rss">
        <Input
          id="podcast-rss"
          type="url"
          value={feedUrl}
          onChange={(event) => setFeedUrl(event.target.value)}
          placeholder="https://example.com/feed.xml"
          required
        />
      </Field>
      <Button type="submit" pending={subscribe.isPending}>
        <RssIcon />
        Add feed
      </Button>
    </form>
  );
}

function AddSubscriptionDialog({
  source,
  podcastSearch,
  onClose,
}: {
  source: MediaSource;
  podcastSearch: boolean;
  onClose: () => void;
}) {
  const [discovery, setDiscovery] = useState(source === "podcasts" && podcastSearch ? 1 : 0);
  const tabs = [
    { id: "youtube", label: "YouTube channels", content: <DiscoverySearch source="youtube" /> },
    ...(podcastSearch
      ? [{ id: "podcasts", label: "Podcasts", content: <DiscoverySearch source="podcasts" /> }]
      : []),
  ];
  return (
    <Dialog open onClose={onClose} title="Add subscription">
      <Tabs items={tabs} selectedIndex={discovery} onChange={setDiscovery} label="Search source" />
      <div className="divider" />
      <RSSSubscription />
    </Dialog>
  );
}

export function SubscriptionsPage({
  source = "all",
  onboarding = false,
}: {
  source?: MediaSource;
  onboarding?: boolean;
}) {
  const { data, isPending, isPlaceholderData, error, refetch } = useSubscriptions(source);
  const session = useSession();
  const [filter, setFilter] = useState("");
  const [adding, setAdding] = useState(onboarding);
  const channels =
    data?.items.filter((channel) =>
      channel.title.toLocaleLowerCase().includes(filter.trim().toLocaleLowerCase()),
    ) ?? [];
  return (
    <Page
      title="Subscriptions"
      description="Manage your YouTube and podcast subscriptions."
      showDescription={false}
      action={
        <Button variant="primary" onClick={() => setAdding(true)}>
          <PlusIcon />
          Add subscription
        </Button>
      }
    >
      <SourceNavigation source={source} to="/app/subscriptions" />
      {(data?.items.length ?? 0) > 6 && (
        <Field label="Filter subscriptions" htmlFor="subscription-filter">
          <Input
            id="subscription-filter"
            type="search"
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder="Find a subscription"
          />
        </Field>
      )}
      {isPending ? (
        <PageLoading />
      ) : error ? (
        <RequestError error={error} retry={() => void refetch()} />
      ) : channels.length ? (
        <div className="channel-grid" aria-busy={isPlaceholderData}>
          {channels.map((channel) => (
            <ChannelTile key={channel.id} channel={channel} />
          ))}
        </div>
      ) : (
        <EmptyState
          title={filter ? "No matching subscriptions" : "No subscriptions"}
          description={
            filter
              ? "Try a different search."
              : "Add a YouTube channel or podcast to start your feed."
          }
        />
      )}
      {adding && (
        <AddSubscriptionDialog
          source={source}
          podcastSearch={!!session.data?.capabilities.podcastSearch}
          onClose={() => setAdding(false)}
        />
      )}
    </Page>
  );
}
