export function parseDate(value: string | Date): Date | null {
  const d = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(d.getTime())) return null;
  return d;
}

export function formatAbsolute(value: string | Date): string {
  const d = parseDate(value);
  if (d == null) return typeof value === "string" ? value : "";
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

export function formatRelative(value: string | Date, now = new Date()): string {
  const d = parseDate(value);
  if (d == null) return typeof value === "string" ? value : "";
  const diffMs = d.getTime() - now.getTime();
  const absSec = Math.abs(Math.round(diffMs / 1000));
  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
  const div = (seconds: number) => Math.round(diffMs / seconds / 1000);
  if (absSec < 45) return "just now";
  if (absSec < 45 * 60) return rtf.format(div(60), "minute");
  if (absSec < 36 * 3600) return rtf.format(div(3600), "hour");
  if (absSec < 10 * 86400) return rtf.format(div(86400), "day");
  if (absSec < 45 * 86400) return rtf.format(div(86400 * 7), "week");
  if (absSec < 365 * 86400) return rtf.format(div(86400 * 30), "month");
  return rtf.format(div(86400 * 365), "year");
}
