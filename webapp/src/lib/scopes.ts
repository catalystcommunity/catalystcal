import type { Scope } from "@catalystcal/client";

export const SCOPES: { scope: Scope; label: string; detail: string }[] = [
  { scope: "calendars.read", label: "See my calendars and event details", detail: "Includes private events." },
  { scope: "freebusy.read", label: "See when I’m busy", detail: "Only busy and free time, no details." },
  { scope: "events.write", label: "Create and change events as me", detail: "Invitations go out in my name." },
  { scope: "events.respond", label: "Answer invitations as me", detail: "Accept, decline, or say maybe for me." },
  { scope: "calendars.manage", label: "Manage calendar settings and sharing", detail: "Rename, recolor, share, and publish my calendars." },
];

export const SCOPE_SHORT: Record<Scope, string> = {
  "calendars.read": "See details",
  "freebusy.read": "See busy times",
  "events.write": "Change events",
  "events.respond": "Answer invitations",
  "calendars.manage": "Manage calendars",
};

/** calendars.read includes freebusy.read; keep the list tidy and ordered. */
export function normalizeScopes(list: Scope[]): Scope[] {
  const set = new Set(list);
  if (set.has("calendars.read")) set.add("freebusy.read");
  return SCOPES.map((s) => s.scope).filter((s) => set.has(s));
}
