import type { Calendar, PersonSummary } from "@catalystcal/client";
import { personName } from "~/lib/people";

export interface CalendarGroup {
  key: string;
  label: string;
  calendars: Calendar[];
}

/**
 * Sidebar groups. `me` is the person whose authority the bearer uses (the
 * principal while acting). Calendars reached through a delegation, when not
 * acting, group under "Acting as <owner>".
 */
export function groupCalendars(cals: Calendar[], me: PersonSummary | null, acting: boolean): CalendarGroup[] {
  const mine: Calendar[] = [];
  const shared: Calendar[] = [];
  const subs: Calendar[] = [];
  const delegated = new Map<string, { owner: PersonSummary; list: Calendar[] }>();
  for (const c of cals) {
    const own = !!me && c.owner.userId === me.userId;
    if (c.kind === "subscription" && own) subs.push(c);
    else if (own) mine.push(c);
    else if (c.accessPath === "delegation") {
      const g = delegated.get(c.owner.userId) ?? { owner: c.owner, list: [] };
      g.list.push(c);
      delegated.set(c.owner.userId, g);
    } else shared.push(c);
  }
  const byDefaultThenName = (a: Calendar, b: Calendar) => Number(b.isDefault) - Number(a.isDefault) || a.name.localeCompare(b.name);
  const groups: CalendarGroup[] = [];
  if (mine.length || !acting) groups.push({ key: "mine", label: acting && me ? `Acting as ${personName(me)}` : "My calendars", calendars: mine.sort(byDefaultThenName) });
  if (shared.length) groups.push({ key: "shared", label: "Shared with me", calendars: shared.sort(byDefaultThenName) });
  if (subs.length) groups.push({ key: "subs", label: "Subscriptions", calendars: subs.sort(byDefaultThenName) });
  for (const [id, g] of delegated) groups.push({ key: `deleg:${id}`, label: `Acting as ${personName(g.owner)}`, calendars: g.list.sort(byDefaultThenName) });
  return groups;
}
