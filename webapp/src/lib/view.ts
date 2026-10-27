/** Visible ranges for each calendar view. */
import { DateText, Weekday, addDays, addMonths, monthGrid, rangeDays, startOfWeek, parseDate } from "~/lib/dates";
import { formatDate, formatMonthYear, startOfDayUtc } from "~/lib/tz";

export type ViewKind = "month" | "week" | "day" | "agenda";
export const VIEWS: ViewKind[] = ["month", "week", "day", "agenda"];
export const AGENDA_DAYS = 30;

export interface VisibleRange {
  days: DateText[];
  /** For month view: the week rows. */
  weeks?: DateText[][];
  startMs: number;
  endMs: number;
}

/**
 * @param weekDays days the week view shows: 7, or 3 on a narrow screen
 *   (then the range starts at the cursor, not at the week start).
 */
export function visibleRange(view: ViewKind, cursor: DateText, tz: string, weekStart: Weekday, weekDays = 7): VisibleRange {
  let days: DateText[];
  let weeks: DateText[][] | undefined;
  switch (view) {
    case "month":
      weeks = monthGrid(cursor, weekStart);
      days = weeks.flat();
      break;
    case "week":
      days = weekDays === 7 ? rangeDays(startOfWeek(cursor, weekStart), 7) : rangeDays(cursor, weekDays);
      break;
    case "day":
      days = [cursor];
      break;
    case "agenda":
      days = rangeDays(cursor, AGENDA_DAYS);
      break;
  }
  return {
    days,
    weeks,
    startMs: startOfDayUtc(days[0], tz),
    endMs: startOfDayUtc(addDays(days[days.length - 1], 1), tz),
  };
}

/** Move the cursor one page forward (dir 1) or back (dir -1). */
export function stepCursor(view: ViewKind, cursor: DateText, dir: number, weekDays = 7): DateText {
  switch (view) {
    case "month":
      return addMonths(cursor, dir);
    case "week":
      return addDays(cursor, dir * weekDays);
    case "day":
      return addDays(cursor, dir);
    case "agenda":
      return addDays(cursor, dir * AGENDA_DAYS);
  }
}

/** The heading for a view: "October 2026", "Oct 12 – 18, 2026", ... */
export function viewTitle(view: ViewKind, range: VisibleRange, cursor: DateText): string {
  if (view === "month") return formatMonthYear(cursor);
  if (view === "day") return formatDate(cursor, { weekday: "long", month: "long", day: "numeric", year: "numeric" });
  const a = range.days[0];
  const b = range.days[range.days.length - 1];
  const pa = parseDate(a);
  const pb = parseDate(b);
  if (pa.y !== pb.y) return `${formatDate(a, { month: "short", day: "numeric", year: "numeric" })} – ${formatDate(b, { month: "short", day: "numeric", year: "numeric" })}`;
  if (pa.m !== pb.m) return `${formatDate(a, { month: "short", day: "numeric" })} – ${formatDate(b, { month: "short", day: "numeric" })}, ${pb.y}`;
  return `${formatDate(a, { month: "long" })} ${pa.d} – ${pb.d}, ${pa.y}`;
}
