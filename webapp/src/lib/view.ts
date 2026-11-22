/** The calendar views. The Zest calendar-view draws each one. */
import type { CalendarViewName } from "@catalystcommunity/calendar-view";

export type ViewKind = CalendarViewName;
export const VIEWS: ViewKind[] = ["month", "week", "day", "agenda"];
