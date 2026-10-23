export function safeReturn(value: unknown, fallback = "/app") {
  if (
    typeof value !== "string" ||
    !value.startsWith("/") ||
    value.startsWith("//") ||
    value.includes("\\")
  ) {
    return fallback;
  }
  try {
    const parsed = new URL(value, "https://feedlr.local");
    return parsed.origin === "https://feedlr.local"
      ? `${parsed.pathname}${parsed.search}${parsed.hash}`
      : fallback;
  } catch {
    return fallback;
  }
}
