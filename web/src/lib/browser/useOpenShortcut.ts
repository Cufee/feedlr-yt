import { useEffect, useState } from "react";
import { openShortcutForPlatform } from "./open-shortcut";

export function useOpenShortcut(onOpen: () => void) {
  const [shortcut] = useState(() => {
    if (typeof navigator === "undefined") {
      return openShortcutForPlatform("");
    }
    const hints = navigator as Navigator & { userAgentData?: { platform?: string } };
    return openShortcutForPlatform(hints.userAgentData?.platform ?? "", navigator.userAgent);
  });
  useEffect(() => {
    const abort = new AbortController();
    document.addEventListener(
      "keydown",
      (event) => {
        if (
          event.defaultPrevented ||
          event.repeat ||
          event.altKey ||
          !(event.metaKey || event.ctrlKey) ||
          event.key.toLowerCase() !== "k"
        ) {
          return;
        }
        if (document.querySelector('[role="dialog"], dialog[open]')) {
          return;
        }
        event.preventDefault();
        onOpen();
      },
      { signal: abort.signal },
    );
    return () => abort.abort();
  }, [onOpen]);
  return shortcut;
}
