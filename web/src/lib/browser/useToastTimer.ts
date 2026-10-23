import { type FocusEvent, useEffect, useEffectEvent, useRef } from "react";

// Each pause reason owns its hold; leaving hover must not release keyboard focus.
export class ToastTimer {
  private remaining: number;
  private started = 0;
  private timeout?: ReturnType<typeof setTimeout>;
  private holds = new Set<string>();
  private disposed = false;
  constructor(
    duration: number,
    private readonly expire: () => void,
  ) {
    this.remaining = duration;
  }
  pause(reason: string) {
    this.holds.add(reason);
    if (this.timeout !== undefined) {
      clearTimeout(this.timeout);
      this.timeout = undefined;
      this.remaining = Math.max(0, this.remaining - (performance.now() - this.started));
    }
  }
  resume(reason: string) {
    this.holds.delete(reason);
    if (this.disposed || this.holds.size > 0 || this.timeout !== undefined || this.remaining <= 0) {
      return;
    }
    this.started = performance.now();
    this.timeout = setTimeout(() => {
      this.timeout = undefined;
      this.remaining = 0;
      this.expire();
    }, this.remaining);
  }
  dispose() {
    this.disposed = true;
    clearTimeout(this.timeout);
    this.timeout = undefined;
  }
}

export function useToastTimer(duration: number, onDismiss: () => void) {
  const timer = useRef<ToastTimer | null>(null);
  const dismiss = useEffectEvent(onDismiss);
  useEffect(() => {
    const lifetime = new ToastTimer(duration, () => dismiss());
    timer.current = lifetime;
    const visibility = () => {
      if (document.hidden) {
        lifetime.pause("visibility");
      } else {
        lifetime.resume("visibility");
      }
    };
    document.addEventListener("visibilitychange", visibility);
    visibility();
    return () => {
      document.removeEventListener("visibilitychange", visibility);
      lifetime.dispose();
      timer.current = null;
    };
  }, [duration]);
  return {
    onMouseEnter: () => timer.current?.pause("hover"),
    onMouseLeave: () => timer.current?.resume("hover"),
    onFocusCapture: () => timer.current?.pause("focus"),
    onBlurCapture: (event: FocusEvent<HTMLElement>) => {
      if (!event.currentTarget.contains(event.relatedTarget)) {
        timer.current?.resume("focus");
      }
    },
  };
}
