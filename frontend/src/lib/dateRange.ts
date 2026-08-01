// Time-range model behind the Grafana-style <DateRangeFilter>.
//
// Two flavours of range, mirroring Grafana's time picker:
//   - a **quick range** (`preset`) — re-resolved from "now" every time it's
//     picked: Today, Yesterday, This month, Last 30 days, …
//   - an **absolute range** (`preset: "custom"`) — pinned to two wall-clock
//     instants the user typed or picked on the calendar, kept in `customFrom` /
//     `customTo` as `YYYY-MM-DD HH:mm:ss` (local time).
//
// Either way the range always carries resolved `fromUnix` / `toUnix` (seconds),
// so every caller hands those straight to a List / analytics RPC and never has
// to know which flavour it is. Day-aligned ranges are **[from, to)** — the end
// is the start of the next day, matching how the backend filters.

export type RangePreset =
  // calendar-relative
  | "today"
  | "yesterday"
  | "dayBeforeYesterday"
  | "thisDayLastWeek"
  | "thisWeek"
  | "lastWeek"
  | "thisMonth"
  | "lastMonth"
  | "thisYear"
  | "lastYear"
  | "ytd"
  // rolling window ending now
  | "15m"
  | "1h"
  | "6h"
  | "12h"
  | "24h"
  // rolling window of whole days ending at end-of-today
  | "7d"
  | "30d"
  | "90d"
  | "6M"
  | "1y"
  // absolute
  | "custom";

export type DateRange = {
  preset: RangePreset;
  fromUnix: number;
  toUnix: number;
  /** Absolute bounds, only when preset === "custom". `YYYY-MM-DD HH:mm:ss`. */
  customFrom?: string;
  customTo?: string;
};

// Order the quick-range list renders in. Labels live at `analytics.range.<preset>`.
export const QUICK_RANGES: readonly RangePreset[] = [
  "today",
  "yesterday",
  "dayBeforeYesterday",
  "thisDayLastWeek",
  "thisWeek",
  "lastWeek",
  "thisMonth",
  "lastMonth",
  "thisYear",
  "lastYear",
  "ytd",
  "15m",
  "1h",
  "6h",
  "12h",
  "24h",
  "7d",
  "30d",
  "90d",
  "6M",
  "1y",
];

// --- formatting / parsing (local time, no date library) --------------------

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

/** `2026-07-30 23:59:59` — the absolute-input format. */
export function formatAbsolute(d: Date): string {
  return `${formatDateOnly(d)} ${formatTimeOnly(d)}`;
}

/** `2026-07-30` — the calendar's value format (shared with <DatePickerField>). */
export function formatDateOnly(d: Date): string {
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}

