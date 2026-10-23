import { expect, test } from "@playwright/test";

function contrast(first: string, second: string) {
  function luminance(color: string) {
    const channels = color
      .match(/[\d.]+/g)
      ?.slice(0, 3)
      .map(Number);
    if (channels?.length !== 3) {
      throw new Error(`Expected an RGB color: ${color}`);
    }
    const linear = channels.map((value) => {
      const channel = value / 255;
      return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
    });
    return (linear[0] ?? 0) * 0.2126 + (linear[1] ?? 0) * 0.7152 + (linear[2] ?? 0) * 0.0722;
  }
  const a = luminance(first),
    b = luminance(second);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

async function decodeImages(page: import("@playwright/test").Page) {
  await page.evaluate(async () => {
    for (const image of document.images) {
      image.loading = "eager";
    }
    await Promise.all(
      Array.from(document.images)
        .filter((image) => image.currentSrc)
        .map((image) => image.decode()),
    );
  });
}

test("media progress preserves the subdued legacy treatment and duration spacing", async ({
  page,
}, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=media-progress--${viewport}&viewMode=story`);
  const root = page.locator("#storybook-root");
  await expect(root.locator(".media-card")).toHaveCount(5);
  for (const width of info.project.name === "phone" ? [390, 320] : [1440]) {
    await page.setViewportSize({ width, height: info.project.name === "phone" ? 844 : 1000 });
    for (const card of await root.locator(".media-card:has(.media-progress)").all()) {
      const track = card.locator(".media-progress");
      const bounds = await track.boundingBox();
      const duration = await card.locator(".media-duration").boundingBox();
      expect(
        await card
          .locator(".media-duration")
          .evaluate((element) => Number.parseFloat(getComputedStyle(element).fontSize)),
      ).toBeGreaterThanOrEqual(13);
      if (!bounds || !duration) {
        throw new Error("Progress or duration is missing");
      }
      // 811f26c: .55rem total height, .82rem horizontal/.62rem bottom insets,
      // .42rem duration gap, with a translucent inner border rather than an outer ring.
      expect(bounds.height).toBeCloseTo(8.8, 1);
      expect(bounds.y - duration.y - duration.height).toBeCloseTo(6.72, 1);
      const treatment = await track.evaluate((element) => {
        const style = getComputedStyle(element);
        const fill = getComputedStyle(element.firstElementChild ?? element);
        return {
          left: Number.parseFloat(style.left),
          right: Number.parseFloat(style.right),
          bottom: Number.parseFloat(style.bottom),
          track: style.backgroundColor,
          border: style.borderTopColor,
          borderWidth: style.borderTopWidth,
          outline: style.outlineStyle,
          shadow: style.boxShadow,
          blur: style.backdropFilter,
          fill: fill.backgroundImage,
          fillBorder: fill.borderTopColor,
          fillHeight: Number.parseFloat(fill.height),
          fillMinimum: fill.minWidth,
          fillShadow: fill.boxShadow,
        };
      });
      expect(treatment.left).toBeCloseTo(13.12, 1);
      expect(treatment.right).toBeCloseTo(13.12, 1);
      expect(treatment.bottom).toBeCloseTo(9.92, 1);
      expect(treatment.track).toBe("rgba(0, 0, 0, 0.36)");
      expect(treatment.border).toBe("rgba(255, 255, 255, 0.2)");
      expect(treatment.borderWidth).toBe("1px");
      expect(treatment.outline).toBe("none");
      expect(treatment.shadow).toBe("rgba(255, 255, 255, 0.1) 0px 1px 1px 0px inset");
      expect(treatment.blur).toBe("blur(4px)");
      expect(treatment.fill).toBe(
        "linear-gradient(to right, oklch(0.74 0.11 312 / 0.62), oklch(0.68 0.13 312 / 0.68))",
      );
      expect(treatment.fillBorder).toBe("oklch(0.74 0.11 312 / 0.32)");
      expect(treatment.fillHeight).toBeCloseTo(6.8, 1);
      expect(treatment.fillMinimum).toBe("0px");
      expect(treatment.fillShadow).toBe("rgba(177, 128, 230, 0.2) 0px 0px 6px 0px");
    }
    await expect(root.locator('[data-media-id="unplayed"] .media-progress')).toHaveCount(0);
    const hidden = root.locator('[data-media-id="hidden"]');
    await expect(hidden.getByText("Hidden", { exact: true })).toBeVisible();
    await expect(hidden.getByRole("progressbar")).toHaveCount(0);
    const completed = root.locator('[data-media-id="completed-without-position"]');
    await expect(completed.getByRole("progressbar", { name: "Completed" })).toHaveAttribute(
      "aria-valuenow",
      "100",
    );
    await expect(completed.getByText("Watched", { exact: true })).toHaveCount(1);
    await expect(completed.locator(".media-artwork-badges")).toBeVisible();
    const fullTrack = await completed.locator(".media-progress").boundingBox();
    const fullFill = await completed.locator(".media-progress span").boundingBox();
    expect(Math.abs((fullFill?.width ?? 0) - (fullTrack?.width ?? 0) + 2)).toBeLessThan(0.1);
    const partial = root.locator('[data-media-id="progress-preview"]');
    await expect(partial.getByRole("progressbar", { name: "Played" })).toHaveAttribute(
      "aria-valuenow",
      "35",
    );
    const partialTrack = await partial.locator(".media-progress").boundingBox();
    const partialFill = await partial.locator(".media-progress span").boundingBox();
    expect(
      Math.abs((partialFill?.width ?? 0) - ((partialTrack?.width ?? 0) - 2) * 0.35),
    ).toBeLessThan(0.1);
    const tiny = root
      .locator('[data-media-id="just-started"]')
      .getByRole("progressbar", { name: "Played" });
    await expect(tiny).toHaveAttribute("aria-valuenow", "0");
    const tinyFill = (await tiny.locator("span").boundingBox())?.width ?? 0;
    expect(tinyFill).toBeGreaterThan(0);
    expect(tinyFill).toBeLessThan(3);
    await decodeImages(page);
    await page.screenshot({
      path: info.outputPath(`progress-${width}.png`),
      fullPage: true,
      animations: "disabled",
    });
  }
});

test("Open YouTube advertises and opens the desktop platform keyboard shortcut", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "desktop", "The keyboard hint is a desktop control.");
  await page.setViewportSize({ width: 1440, height: 1000 });
  for (const platform of [
    {
      name: "Linux",
      legacy: "Linux x86_64",
      hint: "Ctrl + K",
      aria: "Control+K",
      keys: "Control+k",
    },
    { name: "macOS", legacy: "MacIntel", hint: "⌘ K", aria: "Meta+K", keys: "Meta+k" },
  ]) {
    await page.addInitScript(({ name, legacy }) => {
      Object.defineProperty(navigator, "platform", { configurable: true, value: legacy });
      Object.defineProperty(navigator, "userAgentData", {
        configurable: true,
        value: { platform: name },
      });
    }, platform);
    await page.goto("/iframe.html?id=app-screens--feed-desktop&viewMode=story");
    const button = page.getByRole("button", { name: "Open YouTube", exact: true });
    await expect(button).toHaveAttribute("aria-keyshortcuts", platform.aria);
    await expect(button.locator(".header-shortcut")).toHaveText(platform.hint);
    await expect(button.locator(".header-shortcut")).toHaveAttribute("aria-hidden", "true");
    await page.keyboard.press(platform.keys);
    const dialog = page.getByRole("dialog", { name: "Open a video" });
    await expect(dialog.locator(".ui-dialog-panel")).toBeVisible();
    await expect(dialog.getByRole("textbox", { name: "YouTube link" })).toBeFocused();
  }
});

test("playlist completion has a contrasting persistent track", async ({ page }, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=app-screens--playlist-${viewport}&viewMode=story`);
  const progress = page.locator(".playlist-summary").getByRole("progressbar");
  await expect(progress).toBeVisible();
  expect((await progress.boundingBox())?.height).toBeGreaterThanOrEqual(8);
  const colors = await progress.evaluate((element) => ({
    track: getComputedStyle(element).backgroundColor,
    fill: getComputedStyle(element.firstElementChild ?? element).backgroundColor,
  }));
  expect(contrast(colors.fill, colors.track)).toBeGreaterThanOrEqual(4.5);
  await decodeImages(page);
  await page.screenshot({
    path: info.outputPath(`playlist-progress-${viewport}.png`),
    fullPage: true,
    animations: "disabled",
  });
});

