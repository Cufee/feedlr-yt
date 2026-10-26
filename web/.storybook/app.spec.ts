import { expect, test } from "@playwright/test";

const screens: { story: string; path: string; title?: string; heading?: string }[] = [
  { story: "feed", path: "/app", title: "Your feed", heading: "New releases" },
  { story: "history", path: "/app/recent", title: "Recently played" },
  { story: "watch-later", path: "/app/watch-later", title: "Watch Later" },
  { story: "subscriptions", path: "/app/subscriptions", title: "Subscriptions" },
  { story: "channel", path: "/channel/field-notes", title: "Field Notes", heading: "Field Notes" },
  {
    story: "podcast-channel",
    path: "/channel/quiet-hours",
    title: "The Quiet Hours",
    heading: "The Quiet Hours",
  },
  { story: "playlists", path: "/app/playlists", title: "Playlists" },
  { story: "playlist", path: "/app/playlist/a-slower-sunday", title: "A slower Sunday" },
  { story: "settings", path: "/app/settings", title: "Settings" },
  { story: "account", path: "/app/settings/account", title: "Settings" },
  { story: "you-tube-settings", path: "/app/settings/youtube", title: "Settings" },
  { story: "podcast-settings", path: "/app/settings/podcasts", title: "Settings" },
  { story: "login", path: "/login", title: "Welcome", heading: "Welcome to Feedlr" },
  { story: "landing", path: "/", title: "YouTube and podcasts", heading: "YouTube and podcasts" },
  {
    story: "legal",
    path: "/legal/privacy-policy",
    title: "Privacy Policy",
    heading: "Your time, your choices",
  },
  { story: "empty-feed", path: "/app", title: "Your feed", heading: "No subscriptions" },
  {
    story: "empty-watch-later",
    path: "/app/watch-later",
    title: "Watch Later",
    heading: "No saved items",
  },
  { story: "empty-playlists", path: "/app/playlists", title: "Playlists", heading: "No playlists" },
  {
    story: "empty-subscriptions",
    path: "/app/subscriptions",
    title: "Subscriptions",
    heading: "No subscriptions",
  },
  { story: "error", path: "/app", heading: "Couldn’t load this page" },
  { story: "session-required", path: "/app/watch-later", heading: "Sign in to see your feed" },
];

for (const screen of screens) {
  test(`app ${screen.story} fits the viewport`, async ({ page }, testInfo) => {
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
    const story = `app-screens--${screen.story}-${viewport}`;
    await page.goto(`/iframe.html?id=${story}&viewMode=story`);
    const root = page.locator("#storybook-root");
    await expect(root.locator("[data-preview-location]")).toHaveAttribute(
      "data-preview-location",
      new RegExp(`^${screen.path}(?:\\?|$)`),
    );
    if (screen.title) {
      await expect(page).toHaveTitle(`${screen.title} · Feedlr`);
    }
    if (screen.heading) {
      await expect(root.getByRole("heading", { name: screen.heading, exact: true })).toBeVisible();
    }
    await expect(root.getByRole("status", { name: "Loading page" })).toHaveCount(0);
    await expect(root.locator(".ui-skeleton")).toHaveCount(0);
    if (screen.path.startsWith("/channel/")) {
      await expect(
        root.getByRole("heading", { level: 1, name: screen.title, exact: true }),
      ).toBeVisible();
    } else if (screen.title && screen.story !== "login" && screen.story !== "landing") {
      await expect(root.getByRole("heading", { level: 1 })).toHaveCount(0);
    }
    if (screen.story === "legal") {
      const width = await root.locator(".legal-prose").evaluate((article) => {
        const main = article.closest("main");
        if (!main) {
          throw new Error("The legal article is missing its main content container");
        }
        const style = getComputedStyle(main);
        return {
          article: article.getBoundingClientRect().width,
          available:
            main.clientWidth -
            Number.parseFloat(style.paddingLeft) -
            Number.parseFloat(style.paddingRight),
        };
      });
      expect(Math.abs(width.article - width.available)).toBeLessThan(1);
    }
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
      .toBe(true);
    await expect(root.getByRole("main")).toBeVisible();
    await expect(root.getByRole("banner")).toBeVisible();
    await page.evaluate(async () => {
      const images = Array.from(document.images);
      for (const image of images) {
        image.loading = "eager";
      }
      await Promise.all(images.map((image) => image.decode()));
    });
    await page.screenshot({
      path: testInfo.outputPath(`${story}.png`),
      fullPage: true,
      animations: "disabled",
    });
    expect(errors).toEqual([]);
  });
}

