import { describe, expect, it } from "vitest";
import { openShortcutForPlatform } from "./open-shortcut";

describe("the Open YouTube keyboard hint", () => {
  it.each(["Linux x86_64", "Linux armv8l", "Win32", "Windows", "", "Android"])(
    "uses Control on %s",
    (platform) => {
      expect(openShortcutForPlatform(platform)).toEqual({
        label: "Ctrl + K",
        ariaKeyShortcuts: "Control+K",
      });
    },
  );
  it.each(["MacIntel", "macOS", "iPhone", "iPad", "iPod"])("uses Command on %s", (platform) => {
    expect(openShortcutForPlatform(platform)).toEqual({
      label: "⌘ K",
      ariaKeyShortcuts: "Meta+K",
    });
  });
  it("recognizes Apple devices when only a user agent is available", () => {
    expect(
      openShortcutForPlatform("", "Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X)"),
    ).toHaveProperty("ariaKeyShortcuts", "Meta+K");
  });
});
