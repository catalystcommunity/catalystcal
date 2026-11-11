/**
 * The caller's calendar list, the hidden-calendar set (per browser), and an
 * event refresh counter that views key their fetches on.
 */
import type { AccessRole, Calendar } from "@catalystcal/client";
import { calendarClient } from "~/data/clients";
import { readJson, writeJson } from "~/lib/storage";
import { errorMessage } from "~/transport/csilrpc";
import { cell } from "~/state/store";

const HIDDEN_KEY = "catalystcal.hiddenCalendars";

/** The store collection of this module. */
export const CALENDARS = "calendars";

const listCell = cell<Calendar[]>(CALENDARS, "list", []);
const loadedCell = cell(CALENDARS, "loaded", false);
const errorCell = cell<string | null>(CALENDARS, "error", null);
const hiddenCell = cell<Set<string>>(CALENDARS, "hidden", new Set(readJson<string[]>(HIDDEN_KEY, [])));
const versionCell = cell(CALENDARS, "eventsVersion", 0);

export const calendars = (): Calendar[] => listCell.get();
export const calendarsLoaded = (): boolean => loadedCell.get();
export const calendarsError = (): string | null => errorCell.get();
export const hiddenCalendars = (): Set<string> => hiddenCell.get();
export const eventsVersion = (): number => versionCell.get();

/** Views refetch events when this changes. Call after any mutation. */
export const bumpEvents = () => versionCell.set(versionCell.get() + 1);

export async function loadCalendars(): Promise<void> {
  try {
    const list = await calendarClient.listCalendars({});
    listCell.set(list.calendars ?? []);
    errorCell.set(null);
  } catch (e) {
    errorCell.set(errorMessage(e));
  } finally {
    loadedCell.set(true);
  }
}

export function clearCalendars(): void {
  listCell.set([]);
  loadedCell.set(false);
}

export function upsertCalendar(c: Calendar): void {
  const list = listCell.get();
  const i = list.findIndex((x) => x.calendarId === c.calendarId);
  const next = list.slice();
  if (i < 0) next.push(c);
  else next[i] = c;
  listCell.set(next);
}

export function removeCalendar(id: string): void {
  listCell.set(listCell.get().filter((c) => c.calendarId !== id));
}

export function toggleHidden(id: string): void {
  const next = new Set(hiddenCell.get());
  if (next.has(id)) next.delete(id);
  else next.add(id);
  hiddenCell.set(next);
  writeJson(HIDDEN_KEY, [...next]);
}

export const calendarById = (id: string): Calendar | undefined => listCell.get().find((c) => c.calendarId === id);

const RANK: Record<AccessRole, number> = { none: 0, free_busy: 1, read: 2, contribute: 3, write: 4, manage: 5, owner: 6 };

/** True when `have` is at least `need`. */
export function accessAtLeast(have: AccessRole | undefined, need: AccessRole): boolean {
  return RANK[have ?? "none"] >= RANK[need];
}

/** True when the caller can add events (contribute or more). */
export function canAddEvents(have: AccessRole | undefined): boolean {
  return accessAtLeast(have, "contribute");
}

/**
 * True when the caller can change an event: write access changes every
 * event, contribute access only the events that the caller created.
 */
export function canEditEvent(have: AccessRole | undefined, createdBy: string | undefined, me: string | undefined): boolean {
  if (accessAtLeast(have, "write")) return true;
  return have === "contribute" && !!createdBy && createdBy === me;
}

export const DEFAULT_COLOR = "#4f6bd8";
export const calendarColor = (c: Calendar | undefined): string => c?.color || DEFAULT_COLOR;

/** Preset swatches for calendars and events. */
export const SWATCHES = ["#4f6bd8", "#2f9e8f", "#d9822b", "#c2415d", "#8a5cd6", "#3d8b3d", "#b8860b", "#5b7083", "#e05b9a", "#1f8fbf"];

/** Where a new event goes: the principal's default calendar if writable. */
export function defaultWritableCalendar(ownerUserId: string | undefined): Calendar | undefined {
  const w = listCell.get().filter((c) => c.kind === "local" && canAddEvents(c.access));
  const hidden = hiddenCell.get();
  return (
    w.find((c) => c.isDefault && c.owner.userId === ownerUserId) ??
    w.find((c) => c.owner.userId === ownerUserId && !hidden.has(c.calendarId)) ??
    w.find((c) => !hidden.has(c.calendarId)) ??
    w[0]
  );
}
