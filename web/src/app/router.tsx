import {
  ArrowTopRightOnSquareIcon,
  BookmarkIcon,
  ClockIcon,
  Cog6ToothIcon,
  HomeIcon,
  QueueListIcon,
  RectangleStackIcon,
} from "@heroicons/react/24/outline";
import {
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  Outlet,
  type RouterHistory,
  useRouterState,
} from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { OpenMediaDialog } from "@/components/media/OpenMediaDialog";
import { Button, EmptyState } from "@/components/ui";
import { LoginPage } from "@/features/auth/LoginPage";
import { safeReturn } from "@/features/auth/passkeys";
import { FeedPage } from "@/features/feed/FeedPage";
import { HistoryPage } from "@/features/feed/HistoryPage";
import { WatchLaterPage } from "@/features/feed/WatchLaterPage";
import { PlayerPage } from "@/features/playback/PlayerPage";
import { PlaylistPage } from "@/features/playlists/PlaylistPage";
import { PlaylistsPage } from "@/features/playlists/PlaylistsPage";
import { ErrorPage, LandingPage, LegalPage, NotFoundPage } from "@/features/public/PublicPages";
import { SettingsPage } from "@/features/settings/SettingsPage";
import { ChannelPage } from "@/features/subscriptions/ChannelPage";
import { SubscriptionsPage } from "@/features/subscriptions/SubscriptionsPage";
import { useSession } from "@/lib/api/queries";
import type { MediaSource } from "@/lib/api/types";
import { useOpenShortcut } from "@/lib/browser/useOpenShortcut";
import { PageLoading, RequestError } from "./Page";

const navigation = [
  { to: "/app", label: "Your feed", mobileLabel: "Feed", icon: HomeIcon },
  { to: "/app/watch-later", label: "Watch Later", mobileLabel: "Saved", icon: BookmarkIcon },
  { to: "/app/playlists", label: "Playlists", mobileLabel: "Playlists", icon: QueueListIcon },
  {
    to: "/app/subscriptions",
    label: "Subscriptions",
    mobileLabel: "Following",
    icon: RectangleStackIcon,
  },
  { to: "/app/recent", label: "Recently played", mobileLabel: "History", icon: ClockIcon },
] as const;

function RootLayout() {
  const session = useSession();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const navigating = useRouterState({ select: (state) => state.isLoading });
  const [open, setOpen] = useState(false);
  useOpenShortcut(() => setOpen(true));
  const player = pathname.startsWith("/video/");
  const authenticated = !!session.data?.authenticated;
  const active = (to: string) =>
    to === "/app"
      ? pathname === to
      : to === "/app/playlists"
        ? pathname.startsWith("/app/playlist")
        : pathname.startsWith(to);
  return (
    <div
      className={`app-frame ${authenticated && !player ? "app-frame-with-sidebar" : ""} ${player ? "app-frame-player" : ""}`}
    >
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      {navigating && (
        <div className="navigation-progress" role="progressbar" aria-label="Loading page" />
      )}
      {!player && (
        <>
          <header className="app-header glass">
            <Link className="brand" to={authenticated ? "/app" : "/"} aria-label="Feedlr home">
              <span className="brand-mark" aria-hidden="true">
                <RectangleStackIcon />
              </span>
              <span>
                feedlr<span className="brand-dot">.</span>
              </span>
            </Link>
            <div className="app-header-actions">
              {authenticated ? (
                <>
                  <Button variant="ghost" onClick={() => setOpen(true)}>
                    <ArrowTopRightOnSquareIcon />
                    <span className="open-video-label">Open YouTube</span>
                    <span className="header-shortcut" aria-hidden="true">
                      ⌘ K
                    </span>
                  </Button>
                  <Link
                    to="/app/settings/$section"
                    params={{ section: "account" }}
                    className="profile-link"
                    aria-label="Account settings"
                  >
                    {session.data?.username?.slice(0, 1).toUpperCase() || "F"}
                  </Link>
                </>
              ) : (
                <Link to="/login" className="ui-button ui-button-primary">
                  Sign in
                </Link>
              )}
            </div>
          </header>
          {authenticated && (
            <aside className="app-sidebar glass">
              <div>
                <nav aria-label="Primary">
                  {navigation.map((item) => (
                    <Link
                      key={item.to}
                      to={item.to}
                      search={
                        item.to === "/app/watch-later"
                          ? { page: 1 }
                          : item.to === "/app/subscriptions" || item.to === "/app/recent"
                            ? { source: "all" }
                            : {}
                      }
                      aria-current={active(item.to) ? "page" : undefined}
                      aria-label={item.label}
                      className={`navigation-link ${active(item.to) ? "is-active" : ""}`}
                    >
                      <item.icon />
                      <span className="desktop-nav-label">{item.label}</span>
                    </Link>
                  ))}
                </nav>
              </div>
              <div>
                <Link
                  to="/app/settings/$section"
                  params={{ section: "account" }}
                  className={`navigation-link ${pathname.startsWith("/app/settings") ? "is-active" : ""}`}
                  aria-current={pathname.startsWith("/app/settings") ? "page" : undefined}
                >
                  <Cog6ToothIcon />
                  <span>Settings</span>
                </Link>
              </div>
            </aside>
          )}
        </>
      )}
      <main id="main-content" className="app-main">
        <Outlet />
      </main>
      {!player && (
        <footer className="app-footer">
          <nav aria-label="Legal">
            <Link to="/legal/$document" params={{ document: "privacy-policy" }}>
              Privacy
            </Link>
            <Link to="/legal/$document" params={{ document: "terms-of-service" }}>
              Terms
            </Link>
          </nav>
        </footer>
      )}
      {open && <OpenMediaDialog open onClose={() => setOpen(false)} />}
    </div>
  );
}

