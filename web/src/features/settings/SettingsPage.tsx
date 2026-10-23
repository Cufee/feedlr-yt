import {
  ArrowTopRightOnSquareIcon,
  FingerPrintIcon,
  PlusIcon,
  TrashIcon,
} from "@heroicons/react/24/outline";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { Page, PageLoading, RequestError } from "@/app/Page";
import { dateLabel } from "@/components/media/format";
import { Badge, Button, Field, Input, Switch } from "@/components/ui";
import { ConfirmDialog } from "@/components/ui-compositions";
import { api, mediaPath } from "@/lib/api/client";
import { useAction, useSettings } from "@/lib/api/queries";
import type { Settings } from "@/lib/api/types";
import { preserveRemovalFocus } from "@/lib/browser/removal-focus";
import { useAddPasskey } from "@/lib/browser/usePasskeys";
import { SkipSettings } from "./SkipSettings";

function addedDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? "Unknown date"
    : new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(date);
}

function AccountSettings({ settings }: { settings: Settings }) {
  const [removal, setRemoval] = useState<{ id: string; row: Element | null } | null>(null);
  const addPasskey = useAddPasskey();
  const add = useAction(addPasskey, { success: "Passkey added" });
  const remove = useAction(() => api.delete(`/passkeys/${mediaPath(removal?.id ?? "")}`), {
    success: "Passkey removed",
    onSuccess: () => setRemoval(null),
  });
  const logout = useAction(() => api.post("/logout"), {
    onSuccess: () => window.location.assign("/login"),
  });
  return (
    <>
      <section className="panel settings-section">
        <header className="settings-section-header">
          <div>
            <h2>Passkeys</h2>
          </div>
          <Button pending={add.isPending} onClick={() => add.mutate()}>
            <PlusIcon />
            Add passkey
          </Button>
        </header>
        <div className="settings-list">
          {settings.passkeys.map((passkey) => (
            <div className="settings-row settings-passkey-row" key={passkey.id}>
              <div className="settings-row-icon">
                <FingerPrintIcon />
              </div>
              <div className="settings-row-copy">
                <h3>{passkey.label || "Passkey"}</h3>
                <p className="muted">
                  Added <time dateTime={passkey.createdAt}>{addedDate(passkey.createdAt)}</time>
                </p>
              </div>
              <Button
                size="sm"
                variant="ghost"
                disabled={settings.passkeys.length <= 1}
                onClick={(event) =>
                  setRemoval({ id: passkey.id, row: event.currentTarget.closest(".settings-row") })
                }
              >
                <TrashIcon />
                Remove
              </Button>
            </div>
          ))}
        </div>
        {settings.passkeys.length <= 1 && (
          <p className="settings-note">Keep at least one passkey to sign in.</p>
        )}
      </section>
      <section className="panel settings-section">
        <div className="settings-section-header">
          <div>
            <h2>Session</h2>
          </div>
          <Button pending={logout.isPending} onClick={() => logout.mutate()}>
            Sign out
          </Button>
        </div>
      </section>
      {removal && (
        <ConfirmDialog
          open
          onClose={() => setRemoval(null)}
          title="Remove this passkey?"
          description="You will no longer be able to use this passkey to sign in."
          pending={remove.isPending}
          error={remove.error}
          onConfirm={() => {
            const stop = preserveRemovalFocus(removal.row);
            void remove.mutateAsync().catch(stop);
          }}
        />
      )}
    </>
  );
}

