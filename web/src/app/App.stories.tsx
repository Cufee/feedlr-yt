import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, RouterProvider, useRouterState } from "@tanstack/react-router";
import { StrictMode, useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { Button } from "@/components/ui";
import { NotificationProvider } from "./Notifications";
import { createPreviewAPI, type PreviewOptions } from "./preview-fixtures";
import { createAppRouter } from "./router";
import "./app.css";

interface PreviewProps {
  entry: string;
  historyTools?: boolean;
}
function AppPreview({ entry, historyTools = false }: PreviewProps) {
  const [runtime] = useState(() => ({
    client: new QueryClient({
      defaultOptions: {
        queries: { retry: false, staleTime: 20_000, gcTime: 0 },
        mutations: { retry: false },
      },
    }),
    router: createAppRouter(createMemoryHistory({ initialEntries: [entry] })),
  }));
  const location = useRouterState({
    router: runtime.router,
    select: (state) => state.location.href,
  });
  return (
    <StrictMode>
      <QueryClientProvider client={runtime.client}>
        <NotificationProvider>
          <div data-preview-location={location}>
            {historyTools && (
              <nav aria-label="Preview history" className="flex items-center gap-3 p-4">
                <Button onClick={() => runtime.router.history.back()}>Back in preview</Button>
                <Button onClick={() => runtime.router.history.forward()}>Forward in preview</Button>
                <output aria-label="Preview route" className="ui-help">
                  {location}
                </output>
              </nav>
            )}
            <RouterProvider router={runtime.router} />
          </div>
        </NotificationProvider>
      </QueryClientProvider>
    </StrictMode>
  );
}

const meta = {
  title: "App/Screens",
  component: AppPreview,
  parameters: { layout: "fullscreen", controls: { disable: true } },
  args: { entry: "/app" },
} satisfies Meta<typeof AppPreview>;
export default meta;
type Story = StoryObj<typeof meta>;
type Viewport = "desktop" | "phone";

function screen(
  entry: string,
  heading: string | RegExp,
  viewport: Viewport,
  options: PreviewOptions = {},
): Story {
  const api = createPreviewAPI(options);
  return {
    args: { entry },
    globals: { viewport: { value: viewport } },
    parameters: { msw: { handlers: api.handlers } },
    beforeEach: () => api.reset(),
    render: (args) => (
      <AppPreview key={`${entry}:${viewport}:${options.mode ?? "ready"}`} {...args} />
    ),
    play: async ({ canvasElement }) => {
      const canvas = within(canvasElement);
      if (options.mode === "loading") {
        await expect(canvas.findByRole("status", { name: "Loading page" })).resolves.toBeVisible();
      } else {
        await expect(canvas.findByRole("heading", { name: heading })).resolves.toBeVisible();
        await waitFor(() =>
          expect(canvas.queryByRole("status", { name: "Loading page" })).not.toBeInTheDocument(),
        );
      }
    },
  };
}

export const FeedDesktop = screen("/app", "Your feed", "desktop");
export const FeedMobile = screen("/app", "Your feed", "phone");
export const HistoryDesktop = screen("/app/recent?source=all&page=1", "Recently played", "desktop");
export const HistoryMobile = screen("/app/recent?source=all&page=1", "Recently played", "phone");
export const HistoryScrollPreservation = screen(
  "/app/recent?source=all&page=1",
  "Recently played",
  "desktop",
  { longHistory: true, holdSourceResponses: true },
);
export const WatchLaterDesktop = screen("/app/watch-later?page=1", "Watch Later", "desktop");
export const WatchLaterMobile = screen("/app/watch-later?page=1", "Watch Later", "phone");
export const SubscriptionsDesktop = screen(
  "/app/subscriptions?source=all",
  "Subscriptions",
  "desktop",
);
export const SubscriptionsMobile = screen(
  "/app/subscriptions?source=all",
  "Subscriptions",
  "phone",
);
export const ChannelDesktop = screen("/channel/field-notes", "Field Notes", "desktop");
export const ChannelMobile = screen("/channel/field-notes", "Field Notes", "phone");
export const PodcastChannelDesktop = screen("/channel/quiet-hours", "The Quiet Hours", "desktop");
export const PodcastChannelMobile = screen("/channel/quiet-hours", "The Quiet Hours", "phone");
export const PlaylistsDesktop = screen("/app/playlists", "Playlists", "desktop");
export const PlaylistsMobile = screen("/app/playlists", "Playlists", "phone");
export const PlaylistDesktop = screen(
  "/app/playlist/a-slower-sunday",
  "A slower Sunday",
  "desktop",
);
export const PlaylistMobile = screen("/app/playlist/a-slower-sunday", "A slower Sunday", "phone");
export const SettingsDesktop = screen("/app/settings", "Settings", "desktop");
export const SettingsMobile = screen("/app/settings", "Settings", "phone");
export const ChannelFilterFailure = screen("/channel/field-notes", "Field Notes", "desktop", {
  mode: "save-error",
});
export const AccountDesktop = screen("/app/settings/account", "Settings", "desktop");
export const AccountMobile = screen("/app/settings/account", "Settings", "phone");
export const YouTubeSettingsDesktop = screen("/app/settings/youtube", "Settings", "desktop");
export const YouTubeSettingsMobile = screen("/app/settings/youtube", "Settings", "phone");
export const PodcastSettingsDesktop = screen("/app/settings/podcasts", "Settings", "desktop");
export const PodcastSettingsMobile = screen("/app/settings/podcasts", "Settings", "phone");
export const LoginDesktop = screen(
  "/login?return=%2Fapp%2Fwatch-later%3Fpage%3D2",
  "Welcome to Feedlr",
  "desktop",
  { guest: true },
);
export const LoginMobile = screen(
  "/login?return=%2Fapp%2Fwatch-later%3Fpage%3D2",
  "Welcome to Feedlr",
  "phone",
  { guest: true },
);
export const LandingDesktop = screen("/", /YouTube\s*and podcasts/, "desktop", {
  guest: true,
});
export const LandingMobile = screen("/", /YouTube\s*and podcasts/, "phone", {
  guest: true,
});
export const LegalDesktop = screen("/legal/privacy-policy", "Privacy Policy", "desktop", {
  guest: true,
});
export const LegalMobile = screen("/legal/privacy-policy", "Privacy Policy", "phone", {
  guest: true,
});
export const LoadingDesktop = screen("/app", "", "desktop", { mode: "loading" });
export const LoadingMobile = screen("/app", "", "phone", { mode: "loading" });
export const ErrorDesktop = screen("/app", "Couldn’t load this page", "desktop", { mode: "error" });
export const ErrorMobile = screen("/app", "Couldn’t load this page", "phone", { mode: "error" });
export const EmptyFeedDesktop = screen("/app", "Your feed", "desktop", { mode: "empty" });
export const EmptyFeedMobile = screen("/app", "Your feed", "phone", { mode: "empty" });
export const EmptyWatchLaterDesktop = screen("/app/watch-later?page=1", "Watch Later", "desktop", {
  mode: "empty",
});
export const EmptyWatchLaterMobile = screen("/app/watch-later?page=1", "Watch Later", "phone", {
  mode: "empty",
});
export const EmptyPlaylistsDesktop = screen("/app/playlists", "Playlists", "desktop", {
  mode: "empty",
});
export const EmptyPlaylistsMobile = screen("/app/playlists", "Playlists", "phone", {
  mode: "empty",
});
export const EmptySubscriptionsDesktop = screen(
  "/app/subscriptions?source=all",
  "Subscriptions",
  "desktop",
  { mode: "empty" },
);
export const EmptySubscriptionsMobile = screen(
  "/app/subscriptions?source=all",
  "Subscriptions",
  "phone",
  { mode: "empty" },
);
export const SessionRequiredDesktop = screen(
  "/app/watch-later?page=2",
  "Sign in to see your feed",
  "desktop",
  { guest: true },
);
export const SessionRequiredMobile = screen(
  "/app/watch-later?page=2",
  "Sign in to see your feed",
  "phone",
  { guest: true },
);

export const NavigationAndHistory: Story = {
  ...screen("/app/recent?source=all&page=1", "Recently played", "desktop", {
    longHistory: true,
    sourceDelayMs: 800,
  }),
  args: { entry: "/app/recent?source=all&page=1", historyTools: true },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByRole("link", { name: "Next" });
    await userEvent.click(canvas.getByRole("link", { name: "Next" }));
    await waitFor(() =>
      expect(canvas.getByRole("status", { name: "Preview route" })).toHaveTextContent("page=2"),
    );
    await expect(canvas.getByText("Page 2", { exact: true })).toBeVisible();
    await userEvent.click(canvas.getByRole("link", { name: "Podcasts" }));
    await waitFor(() => {
      const location = new URL(
        canvas.getByRole("status", { name: "Preview route" }).textContent ?? "/",
        "https://preview.test",
      );
      expect(location.searchParams.get("source")).toBe("podcasts");
      expect(location.searchParams.get("page")).toBe("1");
    });
    await expect(canvas.getByRole("link", { name: "Podcasts" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    await userEvent.click(canvas.getByRole("button", { name: "Back in preview" }));
    await waitFor(() => {
      const location = new URL(
        canvas.getByRole("status", { name: "Preview route" }).textContent ?? "/",
        "https://preview.test",
      );
      expect(location.searchParams.get("source")).toBe("all");
      expect(location.searchParams.get("page")).toBe("2");
    });
    await userEvent.click(canvas.getByRole("button", { name: "Forward in preview" }));
    await waitFor(() => {
      const location = new URL(
        canvas.getByRole("status", { name: "Preview route" }).textContent ?? "/",
        "https://preview.test",
      );
      expect(location.searchParams.get("source")).toBe("podcasts");
      expect(location.searchParams.get("page")).toBe("1");
    });
    await userEvent.click(
      within(canvas.getByRole("navigation", { name: "Primary" })).getByRole("link", {
        name: "Watch Later",
      }),
    );
    await expect(
      canvas.findByRole("heading", { name: "Watch Later", level: 1 }),
    ).resolves.toBeVisible();
    await expect(canvas.getByRole("status", { name: "Preview route" })).toHaveTextContent(
      "/app/watch-later?page=1",
    );
  },
};

export const WatchLaterRemovalFocus: Story = {
  ...screen("/app/watch-later?page=1", "Watch Later", "desktop"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const next = await canvas.findByRole("link", {
      name: "Listen to Making room for a more thoughtful day",
    });
    const first = canvas
      .getByRole("link", { name: "Watch A small cabin, a long weekend, and no plans" })
      .closest("article");
    if (!first) {
      throw new Error("The first saved card is missing");
    }
    await userEvent.click(within(first).getByRole("button", { name: "Remove from Watch Later" }));
    await waitFor(() => expect(first).not.toBeInTheDocument());
    await waitFor(() => expect(next).toHaveFocus());
    await expect(canvas.getByText("Removed from Watch Later", { exact: true })).toBeVisible();
  },
};

export const WatchLaterRollback: Story = {
  ...screen("/app", "Your feed", "phone", { mode: "save-error" }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const artwork = await canvas.findByRole("link", {
      name: "Watch Inside a ceramic studio by the sea",
    });
    const card = artwork.closest("article");
    if (!card) {
      throw new Error("The unsaved card is missing");
    }
    const save = within(card).getByRole("button", { name: "Save to Watch Later" });
    await userEvent.click(save);
    await waitFor(() => expect(save).toHaveAttribute("aria-pressed", "true"));
    await waitFor(() => expect(save).toHaveAttribute("aria-pressed", "false"));
    await expect(canvas.getByRole("alert")).toHaveTextContent(
      "Your previous choice has been restored.",
    );
    await expect(save).toHaveFocus();
  },
};

export const PlaylistRemovalFocus: Story = {
  ...screen("/app/playlist/a-slower-sunday", "A slower Sunday", "desktop"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const next = await canvas.findByRole("link", {
      name: "Listen to Making room for a more thoughtful day",
    });
    const remove = canvas.getByRole("button", {
      name: "Remove A small cabin, a long weekend, and no plans from playlist",
    });
    await userEvent.click(remove);
    await waitFor(() => expect(remove).not.toBeInTheDocument());
    await waitFor(() => expect(next).toHaveFocus());
  },
};

export const SubscriptionRemovalFocus: Story = {
  ...screen("/app/subscriptions?source=all", "Subscriptions", "phone"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const remove = await canvas.findByRole("button", { name: "Following Field Notes. Unfollow" });
    const next = canvas.getByRole("link", { name: "Form & Field" });
    await userEvent.click(remove);
    await waitFor(() => expect(remove).not.toBeInTheDocument());
    await waitFor(() => expect(next).toHaveFocus());
    await expect(canvas.getByText("Subscription removed", { exact: true })).toBeVisible();
  },
};

export const CreatePlaylist: Story = {
  ...screen("/app/playlists", "Playlists", "desktop"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByRole("heading", { name: "Playlists", level: 1 });
    await userEvent.click(canvas.getByRole("button", { name: "Create playlist" }));
    const document = within(canvasElement.ownerDocument.body);
    const dialog = within(await document.findByRole("dialog", { name: "Create a playlist" }));
    await userEvent.type(dialog.getByRole("textbox", { name: "Name" }), "Things to come back to");
    await userEvent.type(
      dialog.getByRole("textbox", { name: "Description (optional)" }),
      "A few good ideas for another day.",
    );
    await userEvent.click(dialog.getByRole("button", { name: "Create playlist" }));
    await expect(
      canvas.findByRole("heading", { name: "Things to come back to", level: 1 }),
    ).resolves.toBeVisible();
    await expect(document.queryByRole("dialog")).not.toBeInTheDocument();
  },
};

export const PlaylistMembership: Story = {
  ...screen("/app/watch-later?page=1", "Watch Later", "phone"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const actions = await canvas.findByRole("button", {
      name: "Actions for A small cabin, a long weekend, and no plans",
    });
    await userEvent.click(actions);
    const document = within(canvasElement.ownerDocument.body);
    await userEvent.click(document.getByRole("menuitem", { name: "Add to playlist" }));
    const dialog = within(await document.findByRole("dialog", { name: "Add to playlist" }));
    const membership = await dialog.findByRole("checkbox", { name: "People who make" });
    await expect(membership).not.toBeChecked();
    await userEvent.click(membership);
    await waitFor(() => expect(membership).toBeChecked());
    await waitFor(() => expect(membership).not.toBeDisabled());
    await userEvent.click(dialog.getByRole("button", { name: "Done" }));
    await expect(document.queryByRole("dialog")).not.toBeInTheDocument();
  },
};

export const PodcastDiscovery: Story = {
  ...screen("/app/subscriptions?source=all", "Subscriptions", "desktop"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Add subscription" }));
    const document = within(canvasElement.ownerDocument.body);
    const dialog = within(await document.findByRole("dialog", { name: "Add subscription" }));
    await userEvent.click(dialog.getByRole("tab", { name: "Podcasts" }));
    await userEvent.type(
      dialog.getByRole("searchbox", { name: "Podcast title or creator" }),
      "Off the Clock",
    );
    await userEvent.click(dialog.getByRole("button", { name: "Search" }));
    const follow = await dialog.findByRole("button", { name: "Follow Off the Clock" });
    await userEvent.click(follow);
    await waitFor(() =>
      expect(
        dialog.getByRole("button", { name: "Following Off the Clock. Unfollow" }),
      ).toBeVisible(),
    );
    await userEvent.click(dialog.getByRole("button", { name: "Close dialog" }));
    await expect(document.queryByRole("dialog")).not.toBeInTheDocument();
    await expect(
      canvas.getAllByRole("button", { name: "Following Off the Clock. Unfollow" }),
    ).toHaveLength(1);
    await userEvent.click(canvas.getByRole("link", { name: "Off the Clock" }));
    await expect(
      canvas.findByRole("heading", { name: "Off the Clock", level: 1 }),
    ).resolves.toBeVisible();
    await expect(canvas.getByRole("button", { name: "Following" })).toBeVisible();
  },
};

function subscriptionDialog(viewport: Viewport): Story {
  return {
    ...screen("/app/subscriptions?source=podcasts", "Subscriptions", viewport),
    play: async ({ canvasElement }) => {
      const canvas = within(canvasElement);
      await userEvent.click(await canvas.findByRole("button", { name: "Add subscription" }));
      const dialog = within(
        await within(canvasElement.ownerDocument.body).findByRole("dialog", {
          name: "Add subscription",
        }),
      );
      await expect(dialog.getByRole("tab", { name: "Podcasts" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
      await waitFor(() =>
        expect(dialog.getByRole("searchbox", { name: "Podcast title or creator" })).toBeVisible(),
      );
      await waitFor(() =>
        expect(dialog.getByRole("textbox", { name: "Podcast RSS feed" })).toBeVisible(),
      );
    },
  };
}
export const AddSubscriptionDialogDesktop = subscriptionDialog("desktop");
export const AddSubscriptionDialogMobile = subscriptionDialog("phone");

export const SaveSkippingPreferences: Story = {
  ...screen("/app/settings/podcasts", "Settings", "phone"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const sponsors = await canvas.findByRole("switch", { name: "Sponsors" });
    const main = canvas.getByRole("switch", { name: "Enable Smart skipping" });
    await userEvent.click(sponsors);
    await waitFor(() => expect(sponsors).not.toBeChecked());
    await waitFor(() => expect(sponsors).not.toBeDisabled());
    await userEvent.click(main);
    await waitFor(() => expect(main).not.toBeChecked());
    await waitFor(() => expect(main).not.toBeDisabled());
    await expect(sponsors).toBeDisabled();
    await expect(canvas.getByText("Paused", { exact: true })).toBeVisible();
  },
};

export const RetryRead: Story = {
  ...screen("/app", "Couldn’t load this page", "desktop", { failFirstRead: true }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Try again" }));
    await expect(
      canvas.findByRole("heading", { name: "Your feed", level: 1 }),
    ).resolves.toBeVisible();
  },
};

export const OpenVideoDialog: Story = {
  ...screen("/app", "Your feed", "phone"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByRole("heading", { name: "Your feed", level: 1 });
    const open = canvas.getByRole("button", { name: "Open YouTube" });
    await expect(open.closest(".app-header")).not.toBeNull();
    await expect(canvas.getAllByRole("button", { name: "Open YouTube" })).toHaveLength(1);
    await expect(
      within(canvas.getByRole("main")).queryByRole("button", { name: "Open YouTube" }),
    ).not.toBeInTheDocument();
    await userEvent.click(open);
    const document = within(canvasElement.ownerDocument.body);
    const dialog = within(await document.findByRole("dialog", { name: "Open a video" }));
    await userEvent.type(
      dialog.getByRole("textbox", { name: "YouTube link" }),
      "https://example.com/a-video",
    );
    await userEvent.click(dialog.getByRole("button", { name: "Open video" }));
    await expect(dialog.getByRole("alert")).toHaveTextContent(
      "Enter a YouTube video link or video ID.",
    );
  },
};

export const CreatePlaylistDialog: Story = {
  ...screen("/app/playlists", "Playlists", "phone"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByRole("heading", { name: "Playlists", level: 1 });
    await userEvent.click(canvas.getByRole("button", { name: "Create playlist" }));
    await expect(
      within(canvasElement.ownerDocument.body).findByRole("dialog", { name: "Create a playlist" }),
    ).resolves.toBeVisible();
  },
};

export const ImportPlaylistDialog: Story = {
  ...screen("/app/playlists", "Playlists", "desktop"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByRole("heading", { name: "Playlists", level: 1 });
    await userEvent.click(canvas.getByRole("button", { name: "Import" }));
    await expect(
      within(canvasElement.ownerDocument.body).findByRole("dialog", {
        name: "Import a YouTube playlist",
      }),
    ).resolves.toBeVisible();
  },
};

export const EditPlaylistDialog: Story = {
  ...screen("/app/playlist/a-slower-sunday", "A slower Sunday", "desktop"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Edit" }));
    const dialog = within(
      await within(canvasElement.ownerDocument.body).findByRole("dialog", {
        name: "Edit playlist",
      }),
    );
    await expect(dialog.getByRole("textbox", { name: "Name" })).toHaveValue("A slower Sunday");
  },
};

export const AddVideoDialog: Story = {
  ...screen("/app/playlist/a-slower-sunday", "A slower Sunday", "phone"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Add a video" }));
    await expect(
      within(canvasElement.ownerDocument.body).findByRole("dialog", { name: "Add a video" }),
    ).resolves.toBeVisible();
  },
};

export const DeletePlaylistDialog: Story = {
  ...screen("/app/playlist/a-slower-sunday", "A slower Sunday", "desktop"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Delete playlist" }));
    await expect(
      within(canvasElement.ownerDocument.body).findByRole("dialog", {
        name: "Delete “A slower Sunday”?",
      }),
    ).resolves.toBeVisible();
  },
};

export const RemovePasskeyDialog: Story = {
  ...screen("/app/settings/account", "Settings", "phone"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const buttons = await canvas.findAllByRole("button", { name: "Remove" });
    const remove = buttons[0];
    if (!remove) {
      throw new Error("The passkey removal action is missing");
    }
    await userEvent.click(remove);
    await expect(
      within(canvasElement.ownerDocument.body).findByRole("dialog", {
        name: "Remove this passkey?",
      }),
    ).resolves.toBeVisible();
  },
};

export const MarkUnwatched: Story = {
  ...screen("/app/recent?source=all&page=1", "Recently played", "desktop"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const actions = await canvas.findByRole("button", {
      name: "Actions for The forest after the rain",
    });
    const card = actions.closest("article");
    if (!card) {
      throw new Error("The completed card is missing");
    }
    await expect(within(card).getByText("Watched", { exact: true })).toBeVisible();
    await userEvent.click(actions);
    await userEvent.click(
      within(canvasElement.ownerDocument.body).getByRole("menuitem", { name: "Mark as unwatched" }),
    );
    await waitFor(() =>
      expect(within(card).queryByText("Watched", { exact: true })).not.toBeInTheDocument(),
    );
    await userEvent.click(actions);
    await waitFor(() =>
      expect(
        within(canvasElement.ownerDocument.body).getByRole("menuitem", { name: "Mark as watched" }),
      ).toBeVisible(),
    );
    await userEvent.keyboard("{Escape}");
  },
};

export const RemovePasskey: Story = {
  ...screen("/app/settings/account", "Settings", "desktop"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const remove = (await canvas.findAllByRole("button", { name: "Remove" }))[0];
    if (!remove) {
      throw new Error("The first passkey action is missing");
    }
    await userEvent.click(remove);
    const dialog = within(
      await within(canvasElement.ownerDocument.body).findByRole("dialog", {
        name: "Remove this passkey?",
      }),
    );
    await userEvent.click(dialog.getByRole("button", { name: "Delete" }));
    await waitFor(() =>
      expect(canvas.queryByText("This laptop", { exact: true })).not.toBeInTheDocument(),
    );
    await expect(canvas.getByText("Personal phone", { exact: true })).toBeVisible();
    await expect(canvas.getByRole("button", { name: "Remove" })).toBeDisabled();
    await waitFor(() =>
      expect(canvas.getByRole("heading", { name: "Settings", level: 1 })).toHaveFocus(),
    );
  },
};
