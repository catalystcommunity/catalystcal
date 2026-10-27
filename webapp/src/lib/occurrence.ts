/**
 * Turn API occurrences into what the views draw: instants, civil-date
 * spans in the viewer's zone, and a stable key.
 */
import type { Occurrence } from "@catalystcal/client";
import { DateText, addDays, daysBetween, isDateText } from "~/lib/dates";
import { civilDate, parseTimestamp } from "~/lib/tz";

export interface Placed {
  key: string;
  occ: Occurrence;
  startMs: number;
  endMs: number;
  /** All-day, or 24 hours or longer: drawn in the all-day row as a bar. */
  bar: boolean;
  allDay: boolean;
  /** First civil date (viewer zone, or the event's own dates if all-day). */
  startDate: DateText;
  /** Last civil date covered (inclusive). */
  lastDate: DateText;
}

export function occurrenceKey(o: Occurrence): string {
  return `${o.event.eventId}|${o.recurrenceId ?? o.start}`;
}

/**
 * The first date of an all-day occurrence. The CSIL does not pin how an
 * all-day instant is written, so accept both: midnight UTC means the UTC
 * date; anything else is midnight in the event's zone.
 */
export function allDayStart(o: Occurrence, viewerTz: string): DateText {
  const ev = o.event;
  if (!ev.recurrence && !o.recurrenceId && ev.startDate && isDateText(ev.startDate)) return ev.startDate;
  if (/T00:00:00Z$/.test(o.start)) return o.start.slice(0, 10);
  return civilDate(parseTimestamp(o.start), ev.timezone || viewerTz);
}

export function allDayLength(o: Occurrence): number {
  const ev = o.event;
  if (ev.startDate && ev.endDate && isDateText(ev.startDate) && isDateText(ev.endDate)) {
    return Math.max(1, daysBetween(ev.startDate, ev.endDate));
  }
  return Math.max(1, Math.round((parseTimestamp(o.end) - parseTimestamp(o.start)) / 86_400_000));
}

export function placeOccurrence(o: Occurrence, tz: string): Placed {
  const startMs = parseTimestamp(o.start);
  const endMs = Math.max(parseTimestamp(o.end), startMs);
  const key = occurrenceKey(o);
  if (o.event.allDay) {
    const sd = allDayStart(o, tz);
    return { key, occ: o, startMs, endMs, bar: true, allDay: true, startDate: sd, lastDate: addDays(sd, allDayLength(o) - 1) };
  }
  const startDate = civilDate(startMs, tz);
  // An event that ends at midnight does not cover the next day.
  const lastDate = endMs > startMs ? civilDate(endMs - 1, tz) : startDate;
  const bar = endMs - startMs >= 86_400_000;
  return { key, occ: o, startMs, endMs, bar, allDay: false, startDate, lastDate };
}

/** Does the placed occurrence cover civil date `d`? */
export function covers(p: Placed, d: DateText): boolean {
  return p.startDate <= d && d <= p.lastDate;
}

export function sortPlaced(list: Placed[]): Placed[] {
  return [...list].sort(
    (a, b) =>
      Number(b.bar) - Number(a.bar) ||
      a.startDate.localeCompare(b.startDate) ||
      a.startMs - b.startMs ||
      b.endMs - a.endMs ||
      a.occ.event.title.localeCompare(b.occ.event.title),
  );
}
