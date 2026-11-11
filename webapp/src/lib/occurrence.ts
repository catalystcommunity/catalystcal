/** A stable key for an occurrence, and the dates of an all-day occurrence. */
import type { Occurrence } from "@catalystcal/client";
import { DateText, daysBetween, isDateText } from "~/lib/dates";
import { civilDate, parseTimestamp } from "~/lib/tz";

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
