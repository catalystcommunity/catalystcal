/** Viewer preferences: week start and the display time zone. */
import { createSignal } from "solid-js";
import type { Weekday } from "~/lib/dates";
import { readStore, writeStore } from "~/lib/storage";
import { browserZone, isValidZone } from "~/lib/tz";
import { useMe } from "~/stores/auth";

const WEEK_KEY = "catalystcal.weekStart";

function localeWeekStart(): Weekday {
  try {
    const loc = new Intl.Locale(navigator.language) as Intl.Locale & { weekInfo?: { firstDay: number }; getWeekInfo?: () => { firstDay: number } };
    const info = loc.getWeekInfo?.() ?? loc.weekInfo;
    if (info) return (info.firstDay % 7) as Weekday;
  } catch {
    // Older engines have no weekInfo.
  }
  return 0;
}

const stored = readStore(WEEK_KEY);
const [weekStart, setWeekStartSig] = createSignal<Weekday>(stored === "1" ? 1 : stored === "0" ? 0 : localeWeekStart() === 1 ? 1 : 0);

export const useWeekStart = () => weekStart;
export function setWeekStart(w: Weekday): void {
  setWeekStartSig(w);
  writeStore(WEEK_KEY, String(w));
}

/** The zone the calendar draws in: the profile zone, else the browser's. */
export function displayZone(): string {
  const z = useMe()()?.user.timezone;
  return z && isValidZone(z) ? z : browserZone();
}
