import { describe, expect, it } from "vitest";
import { civilDate, formatHourLabel, formatTime, minutesIntoDay, offsetLabel, startOfDayUtc, toTimestamp, tzOffsetMinutes, zoneList, zonedToUtc, roundUpToSlot } from "./tz";

const H = 3_600_000;

describe("time zone helpers", () => {
  it("computes offsets on both sides of a DST change", () => {
    expect(tzOffsetMinutes(Date.UTC(2026, 6, 1), "America/Denver")).toBe(-360);
    expect(tzOffsetMinutes(Date.UTC(2026, 0, 1), "America/Denver")).toBe(-420);
    expect(tzOffsetMinutes(Date.UTC(2026, 6, 1), "Europe/Berlin")).toBe(120);
    expect(tzOffsetMinutes(Date.UTC(2026, 6, 1), "Asia/Kolkata")).toBe(330);
  });

  it("maps wall-clock time to UTC", () => {
    expect(toTimestamp(zonedToUtc("2026-10-15", 9 * 60, "America/Denver"))).toBe("2026-10-15T15:00:00Z");
    expect(toTimestamp(zonedToUtc("2026-01-15", 9 * 60, "America/Denver"))).toBe("2026-01-15T16:00:00Z");
    expect(toTimestamp(zonedToUtc("2026-10-15", 0, "Asia/Tokyo"))).toBe("2026-10-14T15:00:00Z");
  });

  it("moves a time in the spring-forward gap forward", () => {
    // 2026-03-08 02:30 does not exist in Denver; it becomes 03:30 MDT.
    expect(toTimestamp(zonedToUtc("2026-03-08", 150, "America/Denver"))).toBe("2026-03-08T09:30:00Z");
  });

  it("picks the earlier instant in the fall-back overlap", () => {
    // 2026-11-01 01:30 happens twice in Denver; the first is MDT.
    expect(toTimestamp(zonedToUtc("2026-11-01", 90, "America/Denver"))).toBe("2026-11-01T07:30:00Z");
    // 2026-10-25 02:30 happens twice in Berlin; the first is CEST.
    expect(toTimestamp(zonedToUtc("2026-10-25", 150, "Europe/Berlin"))).toBe("2026-10-25T00:30:00Z");
  });

  it("has 23- and 25-hour days at the DST changes", () => {
    const tz = "America/Denver";
    expect(startOfDayUtc("2026-03-09", tz) - startOfDayUtc("2026-03-08", tz)).toBe(23 * H);
    expect(startOfDayUtc("2026-11-02", tz) - startOfDayUtc("2026-11-01", tz)).toBe(25 * H);
    expect(startOfDayUtc("2026-10-16", tz) - startOfDayUtc("2026-10-15", tz)).toBe(24 * H);
  });

  it("reads civil dates and wall minutes back", () => {
    const ms = Date.parse("2026-10-16T03:30:00Z");
    expect(civilDate(ms, "America/Denver")).toBe("2026-10-15");
    expect(minutesIntoDay(ms, "America/Denver")).toBe(21 * 60 + 30);
    expect(civilDate(ms, "UTC")).toBe("2026-10-16");
  });

  it("formats times on 12- and 24-hour clocks", () => {
    const ms = Date.parse("2026-10-15T15:30:00Z");
    expect(formatTime(ms, "America/Denver", { hour12: true })).toBe("9:30 AM");
    expect(formatTime(ms, "America/Denver", { hour12: false })).toBe("09:30");
    expect(formatTime(Date.parse("2026-10-15T18:00:00Z"), "America/Denver", { hour12: true })).toBe("12 PM");
    expect(formatTime(Date.parse("2026-10-15T06:00:00Z"), "America/Denver", { hour12: true })).toBe("12 AM");
    expect(formatHourLabel(0, true)).toBe("12 AM");
    expect(formatHourLabel(13, false)).toBe("13:00");
    expect(offsetLabel(Date.UTC(2026, 6, 1), "America/Denver")).toBe("UTC−06:00");
  });

  it("writes CSIL timestamps with whole seconds", () => {
    expect(toTimestamp(Date.parse("2026-10-15T17:00:00.789Z"))).toBe("2026-10-15T17:00:00Z");
  });

  it("puts the chosen zone first in the zone list", () => {
    const list = zoneList("America/Denver");
    expect(list[0]).toBe("America/Denver");
    expect(list.filter((z) => z === "America/Denver").length).toBe(1);
    expect(list).toContain("UTC");
  });

  it("rounds up to the next slot, across midnight", () => {
    expect(roundUpToSlot(Date.parse("2026-10-15T15:10:00Z"), "UTC")).toEqual({ date: "2026-10-15", minutes: 15 * 60 + 30 });
    expect(roundUpToSlot(Date.parse("2026-10-15T23:50:00Z"), "UTC")).toEqual({ date: "2026-10-16", minutes: 0 });
  });
});
