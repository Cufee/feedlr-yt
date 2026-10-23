export function openShortcutForPlatform(platform: string, userAgent = "") {
  const apple = /mac|iphone|ipad|ipod/i.test(platform || userAgent);
  return {
    label: apple ? "⌘ K" : "Ctrl + K",
    ariaKeyShortcuts: apple ? "Meta+K" : "Control+K",
  };
}
