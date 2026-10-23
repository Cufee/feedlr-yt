import { useEffect, useState, useSyncExternalStore } from "react";
import { AddPasskeySession, PasskeySession } from "./passkey-session";

export function usePasskeys(autofill: boolean, onAuthenticated: () => void) {
  const [session] = useState(() => new PasskeySession());
  useEffect(() => {
    const disconnect = session.connect(autofill, onAuthenticated);
    const stop = () => session.reset();
    const resume = (event: PageTransitionEvent) => {
      if (event.persisted) {
        session.resumeAutofill();
      }
    };
    window.addEventListener("pagehide", stop);
    window.addEventListener("pageshow", resume);
    return () => {
      window.removeEventListener("pagehide", stop);
      window.removeEventListener("pageshow", resume);
      disconnect();
    };
  }, [autofill, onAuthenticated, session]);
  const snapshot = useSyncExternalStore(
    session.subscribe,
    session.getSnapshot,
    session.getSnapshot,
  );
  return {
    snapshot,
    login: session.login,
    register: session.register,
    cancel: session.cancel,
    reset: session.reset,
  };
}

export function useAddPasskey() {
  const [session] = useState(() => new AddPasskeySession());
  useEffect(() => {
    window.addEventListener("pagehide", session.cancel);
    return () => {
      window.removeEventListener("pagehide", session.cancel);
      session.cancel();
    };
  }, [session]);
  return session.add;
}