test("app loading preserves navigation while data waits", async ({ page }, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--loading-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  await expect(root.getByRole("status", { name: "Loading page" })).toBeVisible();
  await expect(root.getByRole("navigation", { name: "Primary" })).toBeVisible();
  await expect(root.locator(".ui-skeleton").first()).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath(`app-loading-${viewport}.png`),
    fullPage: true,
    animations: "disabled",
  });
});

test("app exposes one YouTube action in the banner", async ({ page }, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--feed-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  await expect(root.getByRole("heading", { name: "New releases", level: 2 })).toBeVisible();
  const open = root.getByRole("banner").getByRole("button", { name: "Open YouTube" });
  await expect(root.getByRole("button", { name: /Open (YouTube|a video)/ })).toHaveCount(1);
  await expect(
    root.getByRole("main").getByRole("button", { name: /Open (YouTube|a video)/ }),
  ).toHaveCount(0);
  await open.click();
  const dialog = page.getByRole("dialog", { name: "Open a video" });
  await expect(dialog.getByRole("textbox", { name: "YouTube link" })).toBeVisible();
  await dialog.getByRole("button", { name: "Close dialog" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(open).toBeFocused();
});

test("app adds subscriptions from the selected source and keeps the list first", async ({
  page,
}, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--subscriptions-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  await expect(page).toHaveTitle("Subscriptions · Feedlr");
  await expect(root.locator(".channel-tile")).toHaveCount(3);
  await expect(root.getByRole("searchbox", { name: "Channel name or handle" })).toHaveCount(0);
  await root.getByRole("link", { name: "Podcasts", exact: true }).click();
  await expect(root.locator(".channel-tile")).toHaveCount(1);
  const add = root.getByRole("button", { name: "Add subscription" });
  await add.click();
  const dialog = page.getByRole("dialog", { name: "Add subscription" });
  await expect(dialog.getByRole("tab", { name: "Podcasts" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await dialog.getByRole("searchbox", { name: "Podcast title or creator" }).fill("Off the Clock");
  await dialog.getByRole("button", { name: "Search" }).click();
  await dialog.getByRole("button", { name: "Follow Off the Clock" }).click();
  await expect(
    dialog.getByRole("button", { name: "Following Off the Clock. Unfollow" }),
  ).toBeVisible();
  await page.evaluate(async () => {
    const images = Array.from(document.images);
    for (const image of images) {
      image.loading = "eager";
    }
    await Promise.all(images.map((image) => image.decode()));
  });
  await page.screenshot({
    path: testInfo.outputPath(`app-add-subscription-${viewport}.png`),
    fullPage: true,
    animations: "disabled",
  });
  await dialog.getByRole("button", { name: "Close dialog" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(add).toBeFocused();
  await expect(root.getByRole("button", { name: "Following Off the Clock. Unfollow" })).toHaveCount(
    1,
  );
});

test("app navigation retains source, page, and memory history", async ({ page }) => {
  await page.goto("/iframe.html?id=app-screens--history-desktop&viewMode=story");
  const root = page.locator("#storybook-root");
  await expect(page).toHaveTitle("Recently played · Feedlr");
  await expect(root.locator(".media-grid")).toBeVisible();
  await root.getByRole("link", { name: "Podcasts", exact: true }).click();
  await expect(root.locator("[data-preview-location]")).toHaveAttribute(
    "data-preview-location",
    /source=podcasts/,
  );
  await expect(root.getByRole("link", { name: "Podcasts", exact: true })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(root.locator("article")).toHaveCount(2);
  await expect(
    root.getByRole("link", { name: "Listen to Making room for a more thoughtful day" }),
  ).toHaveAttribute("href", /return=%2Fapp%2Frecent%3Fsource%3Dpodcasts$/);
  await root
    .getByRole("navigation", { name: "Primary" })
    .getByRole("link", { name: /Saved|Watch Later/ })
    .click();
  await expect(page).toHaveTitle("Watch Later · Feedlr");
  await expect(root.getByRole("status", { name: "Loading page" })).toHaveCount(0);
  await expect(root.locator("[data-preview-location]")).toHaveAttribute(
    "data-preview-location",
    "/app/watch-later?page=1",
  );
});

test("app committed removal restores focus to the next saved card", async ({ page }) => {
  await page.goto("/iframe.html?id=app-screens--watch-later-desktop&viewMode=story");
  const root = page.locator("#storybook-root");
  const next = root.getByRole("link", { name: "Listen to Making room for a more thoughtful day" });
  await expect(next).toBeVisible();
  const first = root.locator("article").first();
  await first.getByRole("button", { name: "Remove from Watch Later" }).focus();
  await page.keyboard.press("Enter");
  await expect(root.locator("article")).toHaveCount(2);
  await expect(next).toBeFocused();
  await expect(page.locator("[data-sonner-toast]")).toHaveCount(0);
});

test("app editor submits a collection and navigates to it", async ({ page }) => {
  await page.goto("/iframe.html?id=app-screens--playlists-desktop&viewMode=story");
  const root = page.locator("#storybook-root");
  await expect(page).toHaveTitle("Playlists · Feedlr");
  await expect(root.getByRole("status", { name: "Loading page" })).toHaveCount(0);
  await root.getByRole("button", { name: "Create playlist", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Create a playlist" });
  await dialog.getByRole("textbox", { name: "Name" }).fill("A little perspective");
  await dialog
    .getByRole("textbox", { name: "Description (optional)" })
    .fill("Things for another day.");
  await dialog.getByRole("button", { name: "Create playlist", exact: true }).click();
  await expect(page).toHaveTitle("A little perspective · Feedlr");
  await expect(root.getByRole("heading", { name: "No items", exact: true })).toBeVisible();
  await expect(dialog).toHaveCount(0);
  await expect(root.locator("[data-preview-location]")).toHaveAttribute(
    "data-preview-location",
    "/app/playlist/new-collection",
  );
});

for (const state of [
  "sign-in",
  "create-account",
  "legacy-username",
  "choose-passkey",
  "completing",
  "canceled",
  "try-again",
  "unsupported",
  "username-error",
]) {
  test(`auth ${state} fits the viewport`, async ({ page }, testInfo) => {
    const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
    await page.goto(`/iframe.html?id=authentication-passkeys--${state}-${viewport}&viewMode=story`);
    await expect(page.getByRole("heading", { name: "Welcome to Feedlr", level: 1 })).toBeVisible();
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
      .toBe(true);
    await page.screenshot({
      path: testInfo.outputPath(`auth-${state}-${viewport}.png`),
      fullPage: true,
      animations: "disabled",
    });
  });
}

test("auth primary sends no username and account creation is a separate intent", async ({
  page,
}) => {
  await page.goto("/iframe.html?id=app-screens--login-desktop&viewMode=story");
  const root = page.locator("#storybook-root");
  const primary = root.getByRole("button", { name: "Sign in with a passkey" });
  await expect(primary).toBeEnabled();
  await expect(root.getByRole("textbox")).toHaveCount(0);
  const login = page.waitForRequest(
    (request) =>
      request.method() === "POST" && new URL(request.url()).pathname === "/api/v1/auth/login/begin",
  );
  await primary.click();
  expect((await login).postDataJSON()).toEqual({});
  await expect(root.getByRole("alert")).toContainText(
    "Account ceremonies are paused in this preview",
  );
  await expect(primary).not.toHaveAttribute("aria-busy", "true");
  await root.getByRole("button", { name: "Create an account" }).click();
  const username = root.getByRole("textbox", { name: "Username" });
  await expect(username).toBeFocused();
  await username.fill("quiet-reader");
  const registration = page.waitForRequest(
    (request) =>
      request.method() === "POST" &&
      new URL(request.url()).pathname === "/api/v1/auth/register/begin",
  );
  await root.getByRole("button", { name: "Create a passkey" }).click();
  expect((await registration).postDataJSON()).toEqual({ username: "quiet-reader" });
  await expect(root.getByRole("alert")).toContainText(
    "Account ceremonies are paused in this preview",
  );
});

test("auth legacy username fallback stays explicit and visibly supports autofill", async ({
  page,
}) => {
  await page.goto("/iframe.html?id=app-screens--login-mobile&viewMode=story");
  const root = page.locator("#storybook-root");
  await root.getByRole("button", { name: "Use a username instead" }).click();
  const username = root.getByRole("textbox", { name: "Username" });
  await expect(username).toBeVisible();
  await expect(username).toHaveAttribute("autocomplete", "username webauthn");
  await username.fill("older-reader");
  const login = page.waitForRequest(
    (request) =>
      request.method() === "POST" &&
      new URL(request.url()).pathname === "/api/v1/auth/login/begin" &&
      request.postData()?.includes('"username"') === true,
  );
  await root.getByRole("button", { name: "Continue with a passkey" }).click();
  expect((await login).postDataJSON()).toEqual({ username: "older-reader" });
  await expect(root.getByRole("alert")).toContainText(
    "Account ceremonies are paused in this preview",
  );
  await root.getByRole("button", { name: "Back to sign in" }).click();
  await expect(root.getByRole("textbox")).toHaveCount(0);
  await expect(root.getByRole("button", { name: "Sign in with a passkey" })).toBeVisible();
});