function RequireSession({ children }: { children: ReactNode }) {
  const session = useSession();
  const returnTo = useRouterState({ select: (state) => state.location.href });
  if (session.isPending) {
    return <PageLoading />;
  }
  if (session.error) {
    return <RequestError error={session.error} retry={() => void session.refetch()} />;
  }
  if (!session.data.authenticated) {
    return (
      <EmptyState
        title="Sign in to see your feed"
        action={
          <Link to="/login" search={{ return: returnTo }} className="ui-button ui-button-primary">
            Sign in
          </Link>
        }
      />
    );
  }
  return children;
}
const sourceSearch = (search: Record<string, unknown>): { source: MediaSource } => ({
  source: search.source === "youtube" || search.source === "podcasts" ? search.source : "all",
});
const pageSearch = (value: unknown) => {
  const page = Number(value);
  return Number.isFinite(page) ? Math.max(1, Math.min(41_667, Math.trunc(page))) : 1;
};
const root = createRootRoute({
  component: RootLayout,
  notFoundComponent: NotFoundPage,
  errorComponent: ({ error, reset }) => (
    <RequestError
      error={error instanceof Error ? error : new Error("This page could not load.")}
      retry={reset}
    />
  ),
});
const landing = createRoute({ getParentRoute: () => root, path: "/", component: LandingPage });
const login = createRoute({
  getParentRoute: () => root,
  path: "/login",
  validateSearch: (search: Record<string, unknown>) => ({ return: safeReturn(search.return) }),
  component: function LoginRoute() {
    return <LoginPage returnTo={login.useSearch().return} />;
  },
});
const feed = createRoute({
  getParentRoute: () => root,
  path: "/app",
  component: () => (
    <RequireSession>
      <FeedPage />
    </RequireSession>
  ),
});
const watchLater = createRoute({
  getParentRoute: () => root,
  path: "/app/watch-later",
  validateSearch: (search: Record<string, unknown>) => ({
    page: pageSearch(search.page),
  }),
  component: function WatchLaterRoute() {
    return (
      <RequireSession>
        <WatchLaterPage page={watchLater.useSearch().page} />
      </RequireSession>
    );
  },
});
const history = createRoute({
  getParentRoute: () => root,
  path: "/app/recent",
  validateSearch: (search: Record<string, unknown>) => ({
    ...sourceSearch(search),
    page: pageSearch(search.page),
  }),
  component: function HistoryRoute() {
    return (
      <RequireSession>
        <HistoryPage source={history.useSearch().source} page={history.useSearch().page} />
      </RequireSession>
    );
  },
});
const subscriptions = createRoute({
  getParentRoute: () => root,
  path: "/app/subscriptions",
  validateSearch: sourceSearch,
  component: function SubscriptionsRoute() {
    return (
      <RequireSession>
        <SubscriptionsPage source={subscriptions.useSearch().source} />
      </RequireSession>
    );
  },
});
const onboarding = createRoute({
  getParentRoute: () => root,
  path: "/app/onboarding",
  component: () => (
    <RequireSession>
      <SubscriptionsPage onboarding />
    </RequireSession>
  ),
});
const playlists = createRoute({
  getParentRoute: () => root,
  path: "/app/playlists",
  component: () => (
    <RequireSession>
      <PlaylistsPage />
    </RequireSession>
  ),
});
const playlist = createRoute({
  getParentRoute: () => root,
  path: "/app/playlist/$playlistId",
  component: function PlaylistRoute() {
    return (
      <RequireSession>
        <PlaylistPage playlistId={playlist.useParams().playlistId} />
      </RequireSession>
    );
  },
});
const settingsIndex = createRoute({
  getParentRoute: () => root,
  path: "/app/settings",
  component: () => (
    <RequireSession>
      <SettingsPage />
    </RequireSession>
  ),
});
const settings = createRoute({
  getParentRoute: () => root,
  path: "/app/settings/$section",
  component: function SettingsRoute() {
    return (
      <RequireSession>
        <SettingsPage section={settings.useParams().section} />
      </RequireSession>
    );
  },
});
const channel = createRoute({
  getParentRoute: () => root,
  path: "/channel/$channelId",
  component: function ChannelRoute() {
    return <ChannelPage channelId={channel.useParams().channelId} />;
  },
});
const video = createRoute({
  getParentRoute: () => root,
  path: "/video/$mediaId",
  validateSearch: (search: Record<string, unknown>) => ({ return: safeReturn(search.return) }),
  component: function VideoRoute() {
    return <PlayerPage mediaId={video.useParams().mediaId} returnTo={video.useSearch().return} />;
  },
});
const legal = createRoute({
  getParentRoute: () => root,
  path: "/legal/$document",
  component: function LegalRoute() {
    return <LegalPage document={legal.useParams().document} />;
  },
});
const error = createRoute({ getParentRoute: () => root, path: "/error", component: ErrorPage });
const rateLimited = createRoute({
  getParentRoute: () => root,
  path: "/429",
  component: () => <ErrorPage limited />,
});

const routeTree = root.addChildren([
  landing,
  login,
  feed,
  watchLater,
  history,
  subscriptions,
  onboarding,
  playlists,
  playlist,
  settingsIndex,
  settings,
  channel,
  video,
  legal,
  error,
  rateLimited,
]);
export const createAppRouter = (history?: RouterHistory) =>
  createRouter({
    routeTree,
    history,
    defaultPreload: "intent",
    scrollRestoration: true,
  });
export const router = createAppRouter();
