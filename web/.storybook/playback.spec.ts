import { expect, test } from "@playwright/test";

for (const kind of ["video", "podcast"] as const) {
  for (const size of ["default", "narrow", "landscape"] as const) {
    test(`${kind} screen keeps compact chrome and usable controls at ${size}`, async ({
      page,
    }, testInfo) => {
      if (size !== "default" && testInfo.project.name !== "phone") {
        test.skip();
      }
      if (size === "narrow") {
        await page.setViewportSize({ width: 320, height: 740 });
      } else if (size === "landscape") {
        await page.setViewportSize({ width: 844, height: 390 });
      }
      const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
      await page.goto(`/iframe.html?id=playback-screens--${kind}-${viewport}&viewMode=story`);
      await expect(page.locator(".playback-toolbar")).toBeVisible();
      const media = page.locator(kind === "video" ? "video" : "audio");
      await expect
        .poll(() => media.evaluate((element: HTMLMediaElement) => element.readyState))
        .toBeGreaterThanOrEqual(2);
      await media.evaluate((element: HTMLMediaElement) => element.pause());
      const geometry = await page.evaluate(() => {
        const toolbar = document.querySelector(".playback-toolbar");
        const identity = document.querySelector(".playback-identity");
        const trigger = document.querySelector(".playback-menu-trigger");
        if (!toolbar || !identity || !trigger) {
          throw new Error("The playback header is incomplete");
        }
        return {
          header: toolbar.getBoundingClientRect().toJSON(),
          identity: identity.getBoundingClientRect().toJSON(),
          trigger: trigger.getBoundingClientRect().toJSON(),
          width: window.innerWidth,
          height: window.innerHeight,
          scrollWidth: document.documentElement.scrollWidth,
          scrollHeight: document.documentElement.scrollHeight,
          background: getComputedStyle(toolbar).backgroundColor,
        };
      });
      expect(geometry.header.x).toBe(0);
      expect(geometry.header.y).toBe(0);
      expect(geometry.header.width).toBe(geometry.width);
      expect(geometry.header.height).toBeLessThanOrEqual(68);
      expect(geometry.background).toBe("rgb(0, 0, 0)");
      expect(
        Math.abs(
          geometry.identity.y +
            geometry.identity.height / 2 -
            geometry.trigger.y -
            geometry.trigger.height / 2,
        ),
      ).toBeLessThan(2);
      expect(geometry.scrollWidth).toBeLessThanOrEqual(geometry.width);
      await expect(page.getByRole("button", { name: "Player actions" })).toHaveCount(1);
      await expect(page.locator(".playback-toolbar .ui-badge")).toHaveCount(0);
      await expect(page.locator(".playback-toolbar button")).toHaveCount(1);
      if (kind === "video") {
        const stage = await page.locator(".playback-video-stage").boundingBox();
        expect(stage?.x).toBe(0);
        expect(stage?.width).toBe(geometry.width);
        expect(stage?.height).toBeGreaterThanOrEqual(geometry.height - geometry.header.height - 1);
        expect(geometry.scrollHeight).toBeLessThanOrEqual(geometry.height);
        await expect(page.locator(".shaka-controls-container")).toHaveCount(1);
        await page.mouse.move(geometry.width / 2, geometry.height - 40);
      } else {
        const controls = await page.locator(".playback-podcast-controls").boundingBox();
        const notes = await page.locator(".playback-notes").boundingBox();
        expect(controls?.y).toBeLessThan(notes?.y ?? 0);
        if (size !== "landscape") {
          const play = await page.getByRole("button", { name: "Play episode" }).boundingBox();
          expect((play?.y ?? 0) + (play?.height ?? 0)).toBeLessThan(geometry.height);
        }
        const skips = await page
          .locator(".playback-transport .ui-icon-button svg")
          .evaluateAll((icons) =>
            icons.map((icon) => {
              const glyph = icon.getBoundingClientRect();
              const count = icon.querySelector("text")?.getBoundingClientRect();
              if (!count) {
                throw new Error("The skip counter is missing");
              }
              return {
                glyphX: glyph.x + glyph.width / 2,
                countX: count.x + count.width / 2,
                glyphY: glyph.y + glyph.height / 2,
                countY: count.y + count.height / 2,
              };
            }),
          );
        for (const skip of skips) {
          expect(Math.abs(skip.glyphX - skip.countX)).toBeLessThan(1);
          expect(Math.abs(skip.glyphY - skip.countY)).toBeLessThan(3);
        }
      }
      await page.evaluate(async () =>
        Promise.all(
          Array.from(document.images)
            .filter((image) => image.currentSrc && getComputedStyle(image).visibility === "visible")
            .map((image) => image.decode()),
        ),
      );
      await page.screenshot({
        path: testInfo.outputPath(`${kind}-${viewport}-${size}.png`),
        fullPage: true,
        animations: "disabled",
      });
    });
  }
}

