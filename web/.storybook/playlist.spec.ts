import { expect, test } from "@playwright/test";

const firstTitle = "A small cabin, a long weekend, and no plans";
const secondTitle = "Making room for a more thoughtful day";

test("playlist library keeps import in the compact actions menu", async ({ page }, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--playlists-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  const create = root.getByRole("button", { name: "Create playlist", exact: true });
  const actions = root.getByRole("button", { name: "Playlist actions" });
  await expect(create).toBeVisible();
  await expect(root.locator(".page-actions button")).toHaveCount(2);
  await expect(root.getByRole("button", { name: "Import", exact: true })).toHaveCount(0);
  const createBox = await create.boundingBox();
  const actionsBox = await actions.boundingBox();
  expect(createBox).not.toBeNull();
  expect(actionsBox).not.toBeNull();
  if (!createBox || !actionsBox) {
    throw new Error("The playlist header controls are missing");
  }
  expect(actionsBox.x - (createBox.x + createBox.width)).toBeGreaterThanOrEqual(8);
  expect(createBox.height).toBe(testInfo.project.name === "phone" ? 44 : 40);
  expect(actionsBox.height).toBe(testInfo.project.name === "phone" ? 44 : 40);
  await actions.focus();
  await page.keyboard.press("Enter");
  const item = page.getByRole("menuitem", { name: "Import playlist" });
  await expect(page.getByRole("menu")).toBeFocused();
  await expect(page.getByRole("menu")).toHaveAttribute(
    "aria-activedescendant",
    (await item.getAttribute("id")) ?? "",
  );
  await page.keyboard.press("Escape");
  await expect(actions).toBeFocused();
  await actions.click();
  await item.click();
  const dialog = page.getByRole("dialog", { name: "Import a YouTube playlist" });
  await expect(dialog.getByRole("heading", { name: "Import a YouTube playlist" })).toBeVisible();
  await dialog.getByRole("button", { name: "Close dialog" }).click();
  await expect(actions).toBeFocused();
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: testInfo.outputPath(`playlist-library-${viewport}.png`),
    fullPage: true,
    animations: "disabled",
  });
});

