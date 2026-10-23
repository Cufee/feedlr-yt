import { expect, type Page, test } from "@playwright/test";

async function ready(page: Page, kind: "video" | "podcast", story: string) {
  await page.goto(`/iframe.html?id=${story}&viewMode=story`);
  const media = page.locator(kind === "video" ? "video" : "audio");
  await expect
    .poll(() => media.evaluate((element: HTMLMediaElement) => element.readyState))
    .toBeGreaterThanOrEqual(2);
  await media.evaluate((element: HTMLMediaElement) => element.pause());
  await page.evaluate(() => {
    document.documentElement.dataset.escapeDocument = "original";
  });
  return media;
}

for (const kind of ["video", "podcast"] as const) {
  test(`Escape returns the native ${kind} route through the SPA and stops media`, async ({
    page,
  }, info) => {
    const viewport = info.project.name === "phone" ? "mobile" : "desktop";
    const media = await ready(page, kind, `playback-screens--${kind}-${viewport}`);
    const retained = await media.elementHandle();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("heading", { level: 1, name: "Your feed" })).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("data-escape-document", "original");
    await expect(page.locator("audio,video")).toHaveCount(0);
    await expect
      .poll(() =>
        retained?.evaluate((element: HTMLMediaElement) => element.paused && !element.isConnected),
      )
      .toBe(true);
    await expect(page).toHaveTitle("Your feed · Feedlr");
  });
}

test("Escape retains history return search and supports Back and Forward", async ({ page }) => {
  await ready(page, "podcast", "playback-screens--podcast-return-to-history");
  await page.keyboard.press("Escape");
  const route = page.getByRole("status", { name: "Preview route" });
  await expect(page.getByRole("heading", { level: 1, name: "Recently played" })).toBeVisible();
  await expect(route).toContainText("source=podcasts");
  await expect(route).toContainText("page=2");
  await page.getByRole("button", { name: "Back in preview" }).click();
  await expect(
    page.getByRole("heading", { level: 1, name: "How we build better habits" }),
  ).toBeVisible();
  await expect(page).toHaveTitle("How we build better habits · Feedlr");
  await page.getByRole("button", { name: "Forward in preview" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Recently played" })).toBeVisible();
  await expect(page.locator("audio")).toHaveCount(0);
  await expect(page).toHaveTitle("Recently played · Feedlr");
});

test("Escape rejects an external return destination", async ({ page }) => {
  const requests: string[] = [];
  page.on("request", (request) => requests.push(request.url()));
  await ready(page, "video", "playback-screens--unsafe-return");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("heading", { level: 1, name: "Your feed" })).toBeVisible();
  await expect(page.getByRole("status", { name: "Preview route" })).toHaveText("/app");
  expect(requests.some((url) => url.includes("untrusted.example"))).toBe(false);
});

test("menus and playlist dialogs consume Escape before the podcast route", async ({
  page,
}, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await ready(page, "podcast", `playback-screens--podcast-${viewport}`);
  const actions = page.getByRole("button", { name: "Player actions" });
  await actions.click();
  await expect(page.getByRole("menu")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("menu")).toHaveCount(0);
  await expect(actions).toBeFocused();
  await expect(page.locator(".playback-podcast")).toBeVisible();
  await actions.click();
  await page.getByRole("menuitem", { name: "Add to playlist" }).click();
  const dialog = page.getByRole("dialog", { name: "Add to playlist" });
  await expect(dialog.locator(".ui-dialog-panel")).toBeVisible();
  await dialog.getByRole("button", { name: "Close dialog" }).focus();
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(page.locator(".playback-podcast")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("heading", { level: 1, name: "Your feed" })).toBeVisible();
});

test("native Shaka menus consume Escape before closing video", async ({ page }, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await ready(page, "video", `playback-screens--video-${viewport}`);
  await page.locator(".shaka-overflow-menu-button").click();
  const overflow = page.locator(".shaka-overflow-menu");
  await expect(overflow).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(overflow).toBeHidden();
  await expect(page.locator(".playback-video")).toBeVisible();
  await page.locator(".shaka-overflow-menu-button").click();
  await page.locator(".shaka-overflow-menu .shaka-resolution-button").click();
  const resolutions = page.locator(".shaka-resolutions");
  await expect(resolutions).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(resolutions).toBeHidden();
  await expect(page.locator(".playback-video")).toBeVisible();
});

test("a native menu closing before the route key handler still owns its Escape", async ({
  page,
}, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await ready(page, "video", `playback-screens--video-${viewport}`);
  await page.locator(".shaka-overflow-menu-button").click();
  const overflow = page.locator(".shaka-overflow-menu");
  await expect(overflow).toBeVisible();
  await page.evaluate(() => {
    document.addEventListener(
      "keydown",
      (event) => {
        if (event.key !== "Escape") {
          return;
        }
        for (const menu of document.querySelectorAll(".shaka-overflow-menu,.shaka-settings-menu")) {
          menu.classList.add("shaka-hidden");
        }
      },
      { capture: true, once: true },
    );
  });
  await page.keyboard.press("Escape");
  await expect(overflow).toBeHidden();
  await expect(page.locator(".playback-video")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("heading", { level: 1, name: "Your feed" })).toBeVisible();
});

test("fullscreen keeps the player on the first Escape", async ({ page }, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await ready(page, "video", `playback-screens--video-${viewport}`);
  await page.getByRole("button", { name: "Full screen", exact: true }).click();
  await expect.poll(() => page.evaluate(() => !!document.fullscreenElement)).toBe(true);
  await page.keyboard.press("Escape");
  await expect(page.locator(".playback-video")).toBeVisible();
  // Chromium's automated key delivery can omit the browser's fullscreen shortcut.
  // The route must remain open either way; finish the native exit when necessary.
  await page.evaluate(async () => {
    if (document.fullscreenElement) {
      await document.exitFullscreen();
    }
  });
  await expect.poll(() => page.evaluate(() => !!document.fullscreenElement)).toBe(false);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("heading", { level: 1, name: "Your feed" })).toBeVisible();
});

test("prevented, composing, repeated and editing Escape events do not leave the player", async ({
  page,
}, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await ready(page, "podcast", `playback-screens--podcast-${viewport}`);
  await page.evaluate(() => {
    const prevent = (event: KeyboardEvent) => event.preventDefault();
    window.addEventListener("keydown", prevent, { capture: true, once: true });
    document.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }),
    );
    document.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Escape", bubbles: true, repeat: true }),
    );
    document.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Escape", bubbles: true, isComposing: true }),
    );
    const editing = document.createElement("input");
    editing.id = "escape-editing-fixture";
    document.querySelector(".playback-podcast")?.append(editing);
    editing.focus();
  });
  await page.keyboard.press("Escape");
  await expect(page.locator(".playback-podcast")).toBeVisible();
  await page.locator("#escape-editing-fixture").evaluate((element) => element.remove());
  await page.keyboard.press("Escape");
  await expect(page.getByRole("heading", { level: 1, name: "Your feed" })).toBeVisible();
});
