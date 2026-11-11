/** Which view and date the calendar shows, and the narrow-screen drawer. */
import type { DateText } from "~/lib/dates";
import { readStore, writeStore } from "~/lib/storage";
import { VIEWS, ViewKind } from "~/lib/view";
import { browserZone, todayIn } from "~/lib/tz";
import { cell } from "~/state/store";

/** The store collection of this module. */
export const NAV = "nav";

const VIEW_KEY = "catalystcal.view";
const storedView = readStore(VIEW_KEY) as ViewKind | null;

const viewCell = cell<ViewKind>(NAV, "view", storedView && VIEWS.includes(storedView) ? storedView : "week");
const cursorCell = cell<DateText>(NAV, "cursor", todayIn(browserZone()));
const sidebarCell = cell(NAV, "sidebarOpen", false);

export const view = (): ViewKind => viewCell.get();
export const cursor = (): DateText => cursorCell.get();
export const setCursor = (d: DateText) => cursorCell.set(d);
export function setView(v: ViewKind): void {
  viewCell.set(v);
  writeStore(VIEW_KEY, v);
}
export const sidebarOpen = (): boolean => sidebarCell.get();
export const setSidebarOpen = (open: boolean) => sidebarCell.set(open);

/** Show a date in a view (search results, mini month, "+N more"). */
export function goToDate(d: DateText, v?: ViewKind): void {
  setCursor(d);
  if (v) setView(v);
}
