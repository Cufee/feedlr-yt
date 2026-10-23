import { useEffect } from "react";

export function useOpenShortcut(onOpen: () => void) {
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
}
