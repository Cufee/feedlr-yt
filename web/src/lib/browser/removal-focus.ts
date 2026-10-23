// MutationObserver runs after the list's DOM update, including a deferred Query
// refetch. Do not steal focus if the user moved elsewhere while the action saved.
export function preserveRemovalFocus(element: Element | null) {
  if (!element?.parentElement) {
    return () => undefined;
  }
  const parent = element.parentElement;
  const neighbors = [element.nextElementSibling, element.previousElementSibling];
  const observer = new MutationObserver(() => {
    if (element.isConnected) {
      return;
    }
    stop();
    if (document.activeElement !== document.body && document.activeElement?.isConnected) {
      return;
    }
    const selector = 'a[href], button:not(:disabled), [tabindex="0"]';
    const target =
      neighbors
        .filter((neighbor) => neighbor?.isConnected)
        .map((neighbor) => neighbor?.querySelector<HTMLElement>(selector))
        .find(Boolean) ?? document.querySelector<HTMLElement>("#page-title");
    target?.focus({ preventScroll: true });
  });
  const timeout = window.setTimeout(stop, 30_000);
  function stop() {
    observer.disconnect();
    window.clearTimeout(timeout);
  }
  observer.observe(parent, { childList: true, subtree: true });
  return stop;
}
