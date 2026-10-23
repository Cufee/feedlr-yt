import { expect, type Page, test } from "@playwright/test";

async function noticeFixture(page: Page) {
  await page.clock.install({ time: new Date("2026-10-22T11:00:00Z") });
  await page.goto("/iframe.html?id=playback-player--playback-notices&viewMode=story");
  await expect(page.getByRole("region", { name: "Video player" })).toBeVisible();
  await page.clock.pauseAt(new Date("2026-10-22T12:00:00Z"));
  return page.locator(".playback-video-status");
}

test("sponsor notices expire while playback updates and repeated skips get a new lifetime", async ({
  page,
}) => {
  const notice = await noticeFixture(page);
  for (let skip = 0; skip < 2; skip++) {
    await page.getByRole("button", { name: "Skip sponsor" }).click();
    await expect(notice).toHaveText("SponsorBlock skipped a video segment");
    await page.mouse.move(0, 0);
    await page.clock.fastForward(5_000);
    await expect(notice).toBeVisible();
    await page.getByRole("button", { name: "Advance playback" }).click();
    await page.clock.fastForward(1_100);
    await expect(notice).toHaveCount(0);
  }
});

test("a replacement notice starts its own lifetime and its close control dismisses it", async ({
  page,
}) => {
  const notice = await noticeFixture(page);
  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await expect(notice).toBeVisible();
  await page.mouse.move(0, 0);
  await page.clock.fastForward(5_000);
  await page.getByRole("button", { name: "Use YouTube player" }).click();
  await expect(notice).toHaveText("Switched to YouTube player");
  await page.clock.fastForward(1_100);
  await expect(notice).toBeVisible();
  await page.clock.fastForward(5_000);
  await expect(notice).toHaveCount(0);

  for (let repeat = 0; repeat < 2; repeat++) {
    await page.getByRole("button", { name: "Use YouTube player" }).click();
    await expect(notice).toHaveText("Switched to YouTube player");
    const close = notice.getByRole("button", { name: "Dismiss notification" });
    await expect(close).toBeInViewport();
    await close.focus();
    await close.press("Enter");
    await expect(notice).toHaveCount(0);
  }
});

test("playback notice lifetime pauses until both hover and keyboard focus leave", async ({
  page,
}) => {
  const notice = await noticeFixture(page);
  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await notice.hover();
  await page.clock.fastForward(12_000);
  await expect(notice).toBeVisible();
  await notice.getByRole("button", { name: "Dismiss notification" }).focus();
  await page.mouse.move(0, 0);
  await page.clock.fastForward(12_000);
  await expect(notice).toBeVisible();
  await page.getByRole("button", { name: "Advance playback" }).focus();
  await page.clock.fastForward(6_100);
  await expect(notice).toHaveCount(0);
});

test("TV progress keeps notice dismissal and the original expiration time", async ({ page }) => {
  const notice = await noticeFixture(page);
  const toggleTV = page.getByRole("button", { name: "Toggle TV progress" });
  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await notice.getByRole("button", { name: "Dismiss notification" }).click();
  await expect(notice).toHaveCount(0);
  await toggleTV.click();
  await expect(notice).toHaveText("Sending to TV…");
  await toggleTV.click();
  await expect(notice).toHaveCount(0);

  await page.getByRole("button", { name: "Skip sponsor" }).click();
  await expect(notice).toBeVisible();
  await page.mouse.move(0, 0);
  await page.clock.fastForward(5_000);
  await toggleTV.click();
  await expect(notice).toHaveText("Sending to TV…");
  await page.clock.fastForward(2_000);
  await expect(notice).toHaveText("Sending to TV…");
  await toggleTV.click();
  await expect(notice).toHaveCount(0);
});
