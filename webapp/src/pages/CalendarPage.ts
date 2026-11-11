/**
 * The calendar: a Zest calendar-view with the caller's occurrences. The view
 * sends a range; this page loads the occurrences for it, and turns the
 * user's selections and moves into the editor, the detail, or an update.
 */
import type { Occurrence, OccurrenceList, RecurrenceScope } from "@catalystcal/client";
import type { CalendarEventMoveDetail, CalendarItem, CalendarRange, CalendarSlotSelectDetail, default as CalendarView } from "@catalystcommunity/calendar-view";
import { eventClient } from "~/data/clients";
import { AUTH, effectivePerson, session } from "~/stores/auth";
import { CALENDARS, bumpEvents, calendars, canEditEvent, defaultWritableCalendar, eventsVersion, hiddenCalendars } from "~/stores/calendars";
import { PREFS, displayZone, weekStart } from "~/stores/prefs";
import { NAV, cursor, setCursor, setView, view } from "~/stores/nav";
import { openDetail, openEditor } from "~/stores/editor";
import { askScope } from "~/stores/dialogs";
import { toast } from "~/stores/toast";
import { VIEWS, type ViewKind } from "~/lib/view";
import { allDayLength, allDayStart, occurrenceKey } from "~/lib/occurrence";
import { buildUpdate, formFromOccurrence, isSeriesOccurrence, newEventForm, type NewEventSeed } from "~/lib/eventForm";
import { myStatus, occColor, occTitle } from "~/lib/eventVisual";
import { addDays } from "~/lib/dates";
import { civilDate, minutesIntoDay, parseTimestamp, roundUpToSlot, todayIn } from "~/lib/tz";
import { matchesMedia, shortcutBlocked, watchMedia } from "~/lib/media";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { h, setAttr } from "~/ui/dom";
import { icon } from "~/ui/icons";

/** A new event seed without the calendar and zone, which startCreate adds. */
export type CreateSeed = Omit<NewEventSeed, "calendarId" | "timezone">;

/** Start a new event in the default writable calendar. */
export function startCreate(seed: CreateSeed): void {
  const cal = defaultWritableCalendar(effectivePerson()?.userId);
  if (!cal) {
    toast("You have no calendar you can add events to.", "error");
    return;
  }
  openEditor({ mode: "create", form: newEventForm({ ...seed, calendarId: cal.calendarId, timezone: displayZone() }) });
}

/**
 * The state tags of an item. calendar-view styles invited, tentative,
 * declined, and cancelled, and reads each tag to a screen reader. catalystcal
 * adds delegated (see TAG_LABELS and calendar.css).
 */
export function itemTags(o: Occurrence): string[] {
  if (o.busyOnly) return [];
  if (o.event.status === "cancelled") return ["cancelled"];
  if (o.invited) {
    const st = myStatus(o);
    if (!st || st === "needs_action") return ["invited"];
    if (st !== "accepted") return [st];
  }
  if (o.event.status === "tentative") return ["tentative"];
  return [];
}

/** Screen reader text of the tags that calendar-view does not know. */
const TAG_LABELS = { delegated: "Delegated" };

export function toItem(o: Occurrence, tz: string): CalendarItem {
  const base = {
    id: occurrenceKey(o),
    title: occTitle(o),
    tags: itemTags(o),
    color: occColor(o),
    busyOnly: o.busyOnly,
    editable: !o.busyOnly && !o.invited && canEditEvent(o.access, o.event.createdBy?.userId, effectivePerson()?.userId),
    seriesId: o.event.masterEventId ?? (o.event.recurrence ? o.event.eventId : undefined),
    calendarId: o.event.calendarId,
  };
  if (o.event.allDay) {
    const start = allDayStart(o, tz);
    return { ...base, allDay: true, start, end: addDays(start, allDayLength(o)) };
  }
  return { ...base, allDay: false, start: o.start, end: o.end };
}

/** A calendar-view selection (dates, or instants) as an editor seed in `tz`. */
export function seedFromSlot(d: Pick<CalendarSlotSelectDetail, "start" | "end" | "allDay">, tz: string): CreateSeed {
  if (d.allDay) {
    const end = d.end && d.end > d.start ? addDays(d.end.slice(0, 10), -1) : d.start.slice(0, 10);
    return { date: d.start.slice(0, 10), endDate: end, allDay: true };
  }
  const s = parseTimestamp(d.start);
  const e = parseTimestamp(d.end);
  return { date: civilDate(s, tz), startMin: minutesIntoDay(s, tz), endDate: civilDate(e, tz), endMin: minutesIntoDay(e, tz) };
}

