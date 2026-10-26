import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { expect, test } from "@playwright/test";

test("the production build pins every playback dependency to a content hash", async () => {
  test.skip(test.info().project.name !== "desktop");
  const root = new URL("../../", import.meta.url);
  const output = new URL("assets/ui/", root);
  const manifest = JSON.parse(await readFile(new URL("manifest.json", output), "utf8")) as Record<
    string,
    { file: string; src?: string; assets?: string[] }
  >;
  const entry = manifest["index.html"];
  expect(entry).toBeDefined();
  const bootstrap = await readFile(new URL(entry?.file ?? "missing-entry", output), "utf8");
  const sources = [
    "assets/js/feedlr-player.js",
    "assets/js/feedlr-podcast-player.js",
    "assets/css/player.css",
    "assets/vendor/shaka-player.ui.js",
    "assets/vendor/controls.css",
  ];
  for (const source of sources) {
    const asset = Object.values(manifest).find((item) => item.src?.endsWith(source));
    expect(asset, source).toBeDefined();
    expect(asset?.file).toMatch(/^assets\/.+-[\w-]{16}\.(?:js|css)$/);
    expect(entry?.assets).toContain(asset?.file);
    expect(bootstrap).toContain(`/ui-assets/${asset?.file}`);
    const built = await readFile(new URL(asset?.file ?? "missing-asset", output));
    if (source.endsWith(".css")) {
      // Vite minifies stylesheets imported through ?url, while scripts stay opaque assets.
      expect(built.toString()).toContain(
        source.endsWith("player.css") ? ".feedlr-video-page" : ".shaka-controls-container",
      );
      expect(built.toString()).not.toMatch(/@import|@font-face/);
      for (const [, dependency] of built.toString().matchAll(/url\(([^)]+)\)/g)) {
        expect(dependency?.trim().replace(/^["']/, "")).toMatch(/^data:/);
      }
    } else {
      expect(built.equals(await readFile(new URL(source, root))), source).toBe(true);
    }
  }
  for (const asset of Object.values(manifest)) {
    expect(asset.file, fileURLToPath(output)).toMatch(/^assets\/.+-[\w-]{16}\.(?:js|css)$/);
  }
});

test("a failed player stylesheet can be retried before the real native player mounts", async ({
  page,
  context,
}, info) => {
  let styleRequests = 0;
  // Storybook's MSW forwards stylesheet requests through its worker as fetches.
  await context.route(/\/player(?:-[\w-]+)?\.css(?:\?.*)?$/, async (route) => {
    if (!new URL(route.request().url()).searchParams.has("url") && ++styleRequests === 1) {
      await route.abort("failed");
    } else {
      await route.continue();
    }
  });
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=playback-screens--video-${viewport}&viewMode=story`);
  await expect(page.getByRole("alert")).toHaveText("The player styles could not load. Try again.");
  await expect(page.locator("link[data-feedlr-player]")).toHaveCount(0);
  await expect(page.locator("video")).toHaveCount(0);
  await page.getByRole("button", { name: "Try again", exact: true }).click();
  const video = page.locator("video");
  await expect
    .poll(() => video.evaluate((element: HTMLVideoElement) => element.readyState))
    .toBeGreaterThanOrEqual(2);
  await expect(page.locator(".shaka-controls-container")).toHaveCount(1);
  await expect(page.locator("link[data-feedlr-player]")).toHaveCount(1);
  await expect(page.locator("link[data-feedlr-player]")).toHaveAttribute(
    "data-feedlr-loaded",
    "true",
  );
  expect(styleRequests).toBe(2);
  const assets = await page.evaluate(() => {
    const scripts = Array.from(document.scripts)
      .map((script) => script.src)
      .filter((url) => /\/(feedlr-player|shaka-player\.ui)(?:-[\w-]+)?\.js(?:\?|$)/.test(url));
    const styles = Array.from(
      document.querySelectorAll<HTMLLinkElement>(
        "link[data-feedlr-player],link[data-feedlr-shaka]",
      ),
    ).map((link) => link.href);
    return [...scripts, ...styles];
  });
  expect(assets).toHaveLength(4);
  for (const asset of assets) {
    expect(new URL(asset).pathname).not.toMatch(/^\/assets\/(?:js|css|vendor)\//);
  }
  await page.getByRole("link", { name: "Close player" }).click();
  await expect(page.getByRole("heading", { level: 2, name: "New releases" })).toBeVisible();
  await expect(page.locator("video")).toHaveCount(0);
});

test("native controls wait for their stylesheet and preserve cleanup while it loads", async ({
  page,
  context,
}, info) => {
  let release: () => void = () => undefined;
  const loading = new Promise<void>((resolve) => {
    release = resolve;
  });
  let requested = false;
  await context.route(/\/controls(?:-[\w-]+)?\.css(?:\?.*)?$/, async (route) => {
    if (!new URL(route.request().url()).searchParams.has("url")) {
      requested = true;
      await loading;
    }
    await route.continue();
  });
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=playback-screens--video-${viewport}&viewMode=story`);
  await expect.poll(() => requested).toBe(true);
  await expect(page.locator("video")).toHaveCount(0);
  await page.getByRole("link", { name: "Close player" }).click();
  await expect(page.getByRole("heading", { level: 2, name: "New releases" })).toBeVisible();
  release();
  await expect(page.locator("link[data-feedlr-shaka]")).toHaveAttribute(
    "data-feedlr-loaded",
    "true",
  );
  await expect(page.locator("video,.shaka-controls-container")).toHaveCount(0);
});

test("the podcast route loads its pinned runtime and genuine media adapter", async ({
  page,
}, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=playback-screens--podcast-${viewport}&viewMode=story`);
  const audio = page.locator("audio");
  await expect
    .poll(() => audio.evaluate((element: HTMLAudioElement) => element.readyState))
    .toBeGreaterThanOrEqual(2);
  const script = page.locator('script[src*="feedlr-podcast-player"]');
  await expect(script).toHaveCount(1);
  const source = await script.getAttribute("src");
  expect(new URL(source ?? "", page.url()).pathname).not.toBe(
    "/assets/js/feedlr-podcast-player.js",
  );
  await page.getByRole("link", { name: "Close player" }).click();
  await expect(page.getByRole("heading", { level: 2, name: "New releases" })).toBeVisible();
  await expect(audio).toHaveCount(0);
});
