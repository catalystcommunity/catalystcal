import { describe, expect, it } from "vitest";
import type { Calendar, PersonSummary } from "@catalystcal/client";
import { groupCalendars } from "./calendarGroups";

const me: PersonSummary = { userId: "me", domain: "x.com", subject: "me", displayName: "Me" };
const pat: PersonSummary = { userId: "pat", domain: "x.com", subject: "pat", displayName: "Pat" };
const c = (id: string, owner: PersonSummary, extra: Partial<Calendar> = {}): Calendar =>
  ({ calendarId: id, name: id, owner, kind: "local", isDefault: false, access: "owner", accessPath: "owner", createdAt: "", updatedAt: "", ...extra }) as Calendar;

describe("groupCalendars", () => {
  it("groups mine, shared, subscriptions, and delegated calendars", () => {
    const g = groupCalendars(
      [c("b", me), c("a", me, { isDefault: true }), c("s", pat, { accessPath: "share", access: "read" }), c("feed", me, { kind: "subscription" }), c("d", pat, { accessPath: "delegation" })],
      me,
      false,
    );
    expect(g.map((x) => x.label)).toEqual(["My calendars", "Shared with me", "Subscriptions", "Acting as Pat"]);
    expect(g[0].calendars.map((x) => x.calendarId)).toEqual(["a", "b"]);
  });

  it("labels the principal's calendars while acting", () => {
    const g = groupCalendars([c("p", pat)], pat, true);
    expect(g[0].label).toBe("Acting as Pat");
  });
});
