import type { Attendee, Occurrence, ParticipationStatus, PersonSummary } from "@catalystcal/client";
import { calendarById, calendarColor } from "~/stores/calendars";
import { effectivePerson } from "~/stores/auth";

export const INVITE_COLOR = "#7d879b";

export function occColor(o: Occurrence): string {
  if (o.event.color) return o.event.color;
  const cal = calendarById(o.event.calendarId);
  if (cal) return calendarColor(cal);
  return INVITE_COLOR;
}

export function isMe(a: Attendee, me: PersonSummary | null): boolean {
  if (!me) return false;
  if (a.person) return a.person.userId === me.userId;
  if (a.identity?.userId) return a.identity.userId === me.userId && a.identity.domain.toLowerCase() === me.domain.toLowerCase();
  if (a.identity?.handle && me.handle) return a.identity.handle.toLowerCase() === me.handle.toLowerCase() && a.identity.domain.toLowerCase() === me.domain.toLowerCase();
  return false;
}

/** The current principal's attendee entry, if they are on the list. */
export function myAttendee(o: { event: { attendees?: Attendee[] } }, me: PersonSummary | null = effectivePerson()): Attendee | undefined {
  return (o.event.attendees ?? []).find((a) => isMe(a, me));
}

export function myStatus(o: Occurrence): ParticipationStatus | undefined {
  return myAttendee(o)?.status;
}

export const STATUS_LABEL: Record<ParticipationStatus, string> = {
  needs_action: "Not answered",
  accepted: "Going",
  declined: "Not going",
  tentative: "Maybe",
  delegated: "Delegated",
};

export function occClasses(o: Occurrence): Record<string, boolean> {
  const st = o.invited ? myStatus(o) : undefined;
  return {
    "ev-busy": o.busyOnly,
    "ev-invited": o.invited,
    "ev-declined": st === "declined",
    "ev-tentative": st === "tentative" || o.event.status === "tentative",
    "ev-pending": o.invited && (!st || st === "needs_action"),
    "ev-cancelled": o.event.status === "cancelled",
    "ev-free": o.event.transparency === "transparent",
  };
}

export function occTitle(o: Occurrence): string {
  return o.busyOnly ? "Busy" : o.event.title || "(No title)";
}
