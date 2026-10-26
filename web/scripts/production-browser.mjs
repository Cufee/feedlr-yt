import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium, expect } from "@playwright/test";
import { previewMedia } from "../src/app/preview-fixtures.ts";

const buildDirectory = process.env.FEEDLR_PRODUCTION_ASSETS
  ? new URL(`${process.env.FEEDLR_PRODUCTION_ASSETS.replace(/\/$/, "")}/`, "file:///")
  : new URL("../../assets/ui/", import.meta.url);
const outputDirectory =
  process.env.FEEDLR_PRODUCTION_OUTPUT ??
  (await mkdtemp(join(tmpdir(), "feedlr-production-browser-")));
await mkdir(outputDirectory, { recursive: true });
const document = await readFile(new URL("index.html", buildDirectory));
const origin = "http://127.0.0.1:4173";
const saved = previewMedia.filter((item) => item.inWatchLater);
const feed = {
  new: previewMedia.filter((item) => !item.completed && !item.hidden),
  watched: previewMedia.filter((item) => item.completed && !item.hidden),
  watchLater: saved,
  onboarding: false,
};
const executablePath =
  process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH ??
  (existsSync("/run/current-system/sw/bin/chromium")
    ? "/run/current-system/sw/bin/chromium"
    : undefined);
const browser = await chromium.launch({ executablePath, headless: true });
const failures = [];

