/** An event from search or invitations, as a date and as an occurrence. */
import type { Event, Occurrence } from "@catalystcal/client";
import { calendarById } from "~/stores/calendars";
import { civilDate, parseTimestamp } from "~/lib/tz";

/** The civil date of the start of `e` in `tz`. */
export function eventDate(e: Event, tz: string): string | null {
  if (e.allDay && e.startDate) return e.startDate;
  if (e.start) return civilDate(parseTimestamp(e.start), tz);
  return null;
}

/** A detail-dialog occurrence for an event that came from search or invitations. */
export function occurrenceOf(e: Event, invited = false): Occurrence {
  const cal = calendarById(e.calendarId);
  const start = e.start ?? (e.startDate ? `${e.startDate}T00:00:00Z` : new Date().toISOString());
  const end = e.end ?? (e.endDate ? `${e.endDate}T00:00:00Z` : start);
  return { event: e, start, end, access: cal?.access ?? "read", busyOnly: false, invited: invited || !cal };
}
