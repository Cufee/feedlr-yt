import { expect, type Page, test } from "@playwright/test";

async function noticeFixture(page: Page) {
  await page.clock.install({ time: new Date("2026-10-22T11:00:00Z") });
  await page.goto("/iframe.html?id=playback-player--playback-notices&viewMode=story");
  await expect(page.getByRole("region", { name: "Video player" })).toBeVisible();
  await page.clock.pauseAt(new Date("2026-10-22T12:00:00Z"));
  return page.locator('[data-sonner-toast][data-type="info"]');
}

test("sponsor notices expire while playback updates and repeated skips get a new lifetime", async ({
  page,
}) => {
  const notice = await noticeFixture(page);
  for (let skip = 0; skip < 2; skip++) {
    await page.getByRole("button", { name: "Skip sponsor" }).click();
    await page.clock.runFor(300);
    await expect(notice).toHaveText("SponsorBlock skipped a video segment");
    await page.mouse.move(0, 0);
    await page.clock.fastForward(5_000);
    await expect(notice).toBeVisible();
    await page.getByRole("button", { name: "Advance playback" }).click();
    await page.clock.fastForward(1_100);
    await page.clock.runFor(250);
    await expect(notice).toHaveCount(0);
  }
});

test("a replacement notice starts its own lifetime and its close control dismisses it", async ({
  page,
}) => {
  const notice = await noticeFixture(page);
  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await page.clock.runFor(300);
  await expect(notice).toBeVisible();
  await page.mouse.move(0, 0);
  await page.clock.fastForward(5_000);
  await page.getByRole("button", { name: "Use YouTube player" }).click();
  await expect
    .poll(async () => {
      await page.clock.runFor(100);
      return notice.allTextContents();
    })
    .toEqual(["Switched to YouTube player"]);
  await page.clock.fastForward(1_100);
  await expect(notice).toBeVisible();
  await page.clock.fastForward(5_000);
  await page.clock.runFor(250);
  await expect(notice).toHaveCount(0);

  for (let repeat = 0; repeat < 2; repeat++) {
    await page.getByRole("button", { name: "Use YouTube player" }).click();
    await page.clock.runFor(300);
    await expect(notice).toHaveText("Switched to YouTube player");
    const close = notice.getByRole("button", { name: "Dismiss notification" });
    await expect(close).toBeInViewport();
    await close.focus();
    await close.press("Enter");
    await page.clock.runFor(250);
    await expect(notice).toHaveCount(0);
  }
});

test("playback notices use the library hover pause", async ({ page }) => {
  const notice = await noticeFixture(page);
  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await page.clock.runFor(300);
  await notice.hover();
  await page.clock.fastForward(12_000);
  await expect(notice).toBeVisible();
  await page.mouse.move(0, 0);
  await page.clock.fastForward(6_100);
  await page.clock.runFor(250);
  await expect(notice).toHaveCount(0);
});

test("TV progress stays visible for the request without reviving previous notices", async ({
  page,
}) => {
  const notice = await noticeFixture(page);
  const sending = page.locator('[data-sonner-toast][data-type="loading"]');
  const toggleTV = page.getByRole("button", { name: "Toggle TV progress" });
  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await page.clock.runFor(300);
  await notice.getByRole("button", { name: "Dismiss notification" }).click();
  await page.clock.runFor(250);
  await expect(notice).toHaveCount(0);
  await toggleTV.click();
  await page.clock.runFor(100);
  await expect(sending).toHaveText("Sending to TV…");
  await page.clock.fastForward(12_000);
  await expect(sending).toBeVisible();
  await toggleTV.click();
  await expect
    .poll(async () => {
      await page.clock.runFor(100);
      return sending.count();
    })
    .toBe(0);
  await expect(notice).toHaveCount(0);

  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await page.clock.runFor(300);
  await expect(notice).toBeVisible();
  await page.mouse.move(0, 0);
  await page.clock.fastForward(5_000);
  await toggleTV.click();
  await page.clock.runFor(100);
  await expect(sending).toHaveText("Sending to TV…");
  await page.clock.fastForward(2_000);
  await page.clock.runFor(250);
  await expect(sending).toHaveText("Sending to TV…");
  await expect(notice).toHaveCount(0);
  await toggleTV.click();
  await expect
    .poll(async () => {
      await page.clock.runFor(100);
      return sending.count();
    })
    .toBe(0);
  await expect(notice).toHaveCount(0);
});

test("fullscreen preserves a notice's element, lifetime and dismissal", async ({ page }, info) => {
  test.skip(info.project.name === "phone", "This regression exercises native desktop fullscreen.");
  const notice = await noticeFixture(page);
  const stage = page.locator(".playback-video-stage");
  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await page.clock.runFor(300);
  const original = await notice.elementHandle();
  expect(original).not.toBeNull();
  await page.mouse.move(0, 0);
  await page.clock.fastForward(4_000);
  await stage.click({ position: { x: 20, y: 20 } });
  await stage.evaluate((element) => element.requestFullscreen());
  await expect.poll(() => page.evaluate(() => !!document.fullscreenElement)).toBe(true);
  await expect(stage.locator("[data-sonner-toast]")).toBeVisible();
  expect(
    await original?.evaluate(
      (element) =>
        element === document.querySelector('[data-sonner-toast][data-type="info"]') &&
        document.fullscreenElement?.contains(element),
    ),
  ).toBe(true);
  await page.clock.fastForward(1_000);
  await expect(notice).toBeVisible();
  await page.clock.fastForward(1_000);
  await page.clock.runFor(250);
  await expect(notice).toHaveCount(0);
  await page.evaluate(() => document.exitFullscreen());
  await expect.poll(() => page.evaluate(() => !!document.fullscreenElement)).toBe(false);
  await expect(notice).toHaveCount(0);

  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await page.clock.runFor(300);
  await stage.click({ position: { x: 20, y: 20 } });
  await stage.evaluate((element) => element.requestFullscreen());
  const close = stage.getByRole("button", { name: "Dismiss notification" });
  await expect(close).toBeInViewport();
  await close.focus();
  await close.press("Enter");
  await page.clock.runFor(250);
  await expect(notice).toHaveCount(0);
  await page.evaluate(() => document.exitFullscreen());
  await expect.poll(() => page.evaluate(() => !!document.fullscreenElement)).toBe(false);
  await expect(page.locator("[data-sonner-toast]")).toHaveCount(0);
});
