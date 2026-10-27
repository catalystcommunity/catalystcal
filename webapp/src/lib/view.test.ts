import { describe, expect, it } from "vitest";
import { stepCursor, viewTitle, visibleRange } from "./view";
import { toTimestamp } from "./tz";

describe("visible ranges", () => {
  it("spans a DST week as 167 hours", () => {
    const r = visibleRange("week", "2026-03-10", "America/Denver", 0);
    expect(r.days[0]).toBe("2026-03-08");
    expect(toTimestamp(r.startMs)).toBe("2026-03-08T07:00:00Z");
    expect(toTimestamp(r.endMs)).toBe("2026-03-15T06:00:00Z");
    expect((r.endMs - r.startMs) / 3_600_000).toBe(167);
  });

  it("spans a fall-back week as 169 hours", () => {
    const r = visibleRange("week", "2026-11-03", "America/Denver", 0);
    expect((r.endMs - r.startMs) / 3_600_000).toBe(169);
  });

  it("covers whole month rows across month boundaries", () => {
    const r = visibleRange("month", "2026-10-15", "UTC", 1);
    expect(r.weeks?.length).toBe(5);
    expect(r.days[0]).toBe("2026-09-28");
    expect(r.days[r.days.length - 1]).toBe("2026-11-01");
    expect(toTimestamp(r.endMs)).toBe("2026-11-02T00:00:00Z");
  });

  it("starts a narrow week at the cursor", () => {
    const r = visibleRange("week", "2026-10-15", "UTC", 0, 3);
    expect(r.days).toEqual(["2026-10-15", "2026-10-16", "2026-10-17"]);
    expect(stepCursor("week", "2026-10-15", 1, 3)).toBe("2026-10-18");
  });

  it("steps each view", () => {
    expect(stepCursor("month", "2026-01-31", 1)).toBe("2026-02-28");
    expect(stepCursor("week", "2026-10-15", -1)).toBe("2026-10-08");
    expect(stepCursor("day", "2026-12-31", 1)).toBe("2027-01-01");
    expect(stepCursor("agenda", "2026-10-01", 1)).toBe("2026-10-31");
  });

  it("titles ranges that cross months and years", () => {
    const tz = "UTC";
    expect(viewTitle("week", visibleRange("week", "2026-10-15", tz, 0), "2026-10-15")).toBe("October 11 – 17, 2026");
    expect(viewTitle("week", visibleRange("week", "2026-09-30", tz, 0), "2026-09-30")).toBe("Sep 27 – Oct 3, 2026");
    expect(viewTitle("week", visibleRange("week", "2026-11-01", tz, 0), "2026-11-01")).toBe("November 1 – 7, 2026");
    expect(viewTitle("week", visibleRange("week", "2026-12-31", tz, 0), "2026-12-31")).toBe("Dec 27, 2026 – Jan 2, 2027");
  });
});
