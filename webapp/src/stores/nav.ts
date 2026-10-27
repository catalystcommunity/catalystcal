/** Which view and date the calendar shows. */
import { createSignal } from "solid-js";
import type { DateText } from "~/lib/dates";
import { readStore, writeStore } from "~/lib/storage";
import { VIEWS, ViewKind } from "~/lib/view";
import { browserZone, todayIn } from "~/lib/tz";

const VIEW_KEY = "catalystcal.view";
const storedView = readStore(VIEW_KEY) as ViewKind | null;

const [view, setViewSig] = createSignal<ViewKind>(storedView && VIEWS.includes(storedView) ? storedView : "week");
const [cursor, setCursor] = createSignal<DateText>(todayIn(browserZone()));
const [sidebarOpen, setSidebarOpen] = createSignal(false);

export const useView = () => view;
export const useCursor = () => cursor;
export { setCursor };
export function setView(v: ViewKind): void {
  setViewSig(v);
  writeStore(VIEW_KEY, v);
}
export const useSidebarOpen = () => sidebarOpen;
export { setSidebarOpen };

/** Show a date in a view (search results, mini month, "+N more"). */
export function goToDate(d: DateText, v?: ViewKind): void {
  setCursor(d);
  if (v) setView(v);
}
