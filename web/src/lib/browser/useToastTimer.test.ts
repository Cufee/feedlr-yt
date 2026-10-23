import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ToastTimer } from "./useToastTimer";

beforeEach(() => vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "performance"] }));
afterEach(() => vi.useRealTimers());

it("dismisses once after the visible lifetime", () => {
  const expire = vi.fn();
  const timer = new ToastTimer(6_000, expire);
  timer.resume("visibility");
  vi.advanceTimersByTime(5_999);
  expect(expire).not.toHaveBeenCalled();
  vi.advanceTimersByTime(1);
  expect(expire).toHaveBeenCalledOnce();
  timer.resume("hover");
  vi.advanceTimersByTime(10_000);
  expect(expire).toHaveBeenCalledOnce();
});

it("keeps overlapping focus, hover and visibility holds until each is released", () => {
  const expire = vi.fn();
  const timer = new ToastTimer(6_000, expire);
  timer.resume("visibility");
  vi.advanceTimersByTime(2_000);
  timer.pause("hover");
  timer.pause("focus");
  timer.pause("visibility");
  vi.advanceTimersByTime(30_000);
  timer.resume("hover");
  timer.resume("visibility");
  vi.advanceTimersByTime(10_000);
  expect(expire).not.toHaveBeenCalled();
  timer.resume("focus");
  vi.advanceTimersByTime(3_999);
  expect(expire).not.toHaveBeenCalled();
  vi.advanceTimersByTime(1);
  expect(expire).toHaveBeenCalledOnce();
});

it("disposes the old lifetime before a replacement can start", () => {
  const oldExpire = vi.fn();
  const nextExpire = vi.fn();
  const previous = new ToastTimer(6_000, oldExpire);
  previous.resume("visibility");
  vi.advanceTimersByTime(5_000);
  previous.dispose();
  const next = new ToastTimer(6_000, nextExpire);
  next.resume("visibility");
  previous.resume("focus");
  vi.advanceTimersByTime(1_000);
  expect(oldExpire).not.toHaveBeenCalled();
  expect(nextExpire).not.toHaveBeenCalled();
  vi.advanceTimersByTime(5_000);
  expect(nextExpire).toHaveBeenCalledOnce();
});