try {
  for (const width of [1440, 390]) {
    for (const authenticated of [true, false]) {
      const name = `${authenticated ? "feed" : "guest"}-${width}`;
      const context = await browser.newContext({
        viewport: { width, height: width === 390 ? 844 : 1000 },
        isMobile: width === 390,
        hasTouch: width === 390,
        reducedMotion: "reduce",
        serviceWorkers: "block",
      });
      const page = await context.newPage();
      page.setDefaultTimeout(15_000);
      const errors = [];
      const unexpectedRequests = [];
      const requests = [];
      const loadedAssets = [];
      page.on("pageerror", (error) => errors.push(error.message));
      page.on("console", (message) => {
        if (message.type() === "error") {
          errors.push(message.text());
        }
      });
      // Serve the emitted HTML and every asset verbatim. No source entry,
      // Storybook preview, injected stylesheet, network server or live API is used.
      await context.route("**/*", async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        requests.push(`${request.method()} ${url.pathname}${url.search}`);
        if (url.origin === origin && request.method() === "GET") {
          if (request.isNavigationRequest() && url.pathname.startsWith("/app")) {
            return route.fulfill({ contentType: "text/html", body: document });
          }
          if (/^\/ui-assets\/assets\/[\w.-]+\.(?:js|css)$/.test(url.pathname)) {
            const asset = url.pathname.slice("/ui-assets/".length);
            const body = await readFile(new URL(asset, buildDirectory));
            loadedAssets.push(url.pathname);
            return route.fulfill({
              contentType: asset.endsWith(".css") ? "text/css" : "text/javascript",
              body,
            });
          }
          if (url.pathname === "/assets/favicon.png") {
            return route.fulfill({
              contentType: "image/png",
              body: await readFile(new URL("../../assets/favicon.png", import.meta.url)),
            });
          }
          let body;
          if (url.pathname === "/api/v1/session") {
            body = {
              authenticated,
              ...(authenticated ? { userId: "built-preview", username: "Morgan" } : {}),
              capabilities: { podcastSearch: true, youtubeSync: true, tvSync: true },
            };
          } else if (authenticated && url.pathname === "/api/v1/feed") {
            body = feed;
          } else if (authenticated && url.pathname === "/api/v1/watch-later") {
            body = { items: saved, page: { page: 1, hasMore: false } };
          }
          if (body) {
            return route.fulfill({ json: body });
          }
        }
        unexpectedRequests.push(`${request.method()} ${url.href}`);
        return route.abort("blockedbyclient");
      });

      try {
        await page.goto(`${origin}/app`);
        const contentHeading = page.getByRole("heading", {
          name: authenticated ? "New releases" : "Sign in to see your feed",
          exact: true,
        });
        await expect(contentHeading).toBeVisible();
        if (authenticated) {
          await expect(page.locator(".media-card").first()).toBeVisible();
        }
        const measurements = await page.evaluate(() => {
          const selectors = [
            ".app-frame",
            ".app-header",
            ".app-main",
            ".app-header .ui-button",
            ".app-sidebar",
            ".media-card-body",
          ];
          return {
            scrollWidth: document.documentElement.scrollWidth,
            elements: Object.fromEntries(
              selectors.map((selector) => {
                const element = document.querySelector(selector);
                if (!element) {
                  return [selector, null];
                }
                const style = getComputedStyle(element);
                return [
                  selector,
                  {
                    padding: style.padding,
                    paddingTop: style.paddingTop,
                    fontSize: style.fontSize,
                    rect: element.getBoundingClientRect().toJSON(),
                  },
                ];
              }),
            ),
          };
        });
        await writeFile(
          join(outputDirectory, `${name}.json`),
          JSON.stringify({ measurements, loadedAssets, requests, errors }, null, 2),
        );
        await page.screenshot({
          path: join(outputDirectory, `${name}.png`),
          fullPage: true,
          animations: "disabled",
        });

        const elements = measurements.elements;
        const phone = width === 390;
        assert.equal(elements[".app-frame"].paddingTop, phone ? "72px" : "84px", name);
        assert.equal(elements[".app-header"].padding, phone ? "10px 14px" : "12px 20px", name);
        assert.equal(
          elements[".app-main"].padding,
          phone ? "28px 18px 40px" : "44px 40px 56px",
          name,
        );
        assert.equal(elements[".app-header .ui-button"].fontSize, "14px", name);
        assert.equal(
          elements[".app-header .ui-button"].padding,
          phone ? "8px 10px" : authenticated ? "8px" : "8px 16px",
          name,
        );
        assert.ok(measurements.scrollWidth <= width, `${name}: horizontal overflow`);
        const header = elements[".app-header"].rect;
        const content = await contentHeading.boundingBox();
        assert.ok(content.y >= header.bottom + 16, `${name}: content overlaps the fixed header`);
        assert.ok(loadedAssets.some((asset) => /^\/ui-assets\/assets\/index-.+\.js$/.test(asset)));
        assert.ok(loadedAssets.some((asset) => /^\/ui-assets\/assets\/index-.+\.css$/.test(asset)));
        if (authenticated) {
          assert.equal(elements[".app-sidebar"].padding, phone ? "6px 8px" : "16px 10px", name);
          await expect(page.getByRole("heading", { level: 1 })).toHaveCount(0);
          assert.equal(elements[".media-card-body"].paddingTop, phone ? "14px" : "13px", name);
          await page.getByRole("link", { name: "Watch Later", exact: true }).click();
          await expect(page).toHaveTitle("Watch Later · Feedlr");
          await expect(page.locator(".media-card")).toHaveCount(saved.length);
          await page.goBack();
          await expect(contentHeading).toBeVisible();
        }
        assert.deepEqual(unexpectedRequests, [], `${name}: unexpected network requests`);
        assert.deepEqual(errors, [], `${name}: browser errors`);
        console.log(`${name}: production bundle layout and routing passed`);
      } catch (error) {
        failures.push(new Error(`${name}: ${error.message}`, { cause: error }));
        await page.screenshot({
          path: join(outputDirectory, `${name}-failure.png`),
          fullPage: true,
        });
      } finally {
        await context.close();
      }
    }
  }
} finally {
  await browser.close();
  console.log(`Production browser artifacts: ${outputDirectory}`);
  console.log(`Production build: ${fileURLToPath(buildDirectory)}`);
}
if (failures.length) {
  throw new AggregateError(failures, "The production build failed its browser checks");
}