test("podcast seek track and sponsor markers share native thumb geometry", async ({
  page,
}, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=playback-screens--podcast-${viewport}&viewMode=story`);
  const progress = page.getByRole("slider", { name: "Episode progress" });
  await expect(progress).toHaveAttribute("max", "60");
  await page.locator("audio").evaluate((audio: HTMLMediaElement) => audio.pause());
  const sizes =
    testInfo.project.name === "phone"
      ? [
          { width: 390, height: 844 },
          { width: 320, height: 740 },
          { width: 844, height: 390 },
        ]
      : [{ width: 1440, height: 1000 }];
  for (const size of sizes) {
    await page.setViewportSize(size);
    const trackBox = await page.locator(".playback-range-track").boundingBox();
    const markerBox = await page.locator(".playback-segment-markers span").first().boundingBox();
    if (!trackBox || !markerBox) {
      throw new Error("The progress track or sponsor marker is missing");
    }
    // Fixture sponsor spans 10–20 seconds in a 60 second episode.
    expect(Math.abs(markerBox.x - trackBox.x - trackBox.width / 6)).toBeLessThan(1);
    expect(Math.abs(markerBox.width - trackBox.width / 6)).toBeLessThan(1);
    const rangeBox = await progress.boundingBox();
    expect(Math.abs(trackBox.x - (rangeBox?.x ?? 0) - 8)).toBeLessThan(1);
    expect(Math.abs(trackBox.width - (rangeBox?.width ?? 0) + 16)).toBeLessThan(1);
    for (const position of [0, 30, 60]) {
      await progress.focus();
      await page.keyboard.press(position === 0 ? "Home" : "End");
      for (let step = 0; step < (position === 30 ? 30 : 0); step++) {
        await page.keyboard.press("ArrowLeft");
      }
      await expect(progress).toHaveValue(String(position));
      const fill = await page.locator(".playback-range-progress").boundingBox();
      expect(Math.abs((fill?.width ?? 0) - (trackBox.width * position) / 60)).toBeLessThan(1);
    }
  }
});

test("playback actions remain in a keyboard menu and close returns through the SPA", async ({
  page,
}, testInfo) => {
  const viewport = testInfo.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=playback-screens--podcast-${viewport}&viewMode=story`);
  const actions = page.getByRole("button", { name: "Player actions" });
  await actions.focus();
  await page.keyboard.press("Enter");
  const save = page.getByRole("menuitemcheckbox", { name: "Save to Watch Later" });
  await expect(save).toHaveAttribute("data-focus", "");
  await expect(page.getByRole("menu")).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(actions).toBeFocused();
  await actions.click();
  await expect(
    page.getByRole("menuitemcheckbox", { name: "Remove from Watch Later" }),
  ).toBeChecked();
  await page.keyboard.press("Escape");
  await expect(actions).toBeFocused();
  await page.evaluate(() => {
    document.documentElement.dataset.playbackNavigation = "same-document";
  });
  await page.getByRole("link", { name: "Close player" }).click();
  await expect(page.getByRole("heading", { name: "Your feed", level: 1 })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-playback-navigation", "same-document");
  await expect(page.locator("audio")).toHaveCount(0);
});