test("podcast progress remains aligned after increasing its visible track", async ({
  page,
}, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=playback-screens--podcast-${viewport}&viewMode=story`);
  await expect(page.getByRole("slider", { name: "Episode progress" })).toHaveAttribute("max", "60");
  await page.locator("audio").evaluate((audio: HTMLMediaElement) => audio.pause());
  for (const width of info.project.name === "phone" ? [390, 320, 844] : [1440]) {
    await page.setViewportSize({ width, height: width === 844 ? 390 : 1000 });
    const bounds = await page.locator(".playback-range-track").boundingBox();
    const marker = await page.locator(".playback-segment-markers span").first().boundingBox();
    if (!bounds || !marker) {
      throw new Error("The episode track or sponsor marker is missing");
    }
    expect(bounds.height).toBeGreaterThanOrEqual(8);
    expect(Math.abs(marker.x - bounds.x - bounds.width / 6)).toBeLessThan(1);
    expect(Math.abs(marker.width - bounds.width / 6)).toBeLessThan(1);
    expect(Math.abs(marker.y + marker.height / 2 - bounds.y - bounds.height / 2)).toBeLessThan(1);
    const colors = await page.locator(".playback-range-track").evaluate((element) => ({
      track: getComputedStyle(element).backgroundColor,
      fill: getComputedStyle(element.querySelector(".playback-range-progress") ?? element)
        .backgroundColor,
    }));
    expect(contrast(colors.fill, colors.track)).toBeGreaterThanOrEqual(4.5);
    await page.screenshot({
      path: info.outputPath(`podcast-progress-${width}.png`),
      fullPage: true,
      animations: "disabled",
    });
  }
});

test("toast glass reveals content while preserving text and severity contrast", async ({
  page,
}, info) => {
  const viewport = info.project.name === "phone" ? "mobile" : "desktop";
  await page.goto(`/iframe.html?id=media-progress--glass-toasts-${viewport}&viewMode=story`);
  const toast = page.getByRole("alert");
  await expect(toast).toBeVisible();
  const style = await toast.evaluate((element) => ({
    background: getComputedStyle(element).backgroundColor,
    filter: getComputedStyle(element).backdropFilter,
    text: getComputedStyle(element.querySelector(".ui-toast-copy") ?? element).color,
    icon: getComputedStyle(element.querySelector(".ui-toast-icon") ?? element).color,
  }));
  const rgba = style.background.match(/[\d.]+/g)?.map(Number);
  expect(rgba?.[3]).toBeGreaterThanOrEqual(0.65);
  expect(rgba?.[3]).toBeLessThanOrEqual(0.8);
  expect(style.filter).toContain("blur(20px)");
  if (!rgba) {
    throw new Error("Toast background is not translucent");
  }
  const whiteComposite = `rgb(${rgba
    .slice(0, 3)
    .map((channel) => Math.round(channel * (rgba[3] ?? 1) + 255 * (1 - (rgba[3] ?? 1))))
    .join(", ")})`;
  expect(contrast(style.text, whiteComposite)).toBeGreaterThanOrEqual(4.5);
  expect(contrast(style.icon, whiteComposite)).toBeGreaterThanOrEqual(3);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({
    path: info.outputPath(`glass-toasts-${viewport}.png`),
    fullPage: true,
    animations: "disabled",
  });
  await toast.getByRole("button", { name: "Dismiss notification" }).click();
  await expect(toast).toHaveCount(0);
});
