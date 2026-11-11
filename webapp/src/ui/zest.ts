/**
 * Define the Zest tags that catalystcal uses. Importing a Zest package
 * defines nothing, so main.ts calls this before the first element.
 */
import { defineControls } from "@catalystcommunity/controls";
import { defineContainers } from "@catalystcommunity/containers";
import { defineMenus } from "@catalystcommunity/menus";
import { CalendarView } from "@catalystcommunity/calendar-view";
import { DateTimePicker } from "@catalystcommunity/date-time-picker";
import { define } from "~/ui/element";

export function defineZest(): void {
  defineControls();
  defineContainers();
  defineMenus();
  define("calendar-view", CalendarView);
  define("date-time-picker", DateTimePicker);
}
