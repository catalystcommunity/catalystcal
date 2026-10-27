/**
 * RRULE <-> recurrence form state.
 *
 * buildRRule turns the editor's form into an RFC 5545 RRULE value (no
 * "RRULE:" prefix). parseRRule reads an RRULE back into the form. A rule the
 * form cannot show exactly (BYSETPOS, BYHOUR, several month days, ...)
 * comes back as freq "custom" with the original text, so nothing is lost.
 */
import { DateText, WEEKDAY_CODES, WeekdayCode, addMonths, dayOfWeek, isDateText, makeDate, nthWeekdayOfMonth, parseDate } from "~/lib/dates";
import { civilDate, formatDateShort, zonedToUtc } from "~/lib/tz";

export type RecurrenceFreq = "none" | "daily" | "weekly" | "monthly" | "yearly" | "custom";
export type RecurrenceEnds = "never" | "on" | "after";

export interface RecurrenceForm {
  freq: RecurrenceFreq;
  interval: number;
  /** Weekly: the days it repeats on. */
  byDay: WeekdayCode[];
  /** Monthly: on day N of the month, or on the Nth weekday. */
  monthlyMode: "day" | "nth";
  monthDay: number;
  /** 1..5, or -1 for "last". */
  nth: number;
  nthDay: WeekdayCode;
  ends: RecurrenceEnds;
  /** Ends "on": the last date (inclusive). */
  until: DateText;
  /** Ends "after": the number of occurrences. */
  count: number;
  /** freq "custom": the RRULE text. */
  custom: string;
}

/** Form defaults that match the event's first day. */
export function defaultRecurrenceForm(start: DateText): RecurrenceForm {
  const { d } = parseDate(start);
  const { nth, isLast } = nthWeekdayOfMonth(start);
  return {
    freq: "none",
    interval: 1,
    byDay: [WEEKDAY_CODES[dayOfWeek(start)]],
    monthlyMode: "day",
    monthDay: d,
    nth: nth === 5 || (isLast && nth > 4) ? -1 : nth,
    nthDay: WEEKDAY_CODES[dayOfWeek(start)],
    ends: "never",
    until: addMonths(start, 3),
    count: 10,
    custom: "",
  };
}

export interface BuildOptions {
  /** The event's zone; UNTIL for a timed event is the end of that day there. */
  tz: string;
  allDay: boolean;
}

const FREQ_WORD: Record<string, string> = { daily: "DAILY", weekly: "WEEKLY", monthly: "MONTHLY", yearly: "YEARLY" };

/** Strip an optional "RRULE:" prefix and spaces. */
export function normalizeRRule(text: string): string {
  return text.trim().replace(/^RRULE:/i, "").replace(/\s+/g, "");
}

export function buildRRule(form: RecurrenceForm, opts: BuildOptions): string | null {
  if (form.freq === "none") return null;
  if (form.freq === "custom") {
    const t = normalizeRRule(form.custom);
    return t ? t.toUpperCase() : null;
  }
  const parts = [`FREQ=${FREQ_WORD[form.freq]}`];
  const interval = Math.max(1, Math.floor(form.interval || 1));
  if (interval > 1) parts.push(`INTERVAL=${interval}`);
  if (form.freq === "weekly" && form.byDay.length) {
    const days = WEEKDAY_CODES.filter((c) => form.byDay.includes(c));
    parts.push(`BYDAY=${days.join(",")}`);
  }
  if (form.freq === "monthly") {
    if (form.monthlyMode === "day") parts.push(`BYMONTHDAY=${form.monthDay}`);
    else parts.push(`BYDAY=${form.nth}${form.nthDay}`);
  }
  if (form.ends === "after") parts.push(`COUNT=${Math.max(1, Math.floor(form.count || 1))}`);
  if (form.ends === "on" && isDateText(form.until)) parts.push(`UNTIL=${untilValue(form.until, opts)}`);
  return parts.join(";");
}

function untilValue(date: DateText, opts: BuildOptions): string {
  const compact = date.replace(/-/g, "");
  if (opts.allDay) return compact;
  // The last moment of that day in the event's zone, in UTC.
  const ms = zonedToUtc(date, 24 * 60, opts.tz) - 1000;
  return new Date(ms).toISOString().replace(/[-:]/g, "").replace(/\.\d{3}/, "");
}

/** Read an UNTIL value back to a civil date in `tz`. */
export function parseUntil(v: string, tz: string): DateText | null {
  const d = /^(\d{4})(\d{2})(\d{2})$/.exec(v);
  if (d) return makeDate(Number(d[1]), Number(d[2]), Number(d[3]));
  const t = /^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})(Z?)$/.exec(v);
  if (!t) return null;
  const [y, mo, da, h, mi, s] = t.slice(1, 7).map(Number);
  if (t[7] === "Z") return civilDate(Date.UTC(y, mo - 1, da, h, mi, s), tz);
  return makeDate(y, mo, da);
}

const ALLOWED = new Set(["FREQ", "INTERVAL", "BYDAY", "BYMONTHDAY", "BYMONTH", "COUNT", "UNTIL", "WKST"]);

/**
 * Parse an RRULE into form state for an event that starts on `start`.
 * Anything the form cannot show exactly becomes { freq: "custom" }.
 */
