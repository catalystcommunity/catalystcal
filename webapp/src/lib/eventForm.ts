/**
 * The event editor's form state and its conversion to CSIL requests.
 *
 * The generated Event type marks receive-only fields (event_id, sequence,
 * created_at, updated_at) as required and the codec always writes them, so
 * a new event carries empty placeholders. Optional receive-only fields
 * (organizer, created_by, ...) are left out of every request.
 */
import type { Attendee, Event, EventStatus, EventTransparency, EventVisibility, Occurrence, RecurrenceScope, Reminder, UpdateEventRequest, DeleteEventRequest } from "@catalystcal/client";
import { DateText, addDays, daysBetween } from "~/lib/dates";
import { RecurrenceForm, buildRRule, defaultRecurrenceForm, parseRRule } from "~/lib/rrule";
import { allDayLength, allDayStart } from "~/lib/occurrence";
import { civilDate, minutesIntoDay, parseTimestamp, toTimestamp, zonedToUtc } from "~/lib/tz";

export interface EventForm {
  title: string;
  calendarId: string;
  allDay: boolean;
  startDate: DateText;
  /** Minutes since midnight in `timezone`. */
  startMin: number;
  /** For an all-day event: the last day, inclusive. */
  endDate: DateText;
  endMin: number;
  timezone: string;
  location: string;
  conferenceUrl: string;
  url: string;
  description: string;
  status: EventStatus;
  transparency: EventTransparency;
  visibility: EventVisibility;
  recurrence: RecurrenceForm;
  attendees: Attendee[];
  reminders: Reminder[];
  categories: string[];
  color: string;
}

export interface NewEventSeed {
  date: DateText;
  startMin?: number;
  endDate?: DateText;
  endMin?: number;
  allDay?: boolean;
  calendarId: string;
  timezone: string;
  title?: string;
}

export function newEventForm(seed: NewEventSeed): EventForm {
  const startMin = seed.startMin ?? 9 * 60;
  let endDate = seed.endDate ?? seed.date;
  let endMin = seed.endMin ?? startMin + 60;
  if (endMin >= 1440 && !seed.endDate) {
    endDate = addDays(seed.date, 1);
    endMin -= 1440;
  }
  return {
    title: seed.title ?? "",
    calendarId: seed.calendarId,
    allDay: seed.allDay ?? false,
    startDate: seed.date,
    startMin,
    endDate,
    endMin,
    timezone: seed.timezone,
    location: "",
    conferenceUrl: "",
    url: "",
    description: "",
    status: "confirmed",
    transparency: "opaque",
    visibility: "default",
    recurrence: defaultRecurrenceForm(seed.date),
    attendees: [],
    reminders: [{ minutesBefore: 10, method: "display" }],
    categories: [],
    color: "",
  };
}

/** The form for editing an occurrence (the instance's own times). */
export function formFromOccurrence(o: Occurrence, viewerTz: string): EventForm {
  const ev = o.event;
  const tz = ev.timezone || viewerTz;
  let startDate: DateText, endDate: DateText, startMin = 9 * 60, endMin = 10 * 60;
  if (ev.allDay) {
    startDate = allDayStart(o, viewerTz);
    endDate = addDays(startDate, allDayLength(o) - 1);
  } else {
    const s = parseTimestamp(o.start);
    const e = parseTimestamp(o.end);
    startDate = civilDate(s, tz);
    startMin = minutesIntoDay(s, tz);
    endDate = civilDate(e, tz);
    endMin = minutesIntoDay(e, tz);
  }
  // The rule is anchored on the series start.
  const seriesStart = ev.allDay ? (ev.startDate ?? startDate) : ev.start ? civilDate(parseTimestamp(ev.start), tz) : startDate;
  return {
    title: ev.title,
    calendarId: ev.calendarId,
    allDay: !!ev.allDay,
    startDate,
    startMin,
    endDate,
    endMin,
    timezone: tz,
    location: ev.location ?? "",
    conferenceUrl: ev.conferenceUrl ?? "",
    url: ev.url ?? "",
    description: ev.description ?? "",
    status: ev.status ?? "confirmed",
    transparency: ev.transparency ?? "opaque",
    visibility: ev.visibility ?? "default",
    recurrence: ev.recurrence ? parseRRule(ev.recurrence.rrule, seriesStart, tz) : defaultRecurrenceForm(startDate),
    attendees: (ev.attendees ?? []).map((a) => ({ ...a })),
    reminders: (ev.reminders ?? []).map((r) => ({ ...r })),
    categories: [...(ev.categories ?? [])],
    color: ev.color ?? "",
  };
}

/** Start and end instants of a timed form. */
export function formInstants(f: EventForm): { startMs: number; endMs: number } {
  return { startMs: zonedToUtc(f.startDate, f.startMin, f.timezone), endMs: zonedToUtc(f.endDate, f.endMin, f.timezone) };
}

