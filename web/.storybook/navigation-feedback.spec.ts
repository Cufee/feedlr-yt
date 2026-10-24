import { expect, type Page, test } from "@playwright/test";

async function toastFixture(page: Page, viewport: string) {
  await page.clock.install({ time: new Date("2026-10-22T11:00:00Z") });
  await page.goto(`/iframe.html?id=app-screens--imported-playlist-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  await expect(root.getByRole("heading", { name: "The long way home", level: 1 })).toBeVisible();
  await page.clock.pauseAt(new Date("2026-10-22T12:00:00Z"));
  await root.getByRole("button", { name: "Playlist actions" }).click();
  await page.getByRole("menuitem", { name: "Refresh import" }).click();
  const toast = page.locator('[data-sonner-toast][data-type="success"]');
  await expect
    .poll(async () => {
      // Let the fixture mutation and Query notifications finish without allowing
      // wall-clock time to consume the notification's lifetime between commands.
      await page.clock.runFor(100);
      return toast.allTextContents();
    })
    .toEqual([expect.stringContaining("Playlist refreshed")]);
  await page.mouse.move(0, 0);
  return { root, toast };
}

test("notifications use the library layout and dismiss after six visible seconds", async ({
  page,
}, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  const { toast } = await toastFixture(page, viewport);
  const bounds = await toast.boundingBox();
  const toaster = page.locator("[data-sonner-toaster]");
  await expect(toaster).toHaveAttribute("data-x-position", "right");
  await expect(toaster).toHaveAttribute("data-y-position", "bottom");
  expect(bounds).not.toBeNull();
  if (bounds) {
    const width = page.viewportSize()?.width ?? 0;
    if (info.project.name === "phone") {
      expect(bounds.x).toBeCloseTo(16, 0);
      expect(bounds.width).toBeCloseTo(width - 32, 0);
    } else {
      expect(bounds.x).toBeGreaterThan(width / 2);
      expect(width - bounds.x - bounds.width).toBeGreaterThan(0);
      expect(width - bounds.x - bounds.width).toBeLessThanOrEqual(32);
    }
  }
  await expect(toast.locator("[data-icon] svg")).toBeVisible();
  await page.clock.fastForward(5_000);
  await expect(toast).toBeVisible();
  await page.clock.fastForward(2_000);
  await page.clock.runFor(250);
  await expect(toast).toHaveCount(0);
});

test("notification lifetime pauses while hovered", async ({ page }, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  const { toast } = await toastFixture(page, viewport);
  await toast.hover();
  await page.clock.fastForward(12_000);
  await expect(toast).toBeVisible();
  await page.mouse.move(0, 0);
  await page.clock.fastForward(6_100);
  await page.clock.runFor(250);
  await expect(toast).toHaveCount(0);
});

test("notification lifetime pauses while the page is hidden", async ({ page }, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  const { toast } = await toastFixture(page, viewport);
  await page.evaluate(() => {
    Object.defineProperty(document, "hidden", { configurable: true, value: true });
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await page.clock.fastForward(12_000);
  await expect(toast).toBeVisible();
  await page.evaluate(() => {
    Reflect.deleteProperty(document, "hidden");
    document.dispatchEvent(new Event("visibilitychange"));
  });
  await page.clock.fastForward(6_100);
  await page.clock.runFor(250);
  await expect(toast).toHaveCount(0);
});

test("notifications can be focused with Alt+T and dismissed using the keyboard", async ({
  page,
}, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  const { toast } = await toastFixture(page, viewport);
  await page.keyboard.press("Alt+t");
  await expect(page.locator("[data-sonner-toaster]")).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(toast).toBeFocused();
  await page.keyboard.press("Tab");
  const close = toast.getByRole("button", { name: "Dismiss notification" });
  await expect(close).toBeFocused();
  await close.press("Enter");
  await page.clock.runFor(250);
  await expect(toast).toHaveCount(0);
});

test("failed actions still notify and keep errors visible for ten seconds", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-10-22T11:00:00Z") });
  await page.goto("/iframe.html?id=app-screens--channel-filter-failure&viewMode=story");
  const root = page.locator("#storybook-root");
  const streams = root.getByRole("group", { name: "Channel media filter" }).getByRole("button", {
    name: "Streams",
    exact: true,
  });
  await expect(streams).toBeVisible();
  await page.clock.pauseAt(new Date("2026-10-22T12:00:00Z"));
  await streams.click();
  const toast = page.locator('[data-sonner-toast][data-type="error"]');
  await expect
    .poll(async () => {
      await page.clock.runFor(100);
      return toast.allTextContents();
    })
    .toEqual([expect.stringContaining("The channel filter could not be saved")]);
  await page.mouse.move(0, 0);
  await page.clock.fastForward(9_000);
  await expect(toast).toBeVisible();
  await page.clock.fastForward(2_000);
  await page.clock.runFor(250);
  await expect(toast).toHaveCount(0);
});

test("source changes keep the list and scroll while the next response waits", async ({ page }) => {
  await page.goto("/iframe.html?id=app-screens--history-scroll-preservation&viewMode=story");
  const root = page.locator("#storybook-root");
  const source = root.getByRole("navigation", { name: "Media sources" });
  const all = source.getByRole("link", { name: "Everything" });
  await all.press("Enter");
  await expect(all).toHaveAttribute("aria-current", "page");
  await expect(root.locator("article").first()).toBeVisible();
  const count = await root.locator("article").count();
  const podcasts = source.getByRole("link", { name: "Podcasts" });
  await page.evaluate(() => window.scrollTo(0, 100));
  await podcasts.focus();
  const before = await page.evaluate(() => window.scrollY);
  expect(before).toBeGreaterThan(40);
  await podcasts.press("Enter");
  await expect(podcasts).toHaveAttribute("aria-current", "page");
  await expect(root.locator(".media-grid")).toHaveAttribute("aria-busy", "true");
  await expect(root.locator("article")).toHaveCount(count);
  await expect(root.getByRole("status", { name: "Loading page" })).toHaveCount(0);
  expect(Math.abs((await page.evaluate(() => window.scrollY)) - before)).toBeLessThan(2);
  expect(
    await page.evaluate(async () => {
      const response = await fetch("/__preview/release-source-responses", { method: "POST" });
      return response.status;
    }),
  ).toBe(204);
  await expect(root.locator(".media-grid")).toHaveAttribute("aria-busy", "false");
  await expect.poll(() => root.locator("article").count()).toBeLessThan(count);
  expect(Math.abs((await page.evaluate(() => window.scrollY)) - before)).toBeLessThan(2);
});

test("settings statuses and passkey removal stay aligned", async ({ page }, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--account-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  const row = root.locator(".settings-passkey-row").first();
  const remove = await row.getByRole("button", { name: "Remove" }).boundingBox();
  const copy = await row.locator(".settings-row-copy").boundingBox();
  expect(remove).not.toBeNull();
  expect(copy).not.toBeNull();
  if (remove && copy) {
    expect(remove.x).toBeGreaterThanOrEqual(copy.x + copy.width - 1);
    expect(Math.abs(remove.y + remove.height / 2 - (copy.y + copy.height / 2))).toBeLessThan(2);
  }
  await expect(row.locator("time")).not.toContainText(/AM|PM/);
  const youtube = root
    .getByRole("navigation", { name: "Settings sections" })
    .getByRole("link", { name: "YouTube" });
  await page.evaluate(() => window.scrollTo(0, 60));
  await youtube.focus();
  const before = await page.evaluate(() => window.scrollY);
  await youtube.press("Enter");
  await expect(youtube).toHaveAttribute("aria-current", "page");
  expect(Math.abs((await page.evaluate(() => window.scrollY)) - before)).toBeLessThan(2);
  const title = root
    .locator(".settings-title")
    .filter({ has: page.getByRole("heading", { name: "YouTube sync", exact: true }) });
  const heading = await title.getByRole("heading").boundingBox();
  const badge = await title.getByText("Connected", { exact: true }).boundingBox();
  expect(heading).not.toBeNull();
  expect(badge).not.toBeNull();
  if (heading && badge) {
    expect(Math.abs(heading.y + heading.height / 2 - (badge.y + badge.height / 2))).toBeLessThan(2);
  }
});

test("navigation stays anchored and phone navigation remains icon-only", async ({ page }, info) => {
  await page.goto("/iframe.html?id=app-screens--history-scroll-preservation&viewMode=story");
  const root = page.locator("#storybook-root");
  await expect(root.getByRole("heading", { name: "Recently played", level: 1 })).toBeVisible();
  const header = root.getByRole("banner");
  const start = await header.boundingBox();
  await page.evaluate(() => window.scrollTo(0, 500));
  expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(100);
  const after = await header.boundingBox();
  expect(start?.y).toBe(after?.y);
  expect(await header.evaluate((element) => getComputedStyle(element).position)).toBe("fixed");
  await expect(header.getByRole("button", { name: "Open YouTube", exact: true })).toHaveCount(1);
  const navigation = root.getByRole("navigation", { name: "Primary" });
  await expect(navigation.getByRole("link", { name: "Subscriptions", exact: true })).toBeVisible();
  if (info.project.name === "phone") {
    const links = navigation.getByRole("link");
    const widths = await links.evaluateAll((items) =>
      items.map((item) => item.getBoundingClientRect().width),
    );
    expect(Math.max(...widths) - Math.min(...widths)).toBeLessThan(1);
    for (const link of await links.all()) {
      // biome-ignore lint/performance/useDomNodeTextContent: This regression checks visible labels, excluding names hidden by CSS.
      expect((await link.innerText()).trim()).toBe("");
      await expect(link).toHaveAccessibleName(/\w+/);
    }
    const choices = root.getByRole("navigation", { name: "Media sources" });
    const sizes = await choices.getByRole("link").evaluateAll((items) =>
      items.map((item) => {
        const bounds = item.getBoundingClientRect();
        return { width: bounds.width, height: bounds.height };
      }),
    );
    expect(
      Math.max(...sizes.map((size) => size.width)) - Math.min(...sizes.map((size) => size.width)),
    ).toBeLessThan(1);
    expect(sizes.every((size) => size.height >= 44)).toBe(true);
  }
});

test("channel media filters use compact All Videos Streams pills", async ({ page }, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--channel-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  const filters = root.getByRole("group", { name: "Channel media filter" });
  await expect(filters).toBeVisible();
  await expect(filters.getByRole("button")).toHaveCount(3);
  await expect(filters.getByRole("button", { name: "All", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(root.getByRole("combobox")).toHaveCount(0);
  await expect(root.getByRole("heading", { name: "Releases" })).toHaveCount(0);
  const videos = filters.getByRole("button", { name: "Videos", exact: true });
  const streams = filters.getByRole("button", { name: "Streams", exact: true });
  await expect(videos.locator("svg")).toHaveCount(1);
  await expect(streams.locator("svg")).toHaveCount(1);
  await page.evaluate(() => window.scrollTo(0, 80));
  await videos.focus();
  const before = await page.evaluate(() => window.scrollY);
  await videos.press("Enter");
  await expect(videos).toHaveAttribute("aria-pressed", "true");
  await expect(videos).toBeFocused();
  expect(Math.abs((await page.evaluate(() => window.scrollY)) - before)).toBeLessThan(2);
  await streams.click();
  await expect(streams).toHaveAttribute("aria-pressed", "true");
  await expect(root.getByRole("heading", { name: "No videos to show" })).toBeVisible();
  if (info.project.name === "phone") {
    const widths = await filters
      .getByRole("button")
      .evaluateAll((buttons) => buttons.map((button) => button.getBoundingClientRect().width));
    expect(Math.max(...widths) - Math.min(...widths)).toBeLessThan(1);
  }
  await page.goto(`/iframe.html?id=app-screens--podcast-channel-${viewport}&viewMode=story`);
  await expect(root.getByRole("heading", { name: "The Quiet Hours", exact: true })).toBeVisible();
  await expect(root.getByRole("group", { name: "Channel media filter" })).toHaveCount(0);
  await page.goto("/iframe.html?id=app-screens--channel-filter-failure&viewMode=story");
  const failedFilters = root.getByRole("group", { name: "Channel media filter" });
  const failedStreams = failedFilters.getByRole("button", { name: "Streams", exact: true });
  await failedStreams.focus();
  await failedStreams.press("Enter");
  await expect(root.locator(".form-error")).toHaveText("The channel filter could not be saved.");
  await expect(failedFilters.getByRole("button", { name: "All", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(failedStreams).toBeFocused();
  await expect(root.locator("article").first()).toBeVisible();
});
