import { useLayoutEffect, useState } from "react";

export function useFullscreenPortal() {
  const [portal] = useState(() => document.createElement("div"));
  useLayoutEffect(() => {
    // Move the same portal so entering fullscreen keeps active toasts and timers.
    const place = () => (document.fullscreenElement ?? document.body).append(portal);
    place();
    document.addEventListener("fullscreenchange", place);
    return () => {
      document.removeEventListener("fullscreenchange", place);
      portal.remove();
    };
  }, [portal]);
  return portal;
}
