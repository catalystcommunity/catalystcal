import { describe, expect, it } from "vitest";
import { addDays, addMonths, dayOfWeek, daysBetween, monthGrid, nthWeekdayOfMonth, startOfWeek, weekDays } from "./dates";

describe("civil date math", () => {
  it("adds days across month and year ends and leap days", () => {
    expect(addDays("2026-10-31", 1)).toBe("2026-11-01");
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
    expect(addDays("2028-02-28", 1)).toBe("2028-02-29");
    expect(addDays("2027-02-28", 1)).toBe("2027-03-01");
    expect(addDays("2026-03-01", -1)).toBe("2026-02-28");
  });

  it("counts days and weekdays", () => {
    expect(daysBetween("2026-10-01", "2026-11-01")).toBe(31);
    expect(daysBetween("2026-11-01", "2026-10-01")).toBe(-31);
    expect(dayOfWeek("2026-10-15")).toBe(4); // Thursday
    expect(dayOfWeek("1970-01-01")).toBe(4);
    expect(dayOfWeek("1969-12-28")).toBe(0);
  });

  it("clamps the day of month when adding months", () => {
    expect(addMonths("2026-01-31", 1)).toBe("2026-02-28");
    expect(addMonths("2028-01-31", 1)).toBe("2028-02-29");
    expect(addMonths("2026-12-15", 1)).toBe("2027-01-15");
    expect(addMonths("2026-01-15", -1)).toBe("2025-12-15");
  });

  it("finds the week start for Sunday and Monday weeks", () => {
    expect(startOfWeek("2026-10-15", 0)).toBe("2026-10-11");
    expect(startOfWeek("2026-10-15", 1)).toBe("2026-10-12");
    expect(startOfWeek("2026-10-11", 1)).toBe("2026-10-05");
    expect(weekDays("2026-12-31", 0)).toEqual(["2026-12-27", "2026-12-28", "2026-12-29", "2026-12-30", "2026-12-31", "2027-01-01", "2027-01-02"]);
  });

  it("builds month grids that cover the whole month", () => {
    // October 2026 starts on a Thursday and has 31 days.
    const oct = monthGrid("2026-10-15", 0);
    expect(oct.length).toBe(5);
    expect(oct[0][0]).toBe("2026-09-27");
    expect(oct[4][6]).toBe("2026-10-31");
    // February 2026 starts on Sunday and has 28 days: exactly 4 rows.
    const feb = monthGrid("2026-02-10", 0);
    expect(feb.length).toBe(4);
    expect(feb[0][0]).toBe("2026-02-01");
    expect(feb[3][6]).toBe("2026-02-28");
    // August 2026 with Monday weeks needs 6 rows (starts Saturday, 31 days).
    const aug = monthGrid("2026-08-01", 1);
    expect(aug.length).toBe(6);
    expect(aug[0][0]).toBe("2026-07-27");
    expect(aug[5][6]).toBe("2026-09-06");
    for (const w of aug) expect(w.length).toBe(7);
  });

  it("finds the nth weekday", () => {
    expect(nthWeekdayOfMonth("2026-10-15")).toEqual({ nth: 3, isLast: false });
    expect(nthWeekdayOfMonth("2026-10-29")).toEqual({ nth: 5, isLast: true });
    expect(nthWeekdayOfMonth("2026-10-25")).toEqual({ nth: 4, isLast: true });
  });
});
