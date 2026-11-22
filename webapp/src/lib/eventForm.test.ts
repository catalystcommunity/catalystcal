import { describe, expect, it } from "vitest";
import { fromEventCbor, toEventCbor, type Event, type Occurrence } from "@catalystcal/client";
import { buildDelete, buildUpdate, formFromOccurrence, formToEvent, newEventForm, validateForm } from "./eventForm";

const TZ = "America/Denver";

function ev(over: Partial<Event> = {}): Event {
  return {
    eventId: "e1",
    calendarId: "c1",
    title: "Standup",
    start: new Date("2026-10-01T15:00:00Z"),
    end: new Date("2026-10-01T15:30:00Z"),
    timezone: TZ,
    sequence: 3,
    createdAt: new Date("2026-09-01T00:00:00Z"),
    updatedAt: new Date("2026-09-02T00:00:00Z"),
    organizer: { userId: "u1", domain: "example.com", subject: "s1" },
    ...over,
  };
}

function occ(e: Event, start: string, end: string, recurrenceId?: string): Occurrence {
  return { event: e, start: new Date(start), end: new Date(end), recurrenceId: recurrenceId ? new Date(recurrenceId) : undefined, access: "owner", busyOnly: false, invited: false };
}

describe("event form", () => {
  it("makes a timed event in the form's zone", () => {
    const f = newEventForm({ date: "2026-10-15", startMin: 9 * 60, calendarId: "c1", timezone: TZ, title: "Lunch" });
    const e = formToEvent(f);
    expect(e.start).toEqual(new Date("2026-10-15T15:00:00Z"));
    expect(e.end).toEqual(new Date("2026-10-15T16:00:00Z"));
    expect(e.eventId).toBe("");
    expect(e.sequence).toBe(0);
    expect(e.recurrence).toBeUndefined();
    expect(e.organizer).toBeUndefined();
  });

  it("makes an all-day event with an exclusive end date", () => {
    const f = newEventForm({ date: "2026-10-15", endDate: "2026-10-16", allDay: true, calendarId: "c1", timezone: TZ, title: "Trip" });
    const e = formToEvent(f);
    expect(e.startDate).toBe("2026-10-15");
    expect(e.endDate).toBe("2026-10-17");
    expect(e.start).toBeUndefined();
  });

  it("round-trips all-day boundaries and recurrence instants through the client codec", () => {
    const form = newEventForm({ date: "2028-02-29", allDay: true, calendarId: "c1", timezone: TZ, title: "Leap day" });
    const event = formToEvent(form);
    event.recurrence = { rrule: "FREQ=YEARLY", exdates: [new Date("2032-02-29T00:00:00Z")] };
    const decoded = fromEventCbor(toEventCbor(event));
    expect(decoded.startDate).toBe("2028-02-29");
    expect(decoded.endDate).toBe("2028-03-01");
    expect(decoded.recurrence).toEqual(event.recurrence);
    expect(() => toEventCbor({ ...event, startDate: "2027-02-29" })).toThrow();
  });

  it("rolls a late start past midnight", () => {
    const f = newEventForm({ date: "2026-10-15", startMin: 23 * 60 + 30, calendarId: "c1", timezone: TZ });
    expect(f.endDate).toBe("2026-10-16");
    expect(f.endMin).toBe(30);
  });

  it("validates", () => {
    const f = newEventForm({ date: "2026-10-15", calendarId: "c1", timezone: TZ });
    expect(validateForm(f)).toMatch(/title/);
    f.title = "x";
    expect(validateForm(f)).toBeNull();
    f.endMin = f.startMin;
    expect(validateForm(f)).toMatch(/end time/);
  });

  it("reads an occurrence back and keeps attendees writable", () => {
    const e = ev({
      recurrence: { rrule: "FREQ=WEEKLY;BYDAY=TH" },
      attendees: [{ identity: { domain: "example.com", handle: "bob" }, status: "accepted", person: { userId: "u2", domain: "example.com", subject: "s2" } }],
    });
    const f = formFromOccurrence(occ(e, "2026-10-15T15:00:00Z", "2026-10-15T15:30:00Z", "2026-10-15T15:00:00Z"), "UTC");
    expect(f.startDate).toBe("2026-10-15");
    expect(f.startMin).toBe(9 * 60);
    expect(f.timezone).toBe(TZ);
    expect(f.recurrence.freq).toBe("weekly");
    const out = formToEvent(f, e);
    expect(out.attendees?.[0].person).toBeUndefined();
    expect(out.attendees?.[0].status).toBe("accepted");
    expect(out.sequence).toBe(3);
  });
});

describe("series edits", () => {
  const master = ev({ recurrence: { rrule: "FREQ=WEEKLY;BYDAY=TH" } });
  const o = occ(master, "2026-10-15T15:00:00Z", "2026-10-15T15:30:00Z", "2026-10-15T15:00:00Z");

  it("this: an override without a rule, with the recurrence id", () => {
    const f = formFromOccurrence(o, TZ);
    f.title = "Moved standup";
    f.startMin = 10 * 60;
    f.endMin = 10 * 60 + 30;
    const req = buildUpdate(f, o, "this");
    expect(req.scope).toBe("this");
    expect(req.recurrenceId).toEqual(new Date("2026-10-15T15:00:00Z"));
    expect(req.event.recurrence).toBeUndefined();
    expect(req.event.start).toEqual(new Date("2026-10-15T16:00:00Z"));
  });

  it("all: a time change on one instance moves the series start", () => {
    const f = formFromOccurrence(o, TZ);
    f.startMin = 10 * 60;
    f.endMin = 11 * 60;
    const req = buildUpdate(f, o, "all");
    expect(req.scope).toBe("all");
    expect(req.recurrenceId).toBeUndefined();
    expect(req.event.start).toEqual(new Date("2026-10-01T16:00:00Z"));
    expect(req.event.end).toEqual(new Date("2026-10-01T17:00:00Z"));
    expect(req.event.recurrence?.rrule).toBe("FREQ=WEEKLY;BYDAY=TH");
  });

  it("this_and_future: keeps the rule and names the instance", () => {
    const f = formFromOccurrence(o, TZ);
    f.recurrence = { ...f.recurrence, interval: 2 };
    const req = buildUpdate(f, o, "this_and_future");
    expect(req.scope).toBe("this_and_future");
    expect(req.recurrenceId).toEqual(new Date("2026-10-15T15:00:00Z"));
    expect(req.event.recurrence?.rrule).toBe("FREQ=WEEKLY;INTERVAL=2;BYDAY=TH");
    expect(req.event.start).toEqual(new Date("2026-10-15T15:00:00Z"));
  });

  it("deletes with a scope", () => {
    expect(buildDelete(o, "this")).toEqual({ eventId: "e1", scope: "this", recurrenceId: new Date("2026-10-15T15:00:00Z") });
    expect(buildDelete(o, "all")).toEqual({ eventId: "e1", scope: "all" });
    const single = occ(ev(), "2026-10-01T15:00:00Z", "2026-10-01T15:30:00Z");
    expect(buildDelete(single, "this")).toEqual({ eventId: "e1", scope: "all" });
  });
});
