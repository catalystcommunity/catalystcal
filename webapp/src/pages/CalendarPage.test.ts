import { describe, expect, it, beforeEach } from "vitest";
import type { Occurrence } from "@catalystcal/client";
import { itemTags, seedFromSlot, toItem } from "./CalendarPage";
import { signIn, signOut } from "~/stores/auth";

const me = { userId: "u-me", domain: "example.com", subject: "me" };

function occ(over: Partial<Occurrence> = {}, ev: Partial<Occurrence["event"]> = {}): Occurrence {
  return {
    start: "2026-11-10T15:00:00Z",
    end: "2026-11-10T16:00:00Z",
    access: "owner",
    busyOnly: false,
    invited: false,
    ...over,
    event: { eventId: "e1", calendarId: "c1", title: "Standup", sequence: 0, createdAt: "", updatedAt: "", createdBy: me, ...ev } as Occurrence["event"],
  };
}

describe("calendar items", () => {
  beforeEach(() => {
    signOut();
    signIn({ token: "t", expiresAt: "2099-01-01T00:00:00Z", user: me, via: "self" });
  });

  it("gives each state as a calendar-view tag", () => {
    expect(itemTags(occ())).toEqual([]);
    expect(itemTags(occ({}, { status: "cancelled" }))).toEqual(["cancelled"]);
    expect(itemTags(occ({}, { status: "tentative" }))).toEqual(["tentative"]);
    expect(itemTags(occ({ invited: true }, { attendees: [{ person: me, status: "needs_action" }] }))).toEqual(["invited"]);
    expect(itemTags(occ({ invited: true }, { attendees: [{ person: me, status: "declined" }] }))).toEqual(["declined"]);
    expect(itemTags(occ({ invited: true }, { attendees: [{ person: me, status: "tentative" }] }))).toEqual(["tentative"]);
    expect(itemTags(occ({ invited: true }, { attendees: [{ person: me, status: "delegated" }] }))).toEqual(["delegated"]);
    expect(itemTags(occ({ invited: true }, { attendees: [{ person: me, status: "accepted" }] }))).toEqual([]);
    expect(itemTags(occ({ busyOnly: true }, { status: "cancelled" }))).toEqual([]);
    expect(toItem(occ({ invited: true }), "UTC").title).toBe("Standup");
  });

  it("gives all-day items an end date after the last day", () => {
    const item = toItem(occ({ start: "2026-11-10T00:00:00Z", end: "2026-11-12T00:00:00Z" }, { allDay: true, startDate: "2026-11-10", endDate: "2026-11-12" }), "UTC");
    expect(item).toMatchObject({ allDay: true, start: "2026-11-10", end: "2026-11-12" });
  });

  it("lets the owner move an event, and not an invitation or busy time", () => {
    expect(toItem(occ(), "UTC").editable).toBe(true);
    expect(toItem(occ({ invited: true }), "UTC").editable).toBe(false);
    expect(toItem(occ({ busyOnly: true }), "UTC").editable).toBe(false);
    expect(toItem(occ({ access: "read" }), "UTC").editable).toBe(false);
  });

  it("names the series of a repeating occurrence", () => {
    expect(toItem(occ({}, { recurrence: { rrule: "FREQ=WEEKLY" } }), "UTC").seriesId).toBe("e1");
    expect(toItem(occ({}, { masterEventId: "m1" }), "UTC").seriesId).toBe("m1");
    expect(toItem(occ(), "UTC").seriesId).toBeUndefined();
  });
});

describe("seedFromSlot", () => {
  it("turns a timed selection into the wall clock of the zone", () => {
    expect(seedFromSlot({ start: "2026-11-10T16:00:00Z", end: "2026-11-10T17:30:00Z", allDay: false }, "America/Denver")).toEqual({
      date: "2026-11-10",
      startMin: 9 * 60,
      endDate: "2026-11-10",
      endMin: 10 * 60 + 30,
    });
  });

  it("turns an all-day selection into an inclusive last day", () => {
    expect(seedFromSlot({ start: "2026-11-10", end: "2026-11-13", allDay: true }, "UTC")).toEqual({ date: "2026-11-10", endDate: "2026-11-12", allDay: true });
    expect(seedFromSlot({ start: "2026-11-10", end: "2026-11-11", allDay: true }, "UTC")).toEqual({ date: "2026-11-10", endDate: "2026-11-10", allDay: true });
  });
});
