/** Event search: a combobox over EventService.search-events. */
import { For, Show, createSignal, onCleanup } from "solid-js";
import { useNavigate } from "@solidjs/router";
import type { Event, Occurrence } from "@catalystcal/client";
import { Icon } from "~/components/Icons";
import { eventClient } from "~/data/clients";
import { calendarById, calendarColor } from "~/stores/calendars";
import { displayZone } from "~/stores/prefs";
import { goToDate } from "~/stores/nav";
import { openDetail } from "~/stores/editor";
import { civilDate, formatDate, formatTime, parseTimestamp } from "~/lib/tz";
import { errorMessage } from "~/transport/csilrpc";

export function eventDate(e: Event, tz: string): string | null {
  if (e.allDay && e.startDate) return e.startDate;
  if (e.start) return civilDate(parseTimestamp(e.start), tz);
  return null;
}

/** A detail-sheet occurrence for an event that came from search or invitations. */
export function occurrenceOf(e: Event, invited = false): Occurrence {
  const cal = calendarById(e.calendarId);
  const start = e.start ?? (e.startDate ? `${e.startDate}T00:00:00Z` : new Date().toISOString());
  const end = e.end ?? (e.endDate ? `${e.endDate}T00:00:00Z` : start);
  return { event: e, start, end, access: cal?.access ?? (invited ? "read" : "read"), busyOnly: false, invited: invited || !cal };
}

export function SearchBox() {
  const navigate = useNavigate();
  const [q, setQ] = createSignal("");
  const [results, setResults] = createSignal<Event[]>([]);
  const [open, setOpen] = createSignal(false);
  const [active, setActive] = createSignal(-1);
  const [error, setError] = createSignal<string | null>(null);
  const [loading, setLoading] = createSignal(false);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  let wrap!: HTMLDivElement;

  const run = (text: string) => {
    clearTimeout(timer);
    if (!text.trim()) {
      setResults([]);
      setError(null);
      return;
    }
    timer = setTimeout(async () => {
      const my = ++seq;
      setLoading(true);
      try {
        const r = await eventClient.searchEvents({ query: text.trim().slice(0, 256), limit: 20 });
        if (my !== seq) return;
        setResults(r.events ?? []);
        setError(null);
        setActive(r.events?.length ? 0 : -1);
      } catch (e) {
        if (my === seq) setError(errorMessage(e));
      } finally {
        if (my === seq) setLoading(false);
      }
    }, 220);
  };
  onCleanup(() => clearTimeout(timer));

  const choose = (e: Event) => {
    const tz = displayZone();
    const d = eventDate(e, tz);
    if (d) goToDate(d, "day");
    setOpen(false);
    setQ("");
    setResults([]);
    navigate("/");
    openDetail(occurrenceOf(e));
  };

  const onKey = (ev: KeyboardEvent) => {
    const list = results();
    if (ev.key === "ArrowDown") {
      ev.preventDefault();
      setOpen(true);
      setActive((a) => Math.min(list.length - 1, a + 1));
    } else if (ev.key === "ArrowUp") {
      ev.preventDefault();
      setActive((a) => Math.max(0, a - 1));
    } else if (ev.key === "Enter") {
      const e = list[active()];
      if (e) {
        ev.preventDefault();
        choose(e);
      }
    } else if (ev.key === "Escape") {
      setOpen(false);
      (ev.target as HTMLInputElement).blur();
    }
  };

  const when = (e: Event) => {
    const tz = displayZone();
    const d = eventDate(e, tz);
    if (!d) return "";
    const day = formatDate(d, { weekday: "short", month: "short", day: "numeric", year: "numeric" });
    return e.allDay || !e.start ? day : `${day}, ${formatTime(parseTimestamp(e.start), tz)}`;
  };

  return (
    <div class="search" ref={wrap} onFocusOut={(e) => !wrap.contains(e.relatedTarget as Node) && setOpen(false)}>
      <Icon name="search" class="search-icon" size={16} />
      <input
        class="search-input"
        type="search"
        placeholder="Search events"
        aria-label="Search events"
        role="combobox"
        aria-expanded={open() && q().trim() !== ""}
        aria-controls="search-results"
        aria-autocomplete="list"
        aria-activedescendant={active() >= 0 ? `search-opt-${active()}` : undefined}
        value={q()}
        onInput={(e) => {
          setQ(e.currentTarget.value);
          setOpen(true);
          run(e.currentTarget.value);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={onKey}
      />
      <Show when={open() && q().trim() !== ""}>
        <div class="search-pop">
          <Show when={error()}>
            <p class="alert" role="alert">
              {error()}
            </p>
          </Show>
          <ul id="search-results" role="listbox" class="search-list" aria-label="Search results">
            <For each={results()}>
              {(e, i) => (
                <li
                  id={`search-opt-${i()}`}
                  role="option"
                  aria-selected={active() === i()}
                  class="search-opt"
                  onMouseDown={(ev) => {
                    ev.preventDefault();
                    choose(e);
                  }}
                  onMouseEnter={() => setActive(i())}
                >
                  <span class="swatch" style={{ "--sw": e.color || calendarColor(calendarById(e.calendarId)) }} />
                  <span class="grow">
                    <span class="truncate" style={{ display: "block" }}>
                      {e.title}
                    </span>
                    <span class="small muted">{when(e)}</span>
                  </span>
                </li>
              )}
            </For>
          </ul>
          <Show when={!loading() && !error() && results().length === 0}>
            <p class="empty small">No events match “{q().trim()}”.</p>
          </Show>
        </div>
      </Show>
    </div>
  );
}