/** `23:59:59`. */
export function formatTimeOnly(d: Date): string {
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`;
}

const ABSOLUTE_RE = /^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{1,2}):(\d{2})(?::(\d{2}))?)?$/;

/**
 * Parse `YYYY-MM-DD[ HH:mm[:ss]]` as LOCAL time. Returns null for anything
 * malformed or impossible (`2026-02-31`, which the Date constructor would
 * silently roll over to March).
 */
export function parseAbsolute(s: string | null | undefined): Date | null {
  if (!s) return null;
  const m = ABSOLUTE_RE.exec(s.trim());
  if (!m) return null;
  const [, y, mo, d, hh, mi, ss] = m;
  const month = Number(mo) - 1;
  const day = Number(d);
  const hours = Number(hh ?? 0);
  const minutes = Number(mi ?? 0);
  const seconds = Number(ss ?? 0);
  if (month > 11 || day > 31 || hours > 23 || minutes > 59 || seconds > 59) return null;
  const date = new Date(Number(y), month, day, hours, minutes, seconds, 0);
  if (date.getMonth() !== month || date.getDate() !== day) return null;
  return date;
}

/**
 * Time-of-day of an absolute string, or `fallback` when it can't be parsed.
 * Used to keep the typed time while the calendar changes only the day.
 */
export function timePartOf(s: string, fallback: string): string {
  const parsed = parseAbsolute(s);
  return parsed ? formatTimeOnly(parsed) : fallback;
}

// --- boundaries ------------------------------------------------------------

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}
function addDays(d: Date, n: number): Date {
  const c = new Date(d);
  c.setDate(c.getDate() + n);
  return c;
}
function addMonths(d: Date, n: number): Date {
  const c = new Date(d);
  c.setMonth(c.getMonth() + n);
  return c;
}
function addYears(d: Date, n: number): Date {
  const c = new Date(d);
  c.setFullYear(c.getFullYear() + n);
  return c;
}
// Week starts Monday (the id-ID / ISO convention).
function startOfWeek(d: Date): Date {
  const s = startOfDay(d);
  return addDays(s, -((s.getDay() + 6) % 7));
}
function startOfMonth(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), 1);
}
function startOfYear(d: Date): Date {
  return new Date(d.getFullYear(), 0, 1);
}
function unix(d: Date): number {
  return Math.floor(d.getTime() / 1000);
}

// --- resolution ------------------------------------------------------------

/**
 * Resolve a preset against the current clock. `customFrom` / `customTo` are
 * only read for `preset: "custom"`; a missing or malformed value there falls
 * back to today.
 */
export function resolveRange(
  preset: RangePreset,
  customFrom?: string,
  customTo?: string,
): DateRange {
  const now = new Date();
  const today = startOfDay(now);
  // Exclusive end for day-aligned ranges: the start of tomorrow.
  const tomorrow = addDays(today, 1);
  const span = (from: Date, to: Date): DateRange => ({
    preset,
    fromUnix: unix(from),
    toUnix: unix(to),
  });
  // Rolling window ending at this very moment (the sub-day presets).
  const rolling = (ms: number): DateRange => span(new Date(now.getTime() - ms), now);
  const HOUR = 3_600_000;

  switch (preset) {
    case "today":
      return span(today, tomorrow);
    case "yesterday":
      return span(addDays(today, -1), today);
    case "dayBeforeYesterday":
      return span(addDays(today, -2), addDays(today, -1));
    case "thisDayLastWeek":
      return span(addDays(today, -7), addDays(today, -6));
    case "thisWeek": {
      const s = startOfWeek(now);
      return span(s, addDays(s, 7));
    }
    case "lastWeek": {
      const s = startOfWeek(now);
      return span(addDays(s, -7), s);
    }
    case "thisMonth": {
      const s = startOfMonth(now);
      return span(s, addMonths(s, 1));
    }
    case "lastMonth": {
      const s = startOfMonth(now);
      return span(addMonths(s, -1), s);
    }
    case "thisYear": {
      const s = startOfYear(now);
      return span(s, addYears(s, 1));
    }
    case "lastYear": {
      const s = startOfYear(now);
      return span(addYears(s, -1), s);
    }
    case "ytd":
      return span(startOfYear(now), tomorrow);
    case "15m":
      return rolling(15 * 60_000);
    case "1h":
      return rolling(HOUR);
    case "6h":
      return rolling(6 * HOUR);
    case "12h":
      return rolling(12 * HOUR);
    case "24h":
      return rolling(24 * HOUR);
    case "7d":
      return span(addDays(tomorrow, -7), tomorrow);
    case "30d":
      return span(addDays(tomorrow, -30), tomorrow);
    case "90d":
      return span(addDays(tomorrow, -90), tomorrow);
    case "6M":
      return span(addMonths(tomorrow, -6), tomorrow);
    case "1y":
      return span(addYears(tomorrow, -1), tomorrow);
    case "custom": {
      const from = parseAbsolute(customFrom) ?? today;
      const to = parseAbsolute(customTo) ?? tomorrow;
      return absoluteRange(from, to);
    }
  }
}

/** Build an absolute ("custom") range from two instants. */
export function absoluteRange(from: Date, to: Date): DateRange {
  return {
    preset: "custom",
    fromUnix: unix(from),
    toUnix: unix(to),
    customFrom: formatAbsolute(from),
    customTo: formatAbsolute(to),
  };
}

export function rangeBounds(range: DateRange): { from: Date; to: Date } {
  return { from: new Date(range.fromUnix * 1000), to: new Date(range.toUnix * 1000) };
}

/**
 * Button label: the quick-range name, or `from → to` for an absolute range.
 * `t` is the caller's i18next translator (labels live under `analytics.range`).
 */
export function rangeLabel(range: DateRange, t: (key: string) => string): string {
  if (range.preset !== "custom") return t(`analytics.range.${range.preset}`);
  const { from, to } = rangeBounds(range);
  return `${formatAbsolute(from)} → ${formatAbsolute(to)}`;
}