function YouTubeSettings({ settings }: { settings: Settings }) {
  const [code, setCode] = useState("");
  const sync = settings.youtubeSync;
  const tv = settings.tvSync;
  const connect = useAction(() => api.post<{ redirectUrl: string }>("/settings/youtube/connect"), {
    onSuccess: ({ redirectUrl }) => window.location.assign(redirectUrl),
  });
  const toggle = useAction((enabled: boolean) => api.put("/settings/youtube", { enabled }));
  const disconnect = useAction(() => api.post("/settings/youtube/disconnect"), {
    success: "YouTube disconnected",
  });
  const connectTV = useAction(() => api.post("/settings/tv/connect", { code }), {
    success: "TV paired",
    onSuccess: () => setCode(""),
  });
  const toggleTV = useAction((enabled: boolean) => api.put("/settings/tv", { enabled }));
  const disconnectTV = useAction(() => api.delete("/settings/tv"), { success: "TV disconnected" });
  return (
    <>
      <section className="panel settings-section">
        <header className="settings-section-header">
          <div className="settings-heading-group">
            <div className="settings-title">
              <h2>YouTube sync</h2>
              <Badge tone={sync.connected ? "success" : "neutral"}>
                {sync.connected ? "Connected" : "Not connected"}
              </Badge>
            </div>
          </div>
        </header>
        {!sync.available ? (
          <p className="settings-note">
            YouTube account connections are unavailable on this instance. Your Feedlr subscriptions
            still work.
          </p>
        ) : sync.connected ? (
          <>
            <Switch
              label="Sync playlists and Watch Later"
              checked={sync.enabled}
              disabled={toggle.isPending}
              onChange={(enabled) => toggle.mutate(enabled)}
            />
            <p className="settings-note">Last synced {dateLabel(sync.lastSyncedAt)}</p>
            <Button pending={disconnect.isPending} onClick={() => disconnect.mutate()}>
              Disconnect YouTube
            </Button>
          </>
        ) : (
          <Button variant="primary" pending={connect.isPending} onClick={() => connect.mutate()}>
            <ArrowTopRightOnSquareIcon />
            Connect YouTube
          </Button>
        )}
        {sync.lastError && (
          <p className="form-error" role="alert">
            {sync.lastError}
          </p>
        )}
      </section>
      <section className="panel settings-section">
        <header className="settings-section-header">
          <div className="settings-title">
            <h2>TV</h2>
            <Badge tone={tv.connected ? "success" : "neutral"}>
              {tv.connected ? "Paired" : "Not paired"}
            </Badge>
          </div>
        </header>
        {!tv.available ? (
          <p className="settings-note">TV pairing is unavailable on this instance.</p>
        ) : tv.connected ? (
          <>
            <div className="settings-row">
              <div className="grow">
                <h3>{tv.screenName || "Paired TV"}</h3>
                <p className="muted">
                  {tv.connectionState === "online" || tv.connectionState === "connected"
                    ? "Ready to watch"
                    : "Paired · waiting for your TV"}
                </p>
              </div>
            </div>
            <Switch
              label="Keep TV connected"
              checked={tv.enabled}
              disabled={toggleTV.isPending}
              onChange={(enabled) => toggleTV.mutate(enabled)}
            />
            <Button pending={disconnectTV.isPending} onClick={() => disconnectTV.mutate()}>
              Disconnect TV
            </Button>
          </>
        ) : (
          <form
            className="search-form"
            onSubmit={(event) => {
              event.preventDefault();
              connectTV.mutate();
            }}
          >
            <Field
              label="TV code"
              htmlFor="tv-code"
              description="In YouTube on your TV, open Settings → Link with TV code."
              error={connectTV.error?.message}
            >
              <Input
                id="tv-code"
                value={code}
                onChange={(event) => setCode(event.target.value)}
                inputMode="numeric"
                placeholder="Enter your TV code"
                required
              />
            </Field>
            <Button type="submit" pending={connectTV.isPending}>
              Pair TV
            </Button>
          </form>
        )}
        {tv.lastError && (
          <p className="form-error" role="alert">
            {tv.lastError}
          </p>
        )}
      </section>
      <SkipSettings provider="sponsorBlock" settings={settings.sponsorBlock} />
    </>
  );
}

export function SettingsPage({ section = "account" }: { section?: string }) {
  const { data, isPending, error, refetch } = useSettings();
  const active = ["account", "youtube", "podcasts"].includes(section) ? section : "account";
  return (
    <Page
      title="Settings"
      description="Account, YouTube and podcast preferences."
      showDescription={false}
    >
      <nav className="segmented-navigation" aria-label="Settings sections">
        {[
          { id: "account", label: "Account" },
          { id: "youtube", label: "YouTube" },
          { id: "podcasts", label: "Podcasts" },
        ].map((item) => (
          <Link
            key={item.id}
            to="/app/settings/$section"
            params={{ section: item.id }}
            resetScroll={false}
            className={active === item.id ? "is-active" : ""}
            aria-current={active === item.id ? "page" : undefined}
          >
            {item.label}
          </Link>
        ))}
      </nav>
      {isPending ? (
        <PageLoading />
      ) : error ? (
        <RequestError error={error} retry={() => void refetch()} />
      ) : active === "account" ? (
        <AccountSettings settings={data} />
      ) : active === "youtube" ? (
        <YouTubeSettings settings={data} />
      ) : (
        <SkipSettings provider="podcastSegments" settings={data.podcastSegments} />
      )}
    </Page>
  );
}
