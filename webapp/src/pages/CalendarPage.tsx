import { For, Match, Show, Switch, createMemo, createResource, onCleanup } from "solid-js";
import type { Occurrence } from "@catalystcal/client";
import { Icon } from "~/components/Icons";
import { TimeGrid, CreateSeed } from "~/components/TimeGrid";
import { MonthView } from "~/components/MonthView";
import { AgendaView } from "~/components/AgendaView";
import { eventClient } from "~/data/clients";
import { effectivePerson, useSession } from "~/stores/auth";
import { defaultWritableCalendar, useCalendars, useEventsVersion, useHidden } from "~/stores/calendars";
import { displayZone, useWeekStart } from "~/stores/prefs";
import { goToDate, setCursor, setView, useCursor, useView } from "~/stores/nav";
import { openDetail, openEditor } from "~/stores/editor";
import { VIEWS, ViewKind, stepCursor, viewTitle, visibleRange } from "~/lib/view";
import { placeOccurrence, sortPlaced } from "~/lib/occurrence";
import { newEventForm } from "~/lib/eventForm";
import { formatDate, roundUpToSlot, toTimestamp, todayIn } from "~/lib/tz";
import { createMediaQuery, shortcutBlocked } from "~/lib/media";
import { errorMessage } from "~/transport/csilrpc";
import { toast } from "~/stores/toast";

const VIEW_LABEL: Record<ViewKind, string> = { month: "Month", week: "Week", day: "Day", agenda: "Agenda" };
const VIEW_KEY: Record<ViewKind, string> = { month: "M", week: "W", day: "D", agenda: "A" };

/** Start a new event in the default writable calendar. */
export function startCreate(seed: CreateSeed): void {
  const cal = defaultWritableCalendar(effectivePerson()?.userId);
  if (!cal) {
    toast("You have no calendar you can add events to.", "error");
    return;
  }
  openEditor({ mode: "create", form: newEventForm({ ...seed, calendarId: cal.calendarId, timezone: displayZone() }) });
}