const NARROW = "(max-width: 760px)";
const TINY = "(max-width: 480px)";

export class CalendarPage extends CalElement {
  static watches = [AUTH, CALENDARS, PREFS, NAV];
  #cal!: CalendarView;
  #spinner!: HTMLElement;
  #error!: HTMLElement;
  #truncated!: HTMLElement;
  #allHidden!: HTMLElement;
  #occurrences: Occurrence[] = [];
  #byId = new Map<string, Occurrence>();
  #range: CalendarRange | null = null;
  #loadKey = "";
  #seq = 0;

  protected build(): void {
    this.#cal = h("calendar-view", { views: VIEWS.join(","), class: "cal-view" }) as CalendarView;
    this.#cal.updateConfig({ labels: { tags: TAG_LABELS } });
    this.#spinner = h("span", { class: "spinner spinner-sm", role: "status", "aria-label": "Loading events", hidden: true });
    this.#error = h("p", { class: "alert cal-alert", role: "alert", hidden: true });
    this.#truncated = h(
      "p",
      { class: "notice cal-alert", role: "status", hidden: true },
      "Some repeating events are not shown because the range has too many. Choose a shorter view to see them all.",
    );
    this.#allHidden = h("p", { class: "notice cal-alert", hidden: true }, "All your calendars are hidden. Turn one on in the sidebar.");
    this.append(
      h(
        "div",
        { class: "cal-page" },
        h(
          "div",
          { class: "cal-toolbar" },
          h("h1", { class: "sr-only" }, "Calendar"),
          this.#spinner,
          h(
            "push-button",
            { variant: "primary", class: "cal-new", title: "New event (C)", onClick: () => this.#createNow() },
            icon("plus"),
            h("span", null, "New event"),
          ),
        ),
        this.#error,
        this.#truncated,
        this.#allHidden,
        h("div", { class: "cal-surface" }, this.#cal),
      ),
    );
    this.#cal.addEventListener("calendar-range-changed", (e) => this.#onRange(e.detail));
    this.#cal.addEventListener("calendar-event-select", (e) => {
      const o = this.#byId.get(e.detail.id);
      if (o) openDetail(o);
    });
    this.#cal.addEventListener("calendar-slot-select", (e) => startCreate(seedFromSlot(e.detail, displayZone())));
    this.#cal.addEventListener("calendar-event-move", (e) => void this.#move(e.detail));
  }

  protected connected(): void {
    this.listen(document, "keydown", (e) => this.#onKey(e as KeyboardEvent));
    this.own(watchMedia(NARROW, () => this.invalidate()));
    this.own(watchMedia(TINY, () => this.invalidate()));
  }

  /** The view and day count the calendar shows: a week is 3 days or 1 day on a narrow screen. */
  #effectiveView(): { view: ViewKind; days?: number } {
    const v = view();
    if (v !== "week") return { view: v };
    if (matchesMedia(TINY)) return { view: "day", days: 1 };
    if (matchesMedia(NARROW)) return { view: "day", days: 3 };
    return { view: "week" };
  }

  render(): void {
    const eff = this.#effectiveView();
    setAttr(this.#cal, "days", eff.days);
    setAttr(this.#cal, "time-zone", displayZone());
    // A set of the view or the date that shows does nothing in calendar-view.
    this.#cal.view = eff.view;
    this.#cal.date = cursor();
    setAttr(this.#cal, "first-day-of-week", weekStart());
    const cals = calendars();
    this.#allHidden.hidden = !(cals.length > 0 && hiddenCalendars().size >= cals.length);
    this.#maybeLoad();
    this.#showItems();
  }

  #onRange(r: CalendarRange): void {
    this.#range = r;
    // The user can change the view and the date in the calendar's own toolbar.
    if (r.date && r.date !== cursor()) setCursor(r.date);
    const eff = this.#effectiveView();
    if (r.view !== eff.view && VIEWS.includes(r.view as ViewKind)) setView(r.view as ViewKind);
    this.#maybeLoad();
  }

  /** Load when the range, the event version, or the effective person changes. */
  #maybeLoad(): void {
    const r = this.#range;
    if (!r || !session()) return;
    const key = `${r.start}|${r.end}|${eventsVersion()}|${effectivePerson()?.userId ?? ""}`;
    if (key === this.#loadKey) return;
    this.#loadKey = key;
    void this.#load(r);
  }

  async #load(r: CalendarRange): Promise<void> {
    const seq = ++this.#seq;
    this.#spinner.hidden = false;
    let res: OccurrenceList | null = null;
    let problem: string | null = null;
    try {
      res = await eventClient.listEvents({ rangeStart: r.start, rangeEnd: r.end, includeInvitations: true });
    } catch (e) {
      problem = errorMessage(e);
    }
    if (seq !== this.#seq || !this.isActive()) return;
    this.#spinner.hidden = true;
    this.#error.hidden = problem === null;
    this.#error.textContent = problem ? `Events did not load: ${problem}` : "";
    this.#truncated.hidden = !res?.truncated;
    this.#occurrences = res?.occurrences ?? [];
    this.#showItems();
  }

  #showItems(): void {
    const hidden = hiddenCalendars();
    const tz = displayZone();
    const shown = this.#occurrences.filter((o) => !hidden.has(o.event.calendarId));
    this.#byId = new Map(shown.map((o) => [occurrenceKey(o), o]));
    this.#cal.events = shown.map((o) => toItem(o, tz));
  }

  async #move(d: CalendarEventMoveDetail): Promise<void> {
    const occ = this.#byId.get(d.id);
    if (!occ) return;
    const tz = displayZone();
    const form = formFromOccurrence(occ, tz);
    if (d.allDay) {
      Object.assign(form, seedToForm(seedFromSlot(d, tz)), { allDay: true });
    } else {
      // The form keeps the event's own zone.
      Object.assign(form, seedToForm(seedFromSlot(d, form.timezone)), { allDay: false });
    }
    let scope: RecurrenceScope = "all";
    if (isSeriesOccurrence(occ)) {
      const s = await askScope("save");
      if (!s) return this.#showItems(); // The item goes back.
      scope = s;
    }
    try {
      await eventClient.updateEvent(buildUpdate(form, occ, scope));
      toast("Event moved.");
      bumpEvents();
    } catch (e) {
      toast(errorMessage(e), "error");
      if (this.isActive()) this.#showItems();
    }
  }

  // New event: the next half hour if today is on screen, else 09:00 on the cursor date.
  #createNow(): void {
    const tz = displayZone();
    const today = todayIn(tz);
    const r = this.#range;
    if (r && r.startDate <= today && today < r.endDate) {
      const slot = roundUpToSlot(Date.now(), tz);
      startCreate({ date: slot.date, startMin: slot.minutes, endMin: slot.minutes + 60 });
    } else {
      startCreate({ date: cursor(), startMin: 9 * 60, endMin: 10 * 60 });
    }
  }

  #onKey(e: KeyboardEvent): void {
    if (shortcutBlocked(e)) return;
    const inGrid = !!(e.target as Element | null)?.closest?.("calendar-view");
    const keys: Record<string, () => void> = {
      t: () => this.#cal.today(),
      k: () => this.#cal.previous(),
      p: () => this.#cal.previous(),
      j: () => this.#cal.next(),
      n: () => this.#cal.next(),
      m: () => setView("month"),
      w: () => setView("week"),
      d: () => setView("day"),
      a: () => setView("agenda"),
      c: () => this.#createNow(),
    };
    // The arrow keys move in the grid; outside it they change the range.
    if (!inGrid) {
      keys.arrowleft = keys.k;
      keys.arrowright = keys.j;
    }
    const run = keys[e.key.toLowerCase()];
    if (!run) return;
    e.preventDefault();
    run();
  }
}
define("cal-calendar-page", CalendarPage);

/** The form fields of a seed. */
function seedToForm(seed: CreateSeed): { startDate: string; endDate: string; startMin: number; endMin: number } {
  return {
    startDate: seed.date,
    endDate: seed.endDate ?? seed.date,
    startMin: seed.startMin ?? 0,
    endMin: seed.endMin ?? 0,
  };
}
