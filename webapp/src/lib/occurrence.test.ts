import { describe, expect, it } from "vitest";
import type { Event, Occurrence } from "@catalystcal/client";
import { allDayLength, allDayStart, occurrenceKey } from "./occurrence";

const base: Event = { eventId: "e", calendarId: "c", title: "t", sequence: 0, createdAt: "", updatedAt: "" };
const o = (e: Partial<Event>, start: string, end: string, recurrenceId?: string): Occurrence => ({
  event: { ...base, ...e },
  start,
  end,
  recurrenceId,
  access: "owner",
  busyOnly: false,
  invited: false,
});

describe("all-day occurrences", () => {
  it("uses the event's own dates for a single all-day event", () => {
    const occ = o({ allDay: true, startDate: "2026-10-15", endDate: "2026-10-18" }, "2026-10-15T06:00:00Z", "2026-10-18T06:00:00Z");
    expect(allDayStart(occ, "Asia/Tokyo")).toBe("2026-10-15");
    expect(allDayLength(occ)).toBe(3);
  });

  it("reads all-day instances written at midnight UTC or at local midnight", () => {
    const series = { allDay: true, startDate: "2026-10-01", endDate: "2026-10-02", recurrence: { rrule: "FREQ=WEEKLY" }, timezone: "America/Denver" };
    expect(allDayStart(o(series, "2026-10-15T00:00:00Z", "2026-10-16T00:00:00Z", "2026-10-15T00:00:00Z"), "America/Denver")).toBe("2026-10-15");
    expect(allDayStart(o(series, "2026-10-15T06:00:00Z", "2026-10-16T06:00:00Z", "2026-10-15T06:00:00Z"), "Asia/Tokyo")).toBe("2026-10-15");
  });

  it("counts the days from the instants when the event has no dates", () => {
    expect(allDayLength(o({ allDay: true }, "2026-10-15T00:00:00Z", "2026-10-17T00:00:00Z"))).toBe(2);
  });
});

describe("occurrenceKey", () => {
  it("is the event and the instance", () => {
    expect(occurrenceKey(o({}, "2026-10-15T00:00:00Z", "2026-10-15T01:00:00Z"))).toBe("e|2026-10-15T00:00:00Z");
    expect(occurrenceKey(o({}, "2026-10-16T00:00:00Z", "2026-10-16T01:00:00Z", "2026-10-15T00:00:00Z"))).toBe("e|2026-10-15T00:00:00Z");
  });
});