export function CalendarPage() {
  const session = useSession();
  const view = useView();
  const cursor = useCursor();
  const weekStart = useWeekStart();
  const hidden = useHidden();
  const cals = useCalendars();
  const version = useEventsVersion();
  const narrow = createMediaQuery("(max-width: 760px)");
  const tiny = createMediaQuery("(max-width: 480px)");

  const weekDays = () => (tiny() ? 1 : narrow() ? 3 : 7);
  const effView = (): ViewKind => (view() === "week" && weekDays() === 1 ? "day" : view());
  const tz = () => displayZone();
  const range = createMemo(() => visibleRange(effView(), cursor(), tz(), weekStart(), weekDays()));
  const title = () =>
    narrow() && effView() === "day" ? formatDate(cursor(), { weekday: "short", month: "short", day: "numeric", year: "numeric" }) : viewTitle(effView(), range(), cursor());

  const [occs] = createResource(
    () => (session() ? { start: range().startMs, end: range().endMs, v: version(), who: effectivePerson()?.userId } : false),
    async (k) => {
      const r = await eventClient.listEvents({ rangeStart: toTimestamp(k.start), rangeEnd: toTimestamp(k.end), includeInvitations: true });
      return r;
    },
  );

  const placed = createMemo(() => {
    const list = occs.error ? [] : (occs.latest?.occurrences ?? []);
    const h = hidden();
    return sortPlaced(list.filter((o) => !h.has(o.event.calendarId)).map((o) => placeOccurrence(o, tz())));
  });

  const step = (dir: number) => setCursor(stepCursor(effView(), cursor(), dir, effView() === "week" ? weekDays() : 7));
  const today = () => setCursor(todayIn(tz()));
  const pickDay = (d: string) => goToDate(d, "day");
  const open = (o: Occurrence) => openDetail(o);

  // New event: the next half hour if today is on screen, else 09:00 on the cursor date.
  const createNow = () => {
    if (range().days.includes(todayIn(tz()))) {
      const slot = roundUpToSlot(Date.now(), tz());
      startCreate({ date: slot.date, startMin: slot.minutes, endMin: slot.minutes + 60 });
    } else {
      startCreate({ date: cursor(), startMin: 9 * 60, endMin: 10 * 60 });
    }
  };

  const onKey = (e: KeyboardEvent) => {
    if (shortcutBlocked(e)) return;
    const k = e.key;
    const handled = (() => {
      switch (k) {
        case "t":
        case "T":
          today();
          return true;
        case "ArrowLeft":
        case "k":
        case "K":
        case "p":
          step(-1);
          return true;
        case "ArrowRight":
        case "j":
        case "J":
        case "n":
          step(1);
          return true;
        case "m":
        case "M":
          setView("month");
          return true;
        case "w":
        case "W":
          setView("week");
          return true;
        case "d":
        case "D":
          setView("day");
          return true;
        case "a":
        case "A":
          setView("agenda");
          return true;
        case "c":
        case "C":
          createNow();
          return true;
      }
      return false;
    })();
    if (handled) e.preventDefault();
  };
  document.addEventListener("keydown", onKey);
  onCleanup(() => document.removeEventListener("keydown", onKey));

  return (
    <div class="cal-page">
      <div class="cal-toolbar">
        <div class="row cal-nav">
          <button type="button" class="btn" onClick={today} title="Today (T)">
            Today
          </button>
          <div class="arrows">
            <button type="button" class="icon-btn" aria-label="Previous" title="Previous (← or K)" onClick={() => step(-1)}>
              <Icon name="chevronLeft" />
            </button>
            <button type="button" class="icon-btn" aria-label="Next" title="Next (→ or J)" onClick={() => step(1)}>
              <Icon name="chevronRight" />
            </button>
          </div>
          <h1 class="cal-title" aria-live="polite">
            {title()}
          </h1>
          <Show when={occs.loading}>
            <span class="spinner spinner-sm" role="status" aria-label="Loading events" />
          </Show>
        </div>
        <div class="row cal-actions">
          <div class="segmented" role="group" aria-label="Calendar view">
            <For each={VIEWS}>
              {(v) => (
                <button type="button" aria-pressed={effView() === v} onClick={() => setView(v)} title={`${VIEW_LABEL[v]} (${VIEW_KEY[v]})`}>
                  {VIEW_LABEL[v]}
                </button>
              )}
            </For>
          </div>
          <button type="button" class="btn btn-primary" onClick={createNow} title="New event (C)" aria-label="New event">
            <Icon name="plus" /> <span class="hide-sm">New event</span>
          </button>
        </div>
      </div>

      <Show when={occs.error}>
        <p class="alert cal-alert" role="alert">
          Events did not load: {errorMessage(occs.error)}
        </p>
      </Show>
      <Show when={!occs.error && occs.latest?.truncated}>
        <p class="notice cal-alert" role="status">
          Some repeating events are not shown because the range has too many. Choose a shorter view to see them all.
        </p>
      </Show>
      <Show when={cals().length > 0 && hidden().size >= cals().length}>
        <p class="notice cal-alert">All your calendars are hidden. Turn one on in the sidebar.</p>
      </Show>

      <div class="cal-surface">
        <Switch>
          <Match when={effView() === "month"}>
            <MonthView weeks={range().weeks!} cursor={cursor()} placed={placed()} tz={tz()} onCreate={startCreate} onOpen={open} onPickDay={pickDay} />
          </Match>
          <Match when={effView() === "agenda"}>
            <AgendaView days={range().days} placed={placed()} tz={tz()} onOpen={open} onPickDay={pickDay} />
          </Match>
          <Match when={effView() === "week" || effView() === "day"}>
            <TimeGrid days={range().days} placed={placed()} tz={tz()} onCreate={startCreate} onOpen={open} onPickDay={pickDay} />
          </Match>
        </Switch>
      </div>
    </div>
  );
}