test("playlist artwork crops square, portrait, and wide sources into 16:9 cards", async ({
  page,
}) => {
  await page.goto("/iframe.html?id=app-screens--playlists-desktop&viewMode=story");
  const images = page.locator("#storybook-root .playlist-artwork img");
  await expect(images).toHaveCount(3);
  const artwork = await images.evaluateAll(async (elements) =>
    Promise.all(
      elements.map(async (image) => {
        if (!(image instanceof HTMLImageElement) || !image.parentElement) {
          throw new Error("A playlist image is missing");
        }
        image.loading = "eager";
        await image.decode();
        const frame = image.parentElement.getBoundingClientRect();
        const bounds = image.getBoundingClientRect();
        return {
          sourceRatio: image.naturalWidth / image.naturalHeight,
          frameRatio: frame.width / frame.height,
          widthDelta: Math.abs(frame.width - bounds.width),
          heightDelta: Math.abs(frame.height - bounds.height),
          objectFit: getComputedStyle(image).objectFit,
        };
      }),
    ),
  );
  expect(artwork.map((image) => image.sourceRatio)).toEqual([1, 2 / 3, 3]);
  for (const image of artwork) {
    expect(image.frameRatio).toBeCloseTo(16 / 9, 2);
    expect(image.widthDelta).toBeLessThan(1);
    expect(image.heightDelta).toBeLessThan(1);
    expect(image.objectFit).toBe("cover");
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
});

test("media bookmarks stay inside artwork without played-time clutter", async ({
  page,
}, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--playlist-${viewport}&viewMode=story`);
  const card = page.locator("#storybook-root article").first();
  const artwork = card.locator(".media-artwork");
  const bookmark = card.locator(".media-watch-later");
  await expect(bookmark).toBeVisible();
  await expect(artwork.getByRole("button")).toHaveCount(1);
  await expect(bookmark.locator("xpath=ancestor::a")).toHaveCount(0);
  await expect(bookmark).toHaveAttribute("aria-pressed", "true");
  await expect(bookmark).toHaveAccessibleName("Remove from Watch Later");
  const frame = await artwork.boundingBox();
  const button = await bookmark.boundingBox();
  expect(frame).not.toBeNull();
  expect(button).not.toBeNull();
  if (!frame || !button) {
    throw new Error("The media artwork or bookmark is missing");
  }
  expect(button.y - frame.y).toBeGreaterThanOrEqual(0);
  expect(button.y - frame.y).toBeLessThanOrEqual(16);
  expect(frame.x + frame.width - (button.x + button.width)).toBeGreaterThanOrEqual(0);
  expect(frame.x + frame.width - (button.x + button.width)).toBeLessThanOrEqual(16);
  expect(button.height).toBeGreaterThanOrEqual(testInfo.project.name === "phone" ? 44 : 36);
  const duration = await card.locator(".media-duration").boundingBox();
  if (duration) {
    expect(button.y + button.height).toBeLessThan(duration.y);
  }
  await expect(card.locator(".media-detail")).not.toContainText(/\d+:\d+\s+played/);
  await expect(card.locator(".media-card-foot button")).toHaveCount(0);
  await bookmark.click();
  await expect(bookmark).toHaveAttribute("aria-pressed", "false");
  await expect(bookmark).toHaveAccessibleName("Save to Watch Later");
  await expect(bookmark).not.toHaveAttribute("aria-disabled", "true");
  await expect(bookmark).toBeFocused();
  await expect(page.locator(".notification-stack .ui-toast")).toHaveCount(0);
});

test("publication time remains visible through partial, completed, and unplayed states", async ({
  page,
}) => {
  await page.goto("/iframe.html?id=app-screens--playlist-desktop&viewMode=story");
  const root = page.locator("#storybook-root");
  const card = root.locator('article[data-media-id="preview-3"]');
  const published = card.locator("time.media-detail");
  await expect(card.getByRole("progressbar", { name: "Played", exact: true })).toBeVisible();
  await expect(published).toBeVisible();
  await expect(published).toHaveAttribute("datetime", /^2026-/);
  await expect(published).toHaveAttribute("title", /2026/);
  const label = await published.textContent();
  const dateTime = await published.getAttribute("datetime");
  expect(label?.trim()).not.toBe("");
  const actions = card.getByRole("button", { name: /^Actions for / });
  await actions.click();
  await page.getByRole("menuitem", { name: "Mark as watched", exact: true }).click();
  await expect(card.getByText("Watched", { exact: true })).toBeVisible();
  await expect(published).toBeVisible();
  await expect(published).toHaveText(label ?? "");
  await expect(published).toHaveAttribute("datetime", dateTime ?? "");
  await actions.click();
  await page.getByRole("menuitem", { name: "Mark as unwatched", exact: true }).click();
  await expect(card.getByRole("progressbar")).toHaveCount(0);
  await expect(published).toBeVisible();
  await expect(published).toHaveText(label ?? "");
  await expect(published).toHaveAttribute("datetime", dateTime ?? "");
  await expect(card).not.toContainText(/\d+:\d+\s+played/);
  await expect(root.locator(".notification-stack .ui-toast")).toHaveCount(0);
});

test("playlist entries share one menu for media and ordered membership actions", async ({
  page,
}, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--playlist-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  const entries = root.getByRole("list", { name: "Playlist items" }).locator(".playlist-entry");
  await expect(entries).toHaveCount(3);
  await expect(root.locator(".playlist-entry-actions")).toHaveCount(0);
  await expect(entries.first().getByRole("button", { name: /^Actions for / })).toHaveCount(1);
  const firstActions = root.getByRole("button", { name: `Actions for ${firstTitle}` });
  const firstMenu = page.getByRole("menu", {
    name: `Actions for ${firstTitle}`,
    exact: true,
    includeHidden: true,
  });
  await firstActions.click();
  await expect(firstMenu.getByRole("menuitem", { name: "Move up", exact: true })).toBeDisabled();
  await expect(
    firstMenu.getByRole("menuitem", { name: "Add to playlist", exact: true }),
  ).toBeVisible();
  await firstMenu.getByRole("menuitem", { name: "Move down", exact: true }).click();
  await expect(entries.first().locator("article")).toHaveAttribute("data-media-id", "preview-3");
  await expect(entries.nth(1).locator("article")).toHaveAttribute("data-media-id", "preview-1");
  await expect(firstActions).toBeFocused();
  await expect(firstMenu).toHaveCount(0);
  const lastActions = entries.last().getByRole("button", { name: /^Actions for / });
  const lastLabel = await lastActions.getAttribute("aria-label");
  if (!lastLabel) {
    throw new Error("The final playlist entry has no actions label");
  }
  const lastMenu = page.getByRole("menu", { name: lastLabel, exact: true, includeHidden: true });
  await lastActions.click();
  await expect(lastMenu.getByRole("menuitem", { name: "Move down", exact: true })).toBeDisabled();
  await page.keyboard.press("Escape");
  await expect(lastMenu).toHaveCount(0);
  await expect(lastActions).toBeFocused();
  const next = root.getByRole("link", { name: `Watch ${firstTitle}`, exact: true });
  await root.getByRole("button", { name: `Actions for ${secondTitle}` }).click();
  await page
    .getByRole("menu", { name: `Actions for ${secondTitle}`, exact: true })
    .getByRole("menuitem", { name: "Remove from playlist", exact: true })
    .click();
  await expect(entries).toHaveCount(2);
  await expect(next).toBeFocused();
  await expect(entries.first().locator("article")).toHaveAttribute("data-media-id", "preview-1");
  await expect(page.locator(".notification-stack .ui-toast")).toHaveCount(0);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: testInfo.outputPath(`playlist-contents-${viewport}.png`),
    fullPage: true,
    animations: "disabled",
  });
});

test("playlist membership updates visibly without routine success notifications", async ({
  page,
}) => {
  await page.goto("/iframe.html?id=app-screens--playlist-desktop&viewMode=story");
  const root = page.locator("#storybook-root");
  await root.getByRole("button", { name: `Actions for ${firstTitle}` }).click();
  await page.getByRole("menuitem", { name: "Add to playlist", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Add to playlist" });
  const membership = dialog.getByRole("checkbox", { name: "People who make" });
  await expect(membership).not.toBeChecked();
  await membership.click();
  await expect(membership).toBeChecked();
  await expect(membership).toBeEnabled();
  await expect(page.locator(".notification-stack .ui-toast")).toHaveCount(0);
  await membership.click();
  await expect(membership).not.toBeChecked();
  await expect(membership).toBeEnabled();
  await expect(page.locator(".notification-stack .ui-toast")).toHaveCount(0);
});

test("routine following, media state, and channel filters stay quiet", async ({ page }) => {
  await page.goto("/iframe.html?id=app-screens--channel-desktop&viewMode=story");
  const root = page.locator("#storybook-root");
  await root.getByRole("button", { name: "Following", exact: true }).click();
  await expect(root.getByRole("button", { name: "Follow", exact: true })).toBeVisible();
  await expect(page.locator(".notification-stack .ui-toast")).toHaveCount(0);
  await root.getByRole("button", { name: "Follow", exact: true }).click();
  await expect(root.getByRole("button", { name: "Following", exact: true })).toBeVisible();
  const videos = root.getByRole("group", { name: "Channel media filter" }).getByRole("button", {
    name: "Videos",
    exact: true,
  });
  await videos.click();
  await expect(videos).toHaveAttribute("aria-pressed", "true");
  const card = root.locator("article").first();
  const actions = card.getByRole("button", { name: /^Actions for / });
  await actions.click();
  await page.getByRole("menuitem", { name: "Mark as watched", exact: true }).click();
  await expect(card.getByText("Watched", { exact: true })).toBeVisible();
  await expect(page.locator(".notification-stack .ui-toast")).toHaveCount(0);
  await actions.click();
  await page.getByRole("menuitem", { name: "Hide from feed", exact: true }).click();
  await expect(card.getByText("Hidden", { exact: true })).toBeVisible();
  await expect(page.locator(".notification-stack .ui-toast")).toHaveCount(0);
});

test("playlist deletion stays behind confirmation and restores menu focus on cancel", async ({
  page,
}) => {
  await page.goto("/iframe.html?id=app-screens--playlist-desktop&viewMode=story");
  const root = page.locator("#storybook-root");
  const actions = root.getByRole("button", { name: "Playlist actions" });
  await expect(root.getByRole("button", { name: "Add a video", exact: true })).toBeVisible();
  await expect(root.locator(".page-actions button")).toHaveCount(2);
  await expect(root.getByRole("button", { name: "Edit", exact: true })).toHaveCount(0);
  await expect(root.getByRole("button", { name: "Delete playlist", exact: true })).toHaveCount(0);
  await actions.click();
  await expect(page.getByRole("menuitem", { name: "Edit playlist" })).toBeVisible();
  await page.getByRole("menuitem", { name: "Delete playlist" }).click();
  const dialog = page.getByRole("dialog", { name: "Delete “A slower Sunday”?" });
  await expect
    .poll(() => dialog.evaluate((element) => element.contains(document.activeElement)))
    .toBe(true);
  await expect(dialog.getByRole("button", { name: "Delete", exact: true })).not.toBeFocused();
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(actions).toBeFocused();
  await expect(root.getByRole("heading", { name: "A slower Sunday", level: 1 })).toBeVisible();
  await actions.click();
  await page.getByRole("menuitem", { name: "Delete playlist" }).click();
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(root.getByRole("heading", { name: "Playlists", level: 1 })).toBeVisible();
  await expect(root.getByRole("link", { name: /A slower Sunday/ })).toHaveCount(0);
});

for (const activation of ["pointer", "keyboard"] as const) {
  test(`menu dialogs retain focus after ${activation} handoff and restore their trigger`, async ({
    page,
  }) => {
    for (const flow of [
      {
        entry: "playlists-desktop",
        trigger: "Playlist actions",
        command: "Import playlist",
        title: "Import a YouTube playlist",
        key: "Home",
      },
      {
        entry: "playlist-desktop",
        trigger: "Playlist actions",
        command: "Edit playlist",
        title: "Edit playlist",
        key: "Home",
      },
      {
        entry: "playlist-desktop",
        trigger: "Playlist actions",
        command: "Delete playlist",
        title: "Delete “A slower Sunday”?",
        key: "End",
      },
      {
        entry: "playlist-desktop",
        trigger: `Actions for ${firstTitle}`,
        command: "Add to playlist",
        title: "Add to playlist",
        key: "Home",
      },
    ]) {
      await test.step(flow.command, async () => {
        await page.goto(`/iframe.html?id=app-screens--${flow.entry}&viewMode=story`);
        const trigger = page.getByRole("button", { name: flow.trigger, exact: true });
        if (activation === "pointer") {
          await trigger.click();
          await page.getByRole("menuitem", { name: flow.command, exact: true }).click();
        } else {
          await page.mouse.move(0, 0);
          await trigger.focus();
          await page.keyboard.press("Enter");
          const menu = page.getByRole("menu");
          await expect(menu).toBeFocused();
          await expect(menu).toHaveAttribute(
            "aria-activedescendant",
            (await menu.getByRole("menuitem").first().getAttribute("id")) ?? "",
          );
          await page.keyboard.press(flow.key);
          const command = page.getByRole("menuitem", { name: flow.command, exact: true });
          await expect(menu).toHaveAttribute(
            "aria-activedescendant",
            (await command.getAttribute("id")) ?? "",
          );
          await page.keyboard.press("Enter");
        }
        const dialog = page.getByRole("dialog", { name: flow.title, exact: true });
        await expect(dialog.getByRole("heading", { name: flow.title, exact: true })).toBeVisible();
        // Flush the menu's deferred restoration, then verify the modal still owns focus.
        await page.evaluate(
          () =>
            new Promise<void>((resolve) =>
              requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
            ),
        );
        await expect
          .poll(() => dialog.evaluate((element) => element.contains(document.activeElement)))
          .toBe(true);
        await page.keyboard.press("Tab");
        await expect
          .poll(() => dialog.evaluate((element) => element.contains(document.activeElement)))
          .toBe(true);
        await page.keyboard.press("Shift+Tab");
        await expect
          .poll(() => dialog.evaluate((element) => element.contains(document.activeElement)))
          .toBe(true);
        await page.keyboard.press("Escape");
        await expect(dialog).toHaveCount(0);
        await expect(trigger).toBeFocused();
      });
    }
  });
}

test("persistent dialogs restore their opener and allow menus within the same modal", async ({
  page,
}) => {
  await page.goto("/iframe.html?id=foundation-overlays--persistent-dialog-menu&viewMode=story");
  const trigger = page.getByRole("button", { name: "Playlist options", exact: true });
  for (const activation of ["pointer", "keyboard"]) {
    if (activation === "pointer") {
      await trigger.click();
      await page.getByRole("menuitem", { name: "Edit playlist", exact: true }).click();
    } else {
      await trigger.focus();
      await page.keyboard.press("Enter");
      await page.keyboard.press("Enter");
    }
    const dialog = page.getByRole("dialog", { name: "Playlist settings", exact: true });
    await expect(dialog.getByRole("heading", { name: "Playlist settings" })).toBeVisible();
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    );
    await expect
      .poll(() => dialog.evaluate((element) => element.contains(document.activeElement)))
      .toBe(true);
    const nestedTrigger = dialog.getByRole("button", { name: "View options", exact: true });
    await nestedTrigger.click();
    await page.getByRole("menuitem", { name: "Keep editing", exact: true }).click();
    await expect(nestedTrigger).toBeFocused();
    await dialog.getByRole("button", { name: "Done", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(trigger).toBeFocused();
  }
});

test("imported playlist refresh stays available in the secondary menu", async ({
  page,
}, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--playlists-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  await root.getByRole("link", { name: /The long way home/ }).click();
  await expect(root.getByRole("heading", { name: "The long way home", level: 1 })).toBeVisible();
  await expect(root.getByRole("button", { name: "Refresh import", exact: true })).toHaveCount(0);
  await root.getByRole("button", { name: "Playlist actions" }).click();
  const request = page.waitForRequest(
    (request) =>
      request.method() === "POST" &&
      new URL(request.url()).pathname === "/api/v1/playlists/long-way-home/sync",
  );
  await page.getByRole("menuitem", { name: "Refresh import" }).click();
  await request;
  await expect(root.getByText("Playlist refreshed", { exact: true })).toBeVisible();
  await expect(root.getByRole("button", { name: "Playlist actions" })).toBeFocused();
});

test("import refresh preserves a deliberate focus change while the request waits", async ({
  page,
}) => {
  await page.goto("/iframe.html?id=app-screens--imported-playlist-pending&viewMode=story");
  const root = page.locator("#storybook-root");
  const add = root.getByRole("button", { name: "Add a video", exact: true });
  await root.getByRole("button", { name: "Playlist actions" }).click();
  await page.getByRole("menuitem", { name: "Refresh import" }).click();
  await add.focus();
  await expect(root.getByText("Playlist refreshed", { exact: true })).toBeVisible();
  await expect(add).toBeFocused();
});

test("playlist menus fit the viewport and scroll internally for the last keyboard command", async ({
  page,
}, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--playlist-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  const trigger = root
    .locator(".playlist-entry")
    .last()
    .getByRole("button", { name: /^Actions for / });
  await expect(trigger).toBeVisible();
  await trigger.evaluate((element) => {
    window.scrollBy(0, element.getBoundingClientRect().top - window.innerHeight * 0.78);
  });
  await trigger.click();
  const menu = page.getByRole("menu");
  const remove = menu.getByRole("menuitem", { name: "Remove from playlist" });
  await expect
    .poll(() =>
      menu.evaluate((element) => {
        const bounds = element.getBoundingClientRect();
        return (
          bounds.top >= 16 &&
          bounds.left >= 16 &&
          bounds.bottom <= innerHeight - 16 &&
          bounds.right <= innerWidth - 16
        );
      }),
    )
    .toBe(true);
  await expect(remove).toBeInViewport({ ratio: 1 });
  expect(
    await remove.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      return element.contains(
        document.elementFromPoint(bounds.x + bounds.width / 2, bounds.y + bounds.height / 2),
      );
    }),
  ).toBe(true);
  const before = await page.evaluate(() => window.scrollY);
  await page.keyboard.press("End");
  await expect(menu).toHaveAttribute(
    "aria-activedescendant",
    (await remove.getAttribute("id")) ?? "",
  );
  expect(Math.abs((await page.evaluate(() => window.scrollY)) - before)).toBeLessThan(1);
  await page.screenshot({
    path: testInfo.outputPath("playlist-bottom-menu.png"),
    animations: "disabled",
  });
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  await page.setViewportSize({ width: page.viewportSize()?.width ?? 390, height: 320 });
  await trigger.evaluate((element) => {
    window.scrollBy(0, element.getBoundingClientRect().top - window.innerHeight / 2);
  });
  await trigger.click();
  await expect
    .poll(() =>
      menu.evaluate((element) => {
        const bounds = element.getBoundingClientRect();
        return (
          bounds.top >= 16 &&
          bounds.bottom <= innerHeight - 16 &&
          element.scrollHeight > element.clientHeight
        );
      }),
    )
    .toBe(true);
  const shortBefore = await page.evaluate(() => window.scrollY);
  await page.keyboard.press("End");
  await expect(menu).toHaveAttribute(
    "aria-activedescendant",
    (await remove.getAttribute("id")) ?? "",
  );
  await expect(remove).toBeInViewport({ ratio: 1 });
  await expect.poll(() => menu.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
  expect(Math.abs((await page.evaluate(() => window.scrollY)) - shortBefore)).toBeLessThan(1);
});

test("system playlists expose no forbidden editing commands", async ({ page }, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--system-playlist-${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  await expect(root.getByRole("heading", { name: "Watch Later", level: 1 })).toBeVisible();
  await expect(root.getByRole("button", { name: "Add a video", exact: true })).toHaveCount(0);
  await expect(root.getByRole("button", { name: "Playlist actions" })).toHaveCount(0);
  await root.getByRole("button", { name: `Actions for ${firstTitle}` }).click();
  await expect(page.getByRole("menuitem", { name: "Add to playlist", exact: true })).toBeVisible();
  await expect(page.getByRole("menuitem", { name: /^Move / })).toHaveCount(0);
  await expect(page.getByRole("menuitem", { name: "Remove from playlist" })).toHaveCount(0);
});

test("failed playlist writes preserve order, disable pending commands, and allow retry", async ({
  page,
}) => {
  await page.goto("/iframe.html?id=app-screens--playlist-action-failure&viewMode=story");
  const root = page.locator("#storybook-root");
  const actions = root.getByRole("button", { name: `Actions for ${firstTitle}` });
  const entries = root.getByRole("list", { name: "Playlist items" }).locator(".playlist-entry");
  await actions.click();
  await page.getByRole("menuitem", { name: "Move down", exact: true }).click();
  await actions.click();
  for (const label of ["Move up", "Move down", "Remove from playlist"]) {
    await expect(page.getByRole("menuitem", { name: label, exact: true })).toBeDisabled();
  }
  await expect(root.getByRole("alert")).toContainText("The playlist could not be updated");
  await expect(entries.first().locator("article")).toHaveAttribute("data-media-id", "preview-1");
  await expect(page.getByRole("menuitem", { name: "Move down", exact: true })).toBeEnabled();
  await page.getByRole("menuitem", { name: "Move down", exact: true }).click();
  await expect(entries.first().locator("article")).toHaveAttribute("data-media-id", "preview-3");
  await expect(entries.nth(1).locator("article")).toHaveAttribute("data-media-id", "preview-1");
  await expect(actions).toBeFocused();
});
