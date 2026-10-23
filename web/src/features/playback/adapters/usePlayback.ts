import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import type { MediaDetail } from "@/lib/api/types";
import { PodcastSession, VideoSession } from "./runtime";

export function useVideoPlayback(detail: MediaDetail) {
  const [session] = useState(() => new VideoSession(detail));
  const root = useRef<HTMLFieldSetElement>(null);
  useEffect(() => {
    if (root.current) {
      return session.connect(root.current);
    }
  }, [session]);
  const snapshot = useSyncExternalStore(
    session.subscribe,
    session.getSnapshot,
    session.getSnapshot,
  );
  return { root, snapshot, actions: session };
}
export function usePodcastPlayback(detail: MediaDetail) {
  const [session] = useState(() => new PodcastSession(detail));
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (root.current) {
      return session.connect(root.current);
    }
  }, [session]);
  const snapshot = useSyncExternalStore(
    session.subscribe,
    session.getSnapshot,
    session.getSnapshot,
  );
  return { root, snapshot, actions: session };
}

function subscribeFullscreen(listener: () => void) {
  document.addEventListener("fullscreenchange", listener);
  return () => document.removeEventListener("fullscreenchange", listener);
}
const fullscreenSnapshot = () => document.fullscreenElement as HTMLElement | null;
const noFullscreen = () => null;
export function useFullscreenElement() {
  return useSyncExternalStore(subscribeFullscreen, fullscreenSnapshot, noFullscreen);
}
