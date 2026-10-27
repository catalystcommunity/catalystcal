/**
 * Civil-date math on "YYYY-MM-DD" strings. A civil date has no time zone,
 * so this code uses Date.UTC only as a day counter and never reads the
 * local zone. Time-zone work is in tz.ts.
 */

export type DateText = string;
/** 0 = Sunday ... 6 = Saturday. */
export type Weekday = 0 | 1 | 2 | 3 | 4 | 5 | 6;

const DAY_MS = 86_400_000;

export function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

export function makeDate(y: number, m: number, d: number): DateText {
  const t = new Date(Date.UTC(y, m - 1, d));
  return `${String(t.getUTCFullYear()).padStart(4, "0")}-${pad2(t.getUTCMonth() + 1)}-${pad2(t.getUTCDate())}`;
}

export function parseDate(s: DateText): { y: number; m: number; d: number } {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  if (!m) throw new Error(`not a date: ${s}`);
  return { y: Number(m[1]), m: Number(m[2]), d: Number(m[3]) };
}

export function isDateText(s: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(s);
}

const dayNumber = (s: DateText): number => {
  const { y, m, d } = parseDate(s);
  return Math.round(Date.UTC(y, m - 1, d) / DAY_MS);
};

const fromDayNumber = (n: number): DateText => {
  const t = new Date(n * DAY_MS);
  return makeDate(t.getUTCFullYear(), t.getUTCMonth() + 1, t.getUTCDate());
};

export function addDays(s: DateText, n: number): DateText {
  return fromDayNumber(dayNumber(s) + n);
}

/** b - a in whole days. */
export function daysBetween(a: DateText, b: DateText): number {
  return dayNumber(b) - dayNumber(a);
}

export function dayOfWeek(s: DateText): Weekday {
  // 1970-01-01 was a Thursday (4).
  return ((((dayNumber(s) + 4) % 7) + 7) % 7) as Weekday;
}

/** Add months, keeping the day of month inside the target month. */
export function addMonths(s: DateText, n: number): DateText {
  const { y, m, d } = parseDate(s);
  const idx = y * 12 + (m - 1) + n;
  const ny = Math.floor(idx / 12);
  const nm = (idx % 12) + 1;
  return makeDate(ny, nm, Math.min(d, daysInMonth(ny, nm)));
}

export function addYears(s: DateText, n: number): DateText {
  return addMonths(s, n * 12);
}

export function daysInMonth(y: number, m: number): number {
  return new Date(Date.UTC(y, m, 0)).getUTCDate();
}

export function startOfMonth(s: DateText): DateText {
  const { y, m } = parseDate(s);
  return makeDate(y, m, 1);
}

export function startOfWeek(s: DateText, weekStart: Weekday): DateText {
  const diff = (dayOfWeek(s) - weekStart + 7) % 7;
  return addDays(s, -diff);
}

export function rangeDays(start: DateText, count: number): DateText[] {
  return Array.from({ length: count }, (_, i) => addDays(start, i));
}

export function weekDays(s: DateText, weekStart: Weekday): DateText[] {
  return rangeDays(startOfWeek(s, weekStart), 7);
}

/**
 * The weeks a month view shows: from the week that holds the 1st to the
 * week that holds the last day. 4 to 6 rows of 7 dates.
 */
export function monthGrid(s: DateText, weekStart: Weekday): DateText[][] {
  const first = startOfMonth(s);
  const { y, m } = parseDate(first);
  const last = makeDate(y, m, daysInMonth(y, m));
  const start = startOfWeek(first, weekStart);
  const weeks: DateText[][] = [];
  for (let w = start; daysBetween(w, last) >= 0; w = addDays(w, 7)) weeks.push(rangeDays(w, 7));
  return weeks;
}

export function sameMonth(a: DateText, b: DateText): boolean {
  return a.slice(0, 7) === b.slice(0, 7);
}

export function compareDates(a: DateText, b: DateText): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

export function nthWeekdayOfMonth(s: DateText): { nth: number; isLast: boolean } {
  const { y, m, d } = parseDate(s);
  return { nth: Math.ceil(d / 7), isLast: d + 7 > daysInMonth(y, m) };
}

export const WEEKDAY_CODES = ["SU", "MO", "TU", "WE", "TH", "FR", "SA"] as const;
export type WeekdayCode = (typeof WEEKDAY_CODES)[number];
