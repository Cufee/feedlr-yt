const relative = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
const referenceTime = Date.now();
export function relativeDate(value: string) {
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) {
    return "";
  }
  const days = Math.floor((date.getTime() - referenceTime) / 86_400_000);
  if (Math.abs(days) > 365) {
    return relative.format(Math.round(days / 365), "year");
  }
  if (Math.abs(days) > 30) {
    return relative.format(Math.round(days / 30), "month");
  }
  return relative.format(days, "day");
}
export function duration(value: number | null | undefined) {
  if (!value || value < 0) {
    return "";
  }
  const seconds = Math.floor(value);
  const minutes = Math.floor(seconds / 60);
  return minutes >= 60
    ? `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`
    : `${minutes}:${String(seconds % 60).padStart(2, "0")}`;
}
export function dateLabel(value?: string) {
  if (!value || new Date(value).getFullYear() < 1971) {
    return "Never";
  }
  return new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value),
  );
}
