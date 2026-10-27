/**
 * The caller's calendar list, the hidden-calendar set (per browser), and an
 * event refresh counter that views key their fetches on.
 */
import { createSignal } from "solid-js";
import type { AccessRole, Calendar } from "@catalystcal/client";
import { calendarClient } from "~/data/clients";
import { readJson, writeJson } from "~/lib/storage";
import { errorMessage } from "~/transport/csilrpc";

const HIDDEN_KEY = "catalystcal.hiddenCalendars";

const [calendars, setCalendars] = createSignal<Calendar[]>([]);
const [loaded, setLoaded] = createSignal(false);
const [error, setError] = createSignal<string | null>(null);
const [hidden, setHiddenSig] = createSignal<Set<string>>(new Set(readJson<string[]>(HIDDEN_KEY, [])));
const [eventsVersion, setEventsVersion] = createSignal(0);

export const useCalendars = () => calendars;
export const useCalendarsLoaded = () => loaded;
export const useCalendarsError = () => error;
export const useHidden = () => hidden;
export const useEventsVersion = () => eventsVersion;

/** Views refetch events when this changes. Call after any mutation. */
export const bumpEvents = () => setEventsVersion((v) => v + 1);

export async function loadCalendars(): Promise<void> {
  try {
    const list = await calendarClient.listCalendars({});
    setCalendars(list.calendars ?? []);
    setError(null);
  } catch (e) {
    setError(errorMessage(e));
  } finally {
    setLoaded(true);
  }
}

export function clearCalendars(): void {
  setCalendars([]);
  setLoaded(false);
}

export function upsertCalendar(c: Calendar): void {
  setCalendars((list) => {
    const i = list.findIndex((x) => x.calendarId === c.calendarId);
    if (i < 0) return [...list, c];
    const next = list.slice();
    next[i] = c;
    return next;
  });
}

export function removeCalendar(id: string): void {
  setCalendars((list) => list.filter((c) => c.calendarId !== id));
}

export function toggleHidden(id: string): void {
  const next = new Set(hidden());
  if (next.has(id)) next.delete(id);
  else next.add(id);
  setHiddenSig(next);
  writeJson(HIDDEN_KEY, [...next]);
}

export const calendarById = (id: string): Calendar | undefined => calendars().find((c) => c.calendarId === id);

const RANK: Record<AccessRole, number> = { none: 0, free_busy: 1, read: 2, write: 3, manage: 4, owner: 5 };

/** True when `have` is at least `need`. */
export function accessAtLeast(have: AccessRole | undefined, need: AccessRole): boolean {
  return RANK[have ?? "none"] >= RANK[need];
}

export const DEFAULT_COLOR = "#4f6bd8";
export const calendarColor = (c: Calendar | undefined): string => c?.color || DEFAULT_COLOR;

/** Preset swatches for calendars and events. */
export const SWATCHES = ["#4f6bd8", "#2f9e8f", "#d9822b", "#c2415d", "#8a5cd6", "#3d8b3d", "#b8860b", "#5b7083", "#e05b9a", "#1f8fbf"];

/** Where a new event goes: the principal's default calendar if writable. */
export function defaultWritableCalendar(ownerUserId: string | undefined): Calendar | undefined {
  const w = calendars().filter((c) => c.kind === "local" && accessAtLeast(c.access, "write"));
  return (
    w.find((c) => c.isDefault && c.owner.userId === ownerUserId) ??
    w.find((c) => c.owner.userId === ownerUserId && !hidden().has(c.calendarId)) ??
    w.find((c) => !hidden().has(c.calendarId)) ??
    w[0]
  );
}
