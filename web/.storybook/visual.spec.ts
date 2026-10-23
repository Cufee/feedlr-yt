import { expect, test } from "@playwright/test";

for (const story of [
  "foundation-controls--desktop",
  "foundation-feedback--desktop",
  "playback-player--podcast-keyboard-controls",
]) {
  test(`${story} reflows without overflow`, async ({ page }, testInfo) => {
    await page.goto(`/iframe.html?id=${story}&viewMode=story`);
    await expect(page.locator("#storybook-root")).not.toBeEmpty();
    await expect(page.locator("#storybook-root").getByRole("heading").first()).toBeVisible();
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
      .toBe(true);
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
  });
}
