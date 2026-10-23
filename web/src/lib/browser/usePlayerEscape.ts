import { useEffect, useEffectEvent } from "react";

const overlays = [
  '[role="dialog"]',
  '[role="alertdialog"]',
  '[role="menu"]',
  '[role="listbox"]',
  "dialog[open]",
  "[popover]",
  ".shaka-overflow-menu:not(.shaka-hidden)",
  ".shaka-settings-menu:not(.shaka-hidden)",
].join(",");

function overlayIsVisible(element: HTMLElement) {
  // The shared dialog root has no layout box; its fixed panel does.
  const candidates = [element, ...element.querySelectorAll<HTMLElement>(".ui-dialog-panel")];
  return candidates.some((candidate) => {
    const style = getComputedStyle(candidate);
    return candidate.getClientRects().length > 0 && style.visibility !== "hidden";
  });
}

function overlayIsOpen() {
  return Array.from(document.querySelectorAll<HTMLElement>(overlays)).some(overlayIsVisible);
}

export function usePlayerEscape(onClose: () => void) {
  const close = useEffectEvent(onClose);
  useEffect(() => {
    const abort = new AbortController();
    const handledByOverlay = new WeakSet<KeyboardEvent>();
    window.addEventListener(
      "keydown",
      (event) => {
        // Native controls can hide their menus before the document handler runs.
        if (event.key === "Escape" && (document.fullscreenElement || overlayIsOpen())) {
          handledByOverlay.add(event);
        }
      },
      { capture: true, signal: abort.signal },
    );
    document.addEventListener(
      "keydown",
      (event) => {
        if (
          event.key !== "Escape" ||
          event.defaultPrevented ||
          event.repeat ||
          event.isComposing ||
          event.altKey ||
          event.ctrlKey ||
          event.metaKey ||
          event.shiftKey ||
          handledByOverlay.has(event) ||
          document.fullscreenElement
        ) {
          return;
        }
        const target = event.target instanceof HTMLElement ? event.target : null;
        if (
          target?.isContentEditable ||
          target?.closest(
            'textarea, select, input:not([type="range"]):not([type="checkbox"]):not([type="radio"]):not([type="button"]):not([type="submit"]):not([type="reset"])',
          ) ||
          overlayIsOpen()
        ) {
          return;
        }
        event.preventDefault();
        close();
      },
      { signal: abort.signal },
    );
    return () => abort.abort();
  }, []);
}
