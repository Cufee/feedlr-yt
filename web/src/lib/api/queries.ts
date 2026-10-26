import {
  keepPreviousData,
  QueryClient,
  queryOptions,
  useIsMutating,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { toast } from "sonner";
import { ApiError, api, mediaPath } from "./client";
import type {
  Channel,
  ChannelDetail,
  Feed,
  MediaDetail,
  MediaItem,
  MediaSource,
  MediaState,
  PageResult,
  Playlist,
  PlaylistDetail,
  Session,
  Settings,
} from "./types";

export const keys = {
  all: ["feedlr"] as const,
  session: ["feedlr", "session"] as const,
  feed: ["feedlr", "feed"] as const,
  history: (source: MediaSource) => ["feedlr", "history", source] as const,
  watchLater: (page: number) => ["feedlr", "watch-later", page] as const,
  media: (id: string) => ["feedlr", "media", id] as const,
  subscriptions: (source: MediaSource) => ["feedlr", "subscriptions", source] as const,
  channel: (id: string) => ["feedlr", "channel", id] as const,
  playlists: ["feedlr", "playlists"] as const,
  playlist: (id: string) => ["feedlr", "playlist", id] as const,
  settings: ["feedlr", "settings"] as const,
};
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 20_000,
      retry: (attempt, error) => !(error instanceof ApiError && error.status < 500) && attempt < 1,
    },
    mutations: { retry: false },
  },
});
export const sessionOptions = () =>
  queryOptions({
    queryKey: keys.session,
    queryFn: ({ signal }) => api.get<Session>("/session", signal),
    staleTime: 60_000,
  });
export const useSession = () => useQuery(sessionOptions());
export const useFeed = () =>
  useQuery({ queryKey: keys.feed, queryFn: ({ signal }) => api.get<Feed>("/feed", signal) });
export const useHistory = (source: MediaSource) =>
  useQuery({
    queryKey: keys.history(source),
    placeholderData: keepPreviousData,
    queryFn: ({ signal }) =>
      api.get<PageResult>(`/history?source=${source}&limit=24&offset=0`, signal),
  });
export const useWatchLaterPage = (page: number) =>
  useQuery({
    queryKey: keys.watchLater(page),
    queryFn: ({ signal }) =>
      api.get<PageResult>(`/watch-later?limit=24&offset=${(page - 1) * 24}`, signal),
  });
export const useMediaDetail = (id: string) =>
  useQuery({
    queryKey: keys.media(id),
    queryFn: ({ signal }) => api.get<MediaDetail>(`/media/${mediaPath(id)}`, signal),
  });
export const useSubscriptions = (source: MediaSource) =>
  useQuery({
    queryKey: keys.subscriptions(source),
    placeholderData: keepPreviousData,
    queryFn: ({ signal }) =>
      api.get<{ items: Channel[] }>(`/subscriptions?source=${source}`, signal),
  });
export const useChannel = (id: string) =>
  useQuery({
    queryKey: keys.channel(id),
    queryFn: ({ signal }) => api.get<ChannelDetail>(`/channels/${mediaPath(id)}`, signal),
  });
export const usePlaylists = () =>
  useQuery({
    queryKey: keys.playlists,
    queryFn: ({ signal }) => api.get<{ items: Playlist[] }>("/playlists", signal),
  });
export const usePlaylist = (id: string) =>
  useQuery({
    queryKey: keys.playlist(id),
    queryFn: ({ signal }) => api.get<PlaylistDetail>(`/playlists/${mediaPath(id)}`, signal),
  });
export const useSettings = () =>
  useQuery({
    queryKey: keys.settings,
    queryFn: ({ signal }) => api.get<Settings>("/settings", signal),
  });

export function useAction<TVariables = void, TResult = void>(
  mutationFn: (variables: TVariables) => Promise<TResult>,
  options: { success?: string; onSuccess?: (result: TResult, variables: TVariables) => void } = {},
) {
  const client = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: async (result, variables) => {
      if (options.success) {
        toast.success(options.success);
      }
      options.onSuccess?.(result, variables);
      await client.invalidateQueries({ queryKey: keys.all });
    },
    onError: (error) => {
      if (error.name !== "AbortError") {
        toast.error(error.message, { duration: 10_000 });
      }
    },
  });
}

// Patch only media records; unrelated caches and simultaneous actions keep their state.
export function patchMedia(value: unknown, id: string, patch: Partial<MediaItem>): unknown {
  if (Array.isArray(value)) {
    const next = value.map((item) => patchMedia(item, id, patch));
    return next.some((item, index) => item !== value[index]) ? next : value;
  }
  if (value === null || typeof value !== "object") {
    return value;
  }
  const record = value as Record<string, unknown>;
  if (record.id === id && typeof record.inWatchLater === "boolean") {
    return { ...record, ...patch };
  }
  let next = record;
  for (const key of ["new", "watched", "watchLater", "items", "item"]) {
    if (!(key in record)) {
      continue;
    }
    const updated = patchMedia(record[key], id, patch);
    if (updated !== record[key]) {
      next = { ...next, [key]: updated };
    }
  }
  return next;
}

function findMembership(value: unknown, id: string): boolean | undefined {
  if (Array.isArray(value)) {
    return value.map((item) => findMembership(item, id)).find((item) => item !== undefined);
  }
  if (!value || typeof value !== "object") {
    return undefined;
  }
  const record = value as Record<string, unknown>;
  if (record.id === id && typeof record.inWatchLater === "boolean") {
    return record.inWatchLater;
  }
  for (const key of ["new", "watched", "watchLater", "items", "item"]) {
    const result = findMembership(record[key], id);
    if (result !== undefined) {
      return result;
    }
  }
}

export function useWatchLater(id: string) {
  const client = useQueryClient();
  const mutationKey = ["watch-later-membership", id];
  const active = useIsMutating({ mutationKey });
  const { mutateAsync, error } = useMutation({
    mutationKey,
    scope: { id: `membership:${id}` },
    mutationFn: (present: boolean) => api.put(`/watch-later/${mediaPath(id)}`, { present }),
    onMutate: async (present) => {
      await client.cancelQueries({ queryKey: keys.all });
      const previous = client
        .getQueriesData({ queryKey: keys.all })
        .map(([key, value]) => ({ key, present: findMembership(value, id) }));
      client.setQueriesData({ queryKey: keys.all }, (value) =>
        patchMedia(value, id, { inWatchLater: present }),
      );
      return previous;
    },
    onError: (failure, _present, previous) => {
      for (const item of previous ?? []) {
        if (item.present !== undefined) {
          const present = item.present;
          client.setQueryData(item.key, (value: unknown) =>
            patchMedia(value, id, { inWatchLater: present }),
          );
        }
      }
      toast.error(failure.message, { duration: 10_000 });
    },
    onSettled: () => client.invalidateQueries({ queryKey: keys.all }),
  });
  function setPresent(present: boolean) {
    if (client.isMutating({ mutationKey })) {
      return Promise.resolve();
    }
    return mutateAsync(present);
  }
  return { setPresent, pending: active > 0, error };
}

export function useMediaState(id: string) {
  return useAction((state: MediaState) => api.patch(`/media/${mediaPath(id)}/state`, state));
}
