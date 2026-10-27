/**
 * IANA time-zone helpers built on Intl only. Instants are epoch
 * milliseconds; wall-clock values are civil dates plus minutes since
 * midnight in a named zone.
 */
import { DateText, makeDate, pad2, addDays } from "~/lib/dates";

export function browserZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

export function isValidZone(tz: string): boolean {
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: tz });
    return true;
  } catch {
    return false;
  }
}

const partsCache = new Map<string, Intl.DateTimeFormat>();
function partsFormatter(tz: string): Intl.DateTimeFormat {
  let f = partsCache.get(tz);
  if (!f) {
    f = new Intl.DateTimeFormat("en-US", {
      timeZone: tz,
      hourCycle: "h23",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
    partsCache.set(tz, f);
  }
  return f;
}

export interface ZonedParts {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
  second: number;
}

export function zonedParts(ms: number, tz: string): ZonedParts {
  const out: Record<string, number> = {};
  for (const p of partsFormatter(tz).formatToParts(new Date(ms))) {
    if (p.type !== "literal") out[p.type] = Number(p.value);
  }
  return {
    year: out.year,
    month: out.month,
    day: out.day,
    hour: out.hour === 24 ? 0 : out.hour,
    minute: out.minute,
    second: out.second,
  };
}

/** Offset of `tz` from UTC at instant `ms`, in minutes (Denver summer: -360). */
export function tzOffsetMinutes(ms: number, tz: string): number {
  const p = zonedParts(ms, tz);
  const asUtc = Date.UTC(p.year, p.month - 1, p.day, p.hour, p.minute, p.second);
  return Math.round((asUtc - Math.floor(ms / 1000) * 1000) / 60000);
}

/**
 * The instant of a wall-clock time in `tz`. In a spring-forward gap the
 * result is the same wall time shifted forward by the gap (02:30 -> 03:30).
 * In a fall-back overlap the earlier instant wins.
 */
export function zonedToUtc(date: DateText, minutes: number, tz: string): number {
  const [y, m, d] = date.split("-").map(Number);
  const wall = Date.UTC(y, m - 1, d, 0, minutes);
  const o1 = tzOffsetMinutes(wall, tz);
  let t = wall - o1 * 60000;
  const o2 = tzOffsetMinutes(t, tz);
  if (o2 !== o1) {
    t = wall - o2 * 60000;
    // Gap: neither offset gives back the wall time; use the earlier offset.
    if (tzOffsetMinutes(t, tz) !== o2) t = wall - o1 * 60000;
  } else {
    // Overlap: prefer the earlier instant if the larger offset also fits.
    const alt = wall - (o1 + 60) * 60000;
    if (tzOffsetMinutes(alt, tz) === o1 + 60) t = alt;
  }
  return t;
}

/** Midnight at the start of `date` in `tz`. */
export function startOfDayUtc(date: DateText, tz: string): number {
  return zonedToUtc(date, 0, tz);
}

export function civilDate(ms: number, tz: string): DateText {
  const p = zonedParts(ms, tz);
  return makeDate(p.year, p.month, p.day);
}

/** Wall-clock minutes since midnight in `tz`. */
export function minutesIntoDay(ms: number, tz: string): number {
  const p = zonedParts(ms, tz);
  return p.hour * 60 + p.minute;
}

export function todayIn(tz: string, now = Date.now()): DateText {
  return civilDate(now, tz);
}

/** RFC3339 UTC with whole seconds and a Z: the CSIL Timestamp form. */
export function toTimestamp(ms: number): string {
  return new Date(Math.floor(ms / 1000) * 1000).toISOString().replace(/\.\d{3}Z$/, "Z");
}

export function parseTimestamp(ts: string): number {
  return Date.parse(ts);
}

/** "HH:MM" for minutes since midnight. */
export function minutesToHHMM(min: number): string {
  return `${pad2(Math.floor(min / 60))}:${pad2(min % 60)}`;
}

export function hhmmToMinutes(s: string): number {
  const m = /^(\d{1,2}):(\d{2})$/.exec(s);
  if (!m) return NaN;
  return Number(m[1]) * 60 + Number(m[2]);
}

let hour12Cache: boolean | null = null;
/** Does the reader's locale use a 12-hour clock? */
export function prefers12Hour(): boolean {
  if (hour12Cache === null) {
    try {
      hour12Cache = new Intl.DateTimeFormat(undefined, { hour: "numeric" }).resolvedOptions().hourCycle?.startsWith("h1") ?? false;
    } catch {
      hour12Cache = false;
    }
  }
  return hour12Cache;
}

const fmtCache = new Map<string, Intl.DateTimeFormat>();
function fmt(tz: string, opts: Intl.DateTimeFormatOptions, locale?: string): Intl.DateTimeFormat {
  const key = `${locale ?? ""}|${tz}|${JSON.stringify(opts)}`;
  let f = fmtCache.get(key);
  if (!f) {
    f = new Intl.DateTimeFormat(locale, { ...opts, timeZone: tz });
    fmtCache.set(key, f);
  }
  return f;
}

/** "9:30 AM" or "09:30" in `tz`. Whole hours drop ":00" on a 12-hour clock. */
export function formatTime(ms: number, tz: string, opts: { hour12?: boolean; locale?: string } = {}): string {
  const hour12 = opts.hour12 ?? prefers12Hour();
  if (!hour12) {
    const p = zonedParts(ms, tz);
    return `${pad2(p.hour)}:${pad2(p.minute)}`;
  }
  const p = zonedParts(ms, tz);
  const h = p.hour % 12 === 0 ? 12 : p.hour % 12;
  const ap = p.hour < 12 ? "AM" : "PM";
  return p.minute === 0 ? `${h} ${ap}` : `${h}:${pad2(p.minute)} ${ap}`;
}

export function formatTimeRange(startMs: number, endMs: number, tz: string, opts: { hour12?: boolean } = {}): string {
  return `${formatTime(startMs, tz, opts)} – ${formatTime(endMs, tz, opts)}`;
}

/** Hour label for the time grid ("9 AM" / "09:00"). */
export function formatHourLabel(hour: number, hour12 = prefers12Hour()): string {
  if (!hour12) return `${pad2(hour)}:00`;
  const h = hour % 12 === 0 ? 12 : hour % 12;
  return `${h} ${hour < 12 ? "AM" : "PM"}`;
}

/** A civil date in words: "Thursday, October 15, 2026". Zone-free. */
export function formatDate(date: DateText, opts: Intl.DateTimeFormatOptions = { weekday: "long", month: "long", day: "numeric", year: "numeric" }, locale?: string): string {
  const [y, m, d] = date.split("-").map(Number);
  return fmt("UTC", opts, locale).format(new Date(Date.UTC(y, m - 1, d, 12)));
}

export function formatMonthYear(date: DateText, locale?: string): string {
  return formatDate(date, { month: "long", year: "numeric" }, locale);
}

export function weekdayShort(date: DateText, locale?: string): string {
  return formatDate(date, { weekday: "short" }, locale);
}

/** Short zone name in effect at `ms`: "MDT", "GMT+2". */
export function zoneAbbrev(ms: number, tz: string): string {
  const part = fmt(tz, { timeZoneName: "short" }).formatToParts(new Date(ms)).find((p) => p.type === "timeZoneName");
  return part?.value ?? tz;
}

/** "UTC−06:00" style offset label. */
export function offsetLabel(ms: number, tz: string): string {
  const o = tzOffsetMinutes(ms, tz);
  const sign = o < 0 ? "−" : "+";
  const a = Math.abs(o);
  return `UTC${sign}${pad2(Math.floor(a / 60))}:${pad2(a % 60)}`;
}

/** IANA zones, `first` at the top, then the rest A-Z. */
export function zoneList(first: string): string[] {
  let all: string[] = [];
  try {
    const sv = (Intl as unknown as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf;
    all = sv ? sv("timeZone") : [];
  } catch {
    all = [];
  }
  if (!all.includes("UTC")) all = [...all, "UTC"];
  const rest = all.filter((z) => z !== first).sort();
  return first ? [first, ...rest] : rest;
}

/** The next instant on a 30-minute boundary at or after `ms`, in `tz`. */
export function roundUpToSlot(ms: number, tz: string, slot = 30): { date: DateText; minutes: number } {
  const date = civilDate(ms, tz);
  let min = Math.ceil(minutesIntoDay(ms, tz) / slot) * slot;
  if (min >= 1440) return { date: addDays(date, 1), minutes: min - 1440 };
  return { date, minutes: min };
}

/** "Oct 15, 2026" (fixed English, used in generated sentences). */
export function formatDateShort(d: DateText): string {
  return formatDate(d, { month: "short", day: "numeric", year: "numeric" }, "en-US");
}