export function parseRRule(rrule: string | undefined | null, start: DateText, tz: string): RecurrenceForm {
  const base = defaultRecurrenceForm(start);
  if (!rrule || !normalizeRRule(rrule)) return base;
  const text = normalizeRRule(rrule).toUpperCase();
  const custom: RecurrenceForm = { ...base, freq: "custom", custom: text };

  const map = new Map<string, string>();
  for (const part of text.split(";")) {
    if (!part) continue;
    const [k, v, ...rest] = part.split("=");
    if (!k || v === undefined || rest.length || map.has(k) || !ALLOWED.has(k)) return custom;
    map.set(k, v);
  }

  const FREQS: Record<string, "daily" | "weekly" | "monthly" | "yearly"> = { DAILY: "daily", WEEKLY: "weekly", MONTHLY: "monthly", YEARLY: "yearly" };
  const freq = FREQS[map.get("FREQ") ?? ""] as "daily" | "weekly" | "monthly" | "yearly" | undefined;
  if (!freq) return custom;
  const form: RecurrenceForm = { ...base, freq };

  if (map.has("INTERVAL")) {
    const n = Number(map.get("INTERVAL"));
    if (!Number.isInteger(n) || n < 1) return custom;
    form.interval = n;
  }
  if (map.has("WKST") && !WEEKDAY_CODES.includes(map.get("WKST") as WeekdayCode)) return custom;

  if (map.has("COUNT") && map.has("UNTIL")) return custom;
  if (map.has("COUNT")) {
    const n = Number(map.get("COUNT"));
    if (!Number.isInteger(n) || n < 1) return custom;
    form.ends = "after";
    form.count = n;
  }
  if (map.has("UNTIL")) {
    const d = parseUntil(map.get("UNTIL")!, tz);
    if (!d) return custom;
    form.ends = "on";
    form.until = d;
  }

  const byDay = map.get("BYDAY");
  const byMonthDay = map.get("BYMONTHDAY");
  const byMonth = map.get("BYMONTH");

  if (byMonth !== undefined) {
    // Only the event's own month in a yearly rule says nothing new.
    if (freq !== "yearly" || Number(byMonth) !== parseDate(start).m) return custom;
  }

  switch (freq) {
    case "daily":
    case "yearly":
      if (byDay !== undefined || byMonthDay !== undefined) return custom;
      break;
    case "weekly": {
      if (byMonthDay !== undefined) return custom;
      if (byDay !== undefined) {
        const days = byDay.split(",");
        if (!days.every((d) => WEEKDAY_CODES.includes(d as WeekdayCode))) return custom;
        form.byDay = WEEKDAY_CODES.filter((c) => days.includes(c));
      }
      break;
    }
    case "monthly": {
      if (byDay !== undefined && byMonthDay !== undefined) return custom;
      if (byMonthDay !== undefined) {
        const n = Number(byMonthDay);
        if (!/^\d{1,2}$/.test(byMonthDay) || n < 1 || n > 31) return custom;
        form.monthlyMode = "day";
        form.monthDay = n;
      } else if (byDay !== undefined) {
        const m = /^(-1|[1-5])(SU|MO|TU|WE|TH|FR|SA)$/.exec(byDay);
        if (!m) return custom;
        form.monthlyMode = "nth";
        form.nth = Number(m[1]);
        form.nthDay = m[2] as WeekdayCode;
      }
      break;
    }
  }
  return form;
}

const DAY_NAME: Record<WeekdayCode, string> = { SU: "Sunday", MO: "Monday", TU: "Tuesday", WE: "Wednesday", TH: "Thursday", FR: "Friday", SA: "Saturday" };
const DAY_SHORT: Record<WeekdayCode, string> = { SU: "Sun", MO: "Mon", TU: "Tue", WE: "Wed", TH: "Thu", FR: "Fri", SA: "Sat" };
const ORDINAL: Record<string, string> = { "1": "first", "2": "second", "3": "third", "4": "fourth", "5": "fifth", "-1": "last" };

export function weekdayName(c: WeekdayCode): string {
  return DAY_NAME[c];
}
export function weekdayShortName(c: WeekdayCode): string {
  return DAY_SHORT[c];
}
export function ordinalWord(n: number): string {
  return ORDINAL[String(n)] ?? `${n}th`;
}

/** Plain-language summary: "Every 2 weeks on Mon, Wed, 10 times". */
export function describeRecurrence(form: RecurrenceForm, start: DateText): string {
  if (form.freq === "none") return "Does not repeat";
  if (form.freq === "custom") return form.custom ? `Custom rule: ${form.custom}` : "Custom rule";
  const n = Math.max(1, form.interval);
  const unit = { daily: "day", weekly: "week", monthly: "month", yearly: "year" }[form.freq];
  let s = n === 1 ? (form.freq === "daily" ? "Daily" : form.freq === "weekly" ? "Weekly" : form.freq === "monthly" ? "Monthly" : "Yearly") : `Every ${n} ${unit}s`;
  if (form.freq === "weekly" && form.byDay.length) {
    const days = WEEKDAY_CODES.filter((c) => form.byDay.includes(c));
    const weekdays = ["MO", "TU", "WE", "TH", "FR"];
    if (days.length === 5 && weekdays.every((d) => days.includes(d as WeekdayCode))) s += " on weekdays";
    else s += ` on ${days.map((d) => DAY_SHORT[d]).join(", ")}`;
  }
  if (form.freq === "monthly") {
    s += form.monthlyMode === "day" ? ` on day ${form.monthDay}` : ` on the ${ordinalWord(form.nth)} ${DAY_NAME[form.nthDay]}`;
  }
  if (form.freq === "yearly") s += ` on ${formatDateShort(start)}`;
  if (form.ends === "after") s += form.count === 1 ? ", once" : `, ${form.count} times`;
  if (form.ends === "on") s += `, until ${formatDateShort(form.until)}`;
  return s;
}

/** Describe a stored RRULE for the event detail. */
export function describeRRule(rrule: string, start: DateText, tz: string): string {
  return describeRecurrence(parseRRule(rrule, start, tz), start);
}
