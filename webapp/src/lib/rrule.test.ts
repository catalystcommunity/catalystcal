import { describe, expect, it } from "vitest";
import { RecurrenceForm, buildRRule, defaultRecurrenceForm, describeRecurrence, parseRRule } from "./rrule";

const TZ = "America/Denver";
const start = "2026-10-15"; // Thursday, the 3rd Thursday
const opts = { tz: TZ, allDay: false };
const form = (over: Partial<RecurrenceForm>): RecurrenceForm => ({ ...defaultRecurrenceForm(start), ...over });

describe("buildRRule", () => {
  it("returns null for no repeat", () => {
    expect(buildRRule(form({ freq: "none" }), opts)).toBeNull();
  });

  it("builds daily, weekly, monthly and yearly rules", () => {
    expect(buildRRule(form({ freq: "daily" }), opts)).toBe("FREQ=DAILY");
    expect(buildRRule(form({ freq: "daily", interval: 3 }), opts)).toBe("FREQ=DAILY;INTERVAL=3");
    expect(buildRRule(form({ freq: "weekly", byDay: ["WE", "MO"] }), opts)).toBe("FREQ=WEEKLY;BYDAY=MO,WE");
    expect(buildRRule(form({ freq: "monthly" }), opts)).toBe("FREQ=MONTHLY;BYMONTHDAY=15");
    expect(buildRRule(form({ freq: "monthly", monthlyMode: "nth" }), opts)).toBe("FREQ=MONTHLY;BYDAY=3TH");
    expect(buildRRule(form({ freq: "monthly", monthlyMode: "nth", nth: -1, nthDay: "FR" }), opts)).toBe("FREQ=MONTHLY;BYDAY=-1FR");
    expect(buildRRule(form({ freq: "yearly", interval: 2 }), opts)).toBe("FREQ=YEARLY;INTERVAL=2");
  });

  it("writes COUNT and UNTIL", () => {
    expect(buildRRule(form({ freq: "weekly", ends: "after", count: 6 }), opts)).toBe("FREQ=WEEKLY;BYDAY=TH;COUNT=6");
    // The end of 2026-12-31 in Denver (MST, -7) is 2027-01-01T06:59:59Z.
    expect(buildRRule(form({ freq: "daily", ends: "on", until: "2026-12-31" }), opts)).toBe("FREQ=DAILY;UNTIL=20270101T065959Z");
    expect(buildRRule(form({ freq: "daily", ends: "on", until: "2026-12-31" }), { tz: TZ, allDay: true })).toBe("FREQ=DAILY;UNTIL=20261231");
  });

  it("passes custom text through without the prefix", () => {
    expect(buildRRule(form({ freq: "custom", custom: " RRULE:freq=monthly;bysetpos=-1;byday=mo,tu,we,th,fr " }), opts)).toBe(
      "FREQ=MONTHLY;BYSETPOS=-1;BYDAY=MO,TU,WE,TH,FR",
    );
    expect(buildRRule(form({ freq: "custom", custom: "  " }), opts)).toBeNull();
  });
});

describe("parseRRule", () => {
  it("defaults to no repeat", () => {
    expect(parseRRule(undefined, start, TZ).freq).toBe("none");
    expect(parseRRule("", start, TZ).freq).toBe("none");
  });

  it("round-trips every form shape", () => {
    const shapes: Partial<RecurrenceForm>[] = [
      { freq: "daily" },
      { freq: "daily", interval: 2, ends: "after", count: 5 },
      { freq: "weekly", byDay: ["MO", "WE", "FR"] },
      { freq: "weekly", interval: 2, byDay: ["TH"], ends: "on", until: "2027-03-01" },
      { freq: "monthly", monthlyMode: "day", monthDay: 31 },
      { freq: "monthly", monthlyMode: "nth", nth: 2, nthDay: "TU" },
      { freq: "monthly", monthlyMode: "nth", nth: -1, nthDay: "FR", ends: "after", count: 12 },
      { freq: "yearly" },
    ];
    for (const s of shapes) {
      const f = form(s);
      const rule = buildRRule(f, opts)!;
      const back = parseRRule(rule, start, TZ);
      expect(back.freq, rule).toBe(f.freq);
      expect(buildRRule(back, opts), rule).toBe(rule);
      if (s.ends === "on") expect(back.until).toBe(s.until);
      if (s.ends === "after") expect(back.count).toBe(s.count);
    }
  });

  it("reads lower case, a prefix, and WKST", () => {
    const f = parseRRule("RRULE:freq=weekly;wkst=MO;byday=tu,th", start, TZ);
    expect(f.freq).toBe("weekly");
    expect(f.byDay).toEqual(["TU", "TH"]);
  });

  it("reads a date-only UNTIL and a yearly BYMONTH of the start month", () => {
    expect(parseRRule("FREQ=DAILY;UNTIL=20261231", start, TZ).until).toBe("2026-12-31");
    expect(parseRRule("FREQ=YEARLY;BYMONTH=10", start, TZ).freq).toBe("yearly");
  });

  it("falls back to custom text for rules the form cannot show", () => {
    const cases = [
      "FREQ=MONTHLY;BYSETPOS=-1;BYDAY=MO,TU,WE,TH,FR",
      "FREQ=WEEKLY;BYDAY=1MO",
      "FREQ=MONTHLY;BYMONTHDAY=1,15",
      "FREQ=MONTHLY;BYDAY=2TU;BYMONTHDAY=10",
      "FREQ=DAILY;BYHOUR=9,17",
      "FREQ=DAILY;BYDAY=MO",
      "FREQ=HOURLY",
      "FREQ=DAILY;COUNT=3;UNTIL=20261231",
      "FREQ=YEARLY;BYMONTH=3",
      "FREQ=DAILY;INTERVAL=0",
      "garbage",
    ];
    for (const c of cases) {
      const f = parseRRule(c, start, TZ);
      expect(f.freq, c).toBe("custom");
      expect(f.custom, c).toBe(c.toUpperCase());
      expect(buildRRule(f, opts), c).toBe(c.toUpperCase());
    }
  });
});

describe("describeRecurrence", () => {
  it("says what the rule does", () => {
    expect(describeRecurrence(form({ freq: "none" }), start)).toBe("Does not repeat");
    expect(describeRecurrence(form({ freq: "weekly", byDay: ["MO", "TU", "WE", "TH", "FR"] }), start)).toBe("Weekly on weekdays");
    expect(describeRecurrence(form({ freq: "weekly", interval: 2, byDay: ["MO", "WE"], ends: "after", count: 10 }), start)).toBe(
      "Every 2 weeks on Mon, Wed, 10 times",
    );
    expect(describeRecurrence(form({ freq: "monthly", monthlyMode: "nth", nth: -1, nthDay: "FR" }), start)).toBe("Monthly on the last Friday");
    expect(describeRecurrence(form({ freq: "yearly", ends: "on", until: "2030-10-15" }), start)).toBe("Yearly on Oct 15, 2026, until Oct 15, 2030");
  });
});
