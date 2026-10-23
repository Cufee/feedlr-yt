import { describe, expect, it } from "vitest";
import { safeReturn } from "./passkeys";

describe("safeReturn", () => {
  it("preserves a local route, filters, pagination, and fragment", () => {
    expect(safeReturn("/app/recent?source=podcasts&page=2#episode")).toBe(
      "/app/recent?source=podcasts&page=2#episode",
    );
  });
  it.each([
    undefined,
    null,
    "https://example.com",
    "//example.com/path",
    "/\\example.com/path",
    "/\t/example.com",
    "javascript:alert(1)",
    123,
  ])("rejects external or ambiguous return destinations: %s", (destination) => {
    expect(safeReturn(destination)).toBe("/app");
  });
});
