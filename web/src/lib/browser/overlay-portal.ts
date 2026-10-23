import { useLayoutEffect } from "react";

// Headless UI Dialog intentionally forces its portal root. Relocate that one
// shared root, rather than nesting another portal that Dialog would ignore.
const targets = new Map<symbol, HTMLElement | null>();
const isolated = new Map<HTMLElement, { inert: boolean; hidden: string | null }>();

function restoreIsolation(
  element: HTMLElement,
  previous: { inert: boolean; hidden: string | null },
) {
  // Headless UI can also own a sibling. Do not overwrite its later cleanup.
  if (element.inert) {
    element.inert = previous.inert;
  }
  if (element.getAttribute("aria-hidden") === "true") {
    if (previous.hidden === null) {
      element.removeAttribute("aria-hidden");
    } else {
      element.setAttribute("aria-hidden", previous.hidden);
    }
  }
}

function placePortal() {
  const root = document.getElementById("headlessui-portal-root");
  const requested = Array.from(targets.values()).at(-1);
  const target =
    targets.size === 0
      ? document.body
      : requested?.isConnected
        ? requested
        : (document.fullscreenElement ?? document.body);
  if (root && root.parentElement !== target && !root.contains(target)) {
    target.append(root);
  }
  const blocked = new Set<HTMLElement>();
  if (root && targets.size > 0 && target !== document.body) {
    let branch: HTMLElement = root;
    while (branch.parentElement && branch.parentElement !== document.body) {
      const ancestor = branch.parentElement;
      // Headless UI hides the body child containing its original React tree.
      // Once the portal moves inside that child, lift that inherited boundary
      // and move isolation to siblings on the path to the modal instead.
      if (ancestor.parentElement === document.body) {
        if (ancestor.inert) {
          ancestor.inert = false;
        }
        if (ancestor.getAttribute("aria-hidden") === "true") {
          ancestor.removeAttribute("aria-hidden");
        }
      }
      for (const sibling of ancestor.children) {
        if (sibling instanceof HTMLElement && sibling !== branch) {
          blocked.add(sibling);
        }
      }
      branch = ancestor;
    }
  }
  for (const [element, previous] of isolated) {
    if (!blocked.has(element)) {
      restoreIsolation(element, previous);
      isolated.delete(element);
    }
  }
  for (const element of blocked) {
    if (!isolated.has(element)) {
      isolated.set(element, { inert: element.inert, hidden: element.getAttribute("aria-hidden") });
    }
    if (!element.inert) {
      element.inert = true;
    }
    if (element.getAttribute("aria-hidden") !== "true") {
      element.setAttribute("aria-hidden", "true");
    }
  }
}

export function useOverlayPortal(open: boolean, target?: HTMLElement | null) {
  useLayoutEffect(() => {
    if (!open) {
      return;
    }
    const key = Symbol("overlay");
    targets.set(key, target ?? null);
    placePortal();
    const observer = new MutationObserver(placePortal);
    observer.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ["inert", "aria-hidden"],
    });
    document.addEventListener("fullscreenchange", placePortal);
    return () => {
      targets.delete(key);
      observer.disconnect();
      document.removeEventListener("fullscreenchange", placePortal);
      placePortal();
    };
  }, [open, target]);
}
