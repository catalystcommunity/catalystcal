import { describe, expect, it } from "vitest";
import type { Event, Occurrence } from "@catalystcal/client";
import { placeOccurrence } from "./occurrence";

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

describe("placeOccurrence", () => {
  it("places a timed event on its local date", () => {
    const p = placeOccurrence(o({}, "2026-10-16T03:00:00Z", "2026-10-16T04:00:00Z"), "America/Denver");
    expect(p.startDate).toBe("2026-10-15");
    expect(p.lastDate).toBe("2026-10-15");
    expect(p.bar).toBe(false);
  });

  it("does not spill an event that ends at midnight into the next day", () => {
    const p = placeOccurrence(o({}, "2026-10-15T22:00:00Z", "2026-10-16T00:00:00Z"), "UTC");
    expect(p.lastDate).toBe("2026-10-15");
  });

  it("uses the event's own dates for a single all-day event", () => {
    const p = placeOccurrence(o({ allDay: true, startDate: "2026-10-15", endDate: "2026-10-18" }, "2026-10-15T06:00:00Z", "2026-10-18T06:00:00Z"), "Asia/Tokyo");
    expect(p.startDate).toBe("2026-10-15");
    expect(p.lastDate).toBe("2026-10-17");
    expect(p.bar).toBe(true);
  });

  it("reads all-day instances written at midnight UTC or at local midnight", () => {
    const series = { allDay: true, startDate: "2026-10-01", endDate: "2026-10-02", recurrence: { rrule: "FREQ=WEEKLY" }, timezone: "America/Denver" };
    expect(placeOccurrence(o(series, "2026-10-15T00:00:00Z", "2026-10-16T00:00:00Z", "2026-10-15T00:00:00Z"), "America/Denver").startDate).toBe("2026-10-15");
    expect(placeOccurrence(o(series, "2026-10-15T06:00:00Z", "2026-10-16T06:00:00Z", "2026-10-15T06:00:00Z"), "Asia/Tokyo").startDate).toBe("2026-10-15");
  });

  it("draws 24 hours or more as a bar", () => {
    const p = placeOccurrence(o({}, "2026-10-15T12:00:00Z", "2026-10-17T12:00:00Z"), "UTC");
    expect(p.bar).toBe(true);
    expect(p.lastDate).toBe("2026-10-17");
  });
});
