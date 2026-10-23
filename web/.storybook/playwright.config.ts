import { existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { defineConfig } from "@playwright/test";

const storybookPort = process.env.FEEDLR_STORYBOOK_PORT ?? "6006";
if (!/^\d+$/.test(storybookPort) || Number(storybookPort) < 1 || Number(storybookPort) > 65535) {
  throw new Error("FEEDLR_STORYBOOK_PORT must be between 1 and 65535.");
}
const storybookURL = `http://127.0.0.1:${storybookPort}`;

export default defineConfig({
  testDir: ".",
  testMatch: "**/*.spec.ts",
  outputDir: process.env.FEEDLR_VISUAL_OUTPUT ?? join(tmpdir(), "feedlr-visual"),
  fullyParallel: true,
  expect: { timeout: 15_000 },
  use: {
    baseURL: storybookURL,
    reducedMotion: "reduce",
    trace: "retain-on-failure",
    launchOptions: {
      executablePath:
        process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH ??
        (existsSync("/run/current-system/sw/bin/chromium")
          ? "/run/current-system/sw/bin/chromium"
          : undefined),
    },
  },
  projects: [
    {
      name: "phone",
      use: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true },
    },
    { name: "desktop", use: { viewport: { width: 1440, height: 1000 } } },
  ],
  webServer: {
    command: `npm run storybook -- --ci --host 127.0.0.1 --port ${storybookPort}`,
    url: storybookURL,
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
  },
});