export function validateForm(f: EventForm): string | null {
  if (!f.title.trim()) return "Give the event a title.";
  if (f.title.length > 1024) return "The title is too long (1024 characters at most).";
  if (!f.calendarId) return "Choose a calendar.";
  if (f.allDay) {
    if (daysBetween(f.startDate, f.endDate) < 0) return "The end date is before the start date.";
  } else {
    const { startMs, endMs } = formInstants(f);
    if (!(endMs > startMs)) return "The end time must be after the start time.";
  }
  if (f.recurrence.freq === "weekly" && f.recurrence.byDay.length === 0) return "Choose at least one day of the week for the repeat.";
  if (f.recurrence.freq === "custom" && !/^FREQ=/i.test(f.recurrence.custom.trim().replace(/^RRULE:/i, ""))) return "A custom repeat rule must start with FREQ=, for example FREQ=WEEKLY;BYDAY=MO.";
  if (f.recurrence.ends === "on" && f.recurrence.freq !== "none" && f.recurrence.freq !== "custom" && f.recurrence.until < f.startDate) return "The repeat end date is before the first event.";
  return null;
}

const clean = (s: string): string | undefined => (s.trim() ? s.trim() : undefined);

function writableAttendee(a: Attendee): Attendee {
  const { person, respondedBy, ...rest } = a;
  return rest;
}

/** Build the Event to send. `base` keeps the id, uid and sequence of an existing event. */
export function formToEvent(f: EventForm, base?: Event, opts: { withRecurrence?: boolean } = {}): Event {
  const withRecurrence = opts.withRecurrence ?? true;
  const ev: Event = {
    eventId: base?.eventId ?? "",
    calendarId: f.calendarId,
    uid: base?.uid,
    title: f.title.trim(),
    description: clean(f.description),
    location: clean(f.location),
    url: clean(f.url),
    conferenceUrl: clean(f.conferenceUrl),
    allDay: f.allDay,
    timezone: f.timezone,
    status: f.status,
    transparency: f.transparency,
    visibility: f.visibility,
    attendees: f.attendees.map(writableAttendee),
    reminders: f.reminders.filter((r) => Number.isFinite(r.minutesBefore) && r.minutesBefore >= 0),
    categories: f.categories.map((c) => c.trim()).filter(Boolean),
    color: clean(f.color),
    sequence: base?.sequence ?? 0,
    createdAt: base?.createdAt ?? "",
    updatedAt: base?.updatedAt ?? "",
  };
  if (f.allDay) {
    ev.startDate = f.startDate;
    ev.endDate = addDays(f.endDate, 1);
  } else {
    const { startMs, endMs } = formInstants(f);
    ev.start = toTimestamp(startMs);
    ev.end = toTimestamp(endMs);
  }
  if (withRecurrence) {
    const rrule = buildRRule(f.recurrence, { tz: f.timezone, allDay: f.allDay });
    if (rrule) ev.recurrence = { rrule, exdates: base?.recurrence?.exdates, rdates: base?.recurrence?.rdates };
  }
  return ev;
}

/** Is this occurrence part of a series (so edits need a scope)? */
export function isSeriesOccurrence(o: Occurrence): boolean {
  return !!o.event.recurrence || !!o.recurrenceId || !!o.event.masterEventId;
}

export function instanceId(o: Occurrence): string {
  return o.recurrenceId ?? o.event.recurrenceId ?? o.start;
}

/**
 * The update request for an edit of occurrence `o` with `scope`.
 *
 * - all: the master changes. A time change made on one instance moves
 *   the whole series by the same amount.
 * - this: an override for one instance, with no rule.
 * - this_and_future: the series splits at this instance with the form's rule.
 */
export function buildUpdate(f: EventForm, o: Occurrence, scope: RecurrenceScope): UpdateEventRequest {
  const master = o.event;
  if (!isSeriesOccurrence(o) || scope === "all") {
    const ev = formToEvent(f, master);
    if (isSeriesOccurrence(o)) shiftToSeriesStart(ev, f, o);
    return { event: ev, scope: "all" };
  }
  const ev = formToEvent(f, master, { withRecurrence: scope === "this_and_future" });
  return { event: ev, scope, recurrenceId: instanceId(o) };
}

function shiftToSeriesStart(ev: Event, f: EventForm, o: Occurrence): void {
  const master = o.event;
  if (f.allDay) {
    const instStart = allDayStart(o, f.timezone);
    const delta = daysBetween(instStart, f.startDate);
    const len = daysBetween(f.startDate, f.endDate) + 1;
    const seriesStart = master.allDay && master.startDate ? master.startDate : instStart;
    ev.startDate = addDays(seriesStart, delta);
    ev.endDate = addDays(ev.startDate, len);
    return;
  }
  if (!master.start || master.allDay) return;
  const { startMs, endMs } = formInstants(f);
  const delta = startMs - parseTimestamp(o.start);
  const newStart = parseTimestamp(master.start) + delta;
  ev.start = toTimestamp(newStart);
  ev.end = toTimestamp(newStart + (endMs - startMs));
}

export function buildDelete(o: Occurrence, scope: RecurrenceScope): DeleteEventRequest {
  if (!isSeriesOccurrence(o) || scope === "all") return { eventId: o.event.eventId, scope: "all" };
  return { eventId: o.event.eventId, scope, recurrenceId: instanceId(o) };
}

/** A copy of an event as a new event (Duplicate). */
export function duplicateForm(o: Occurrence, viewerTz: string): EventForm {
  const f = formFromOccurrence(o, viewerTz);
  f.title = `${f.title} (copy)`;
  f.attendees = f.attendees.map((a) => ({ ...a, status: "needs_action" }));
  return f;
}
