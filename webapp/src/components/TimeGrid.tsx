/**
 * Week and day views: an all-day row of lane bars over a 24-hour grid.
 * Timed events that overlap sit side by side (lib/layout). Click an empty
 * slot to create a 1-hour event there; drag with a mouse or pen to create a
 * range. Touch taps create; touch drags scroll.
 */
import { For, Index, Show, createMemo, createSignal, onMount } from "solid-js";
import type { Occurrence } from "@catalystcal/client";
import { DateText, addDays, daysBetween, dayOfWeek } from "~/lib/dates";
import { Placed, covers } from "~/lib/occurrence";
import { assignLanes, layoutTimed } from "~/lib/layout";
import { zoneAbbrev, formatDate, formatHourLabel, formatTime, minutesIntoDay, minutesToHHMM, startOfDayUtc, todayIn } from "~/lib/tz";
import { occClasses, occColor, occTitle } from "~/lib/eventVisual";
import { useNow } from "~/stores/clock";

export interface CreateSeed {
  date: DateText;
  startMin?: number;
  endDate?: DateText;
  endMin?: number;
  allDay?: boolean;
}

interface Props {
  days: DateText[];
  placed: Placed[];
  tz: string;
  onCreate: (seed: CreateSeed) => void;
  onOpen: (o: Occurrence) => void;
  onPickDay: (d: DateText) => void;
}

const SNAP = 15;
const MAX_LANES = 3;

export function TimeGrid(props: Props) {
  const now = useNow();
  const today = () => todayIn(props.tz, now());
  let scroller!: HTMLDivElement;

  // ---- all-day row ----
  const bars = createMemo(() => {
    const first = props.days[0];
    const last = props.days[props.days.length - 1];
    const list = props.placed.filter((p) => p.bar && p.startDate <= last && p.lastDate >= first);
    const segs = list.map((p) => ({
      id: p.key,
      p,
      startIdx: Math.max(0, daysBetween(first, p.startDate)),
      endIdx: Math.min(props.days.length - 1, daysBetween(first, p.lastDate)),
    }));
    const lanes = assignLanes(segs);
    return segs.map((s) => ({ ...s, lane: lanes.get(s.id) ?? 0 }));
  });
  const laneCount = createMemo(() => bars().reduce((m, b) => Math.max(m, b.lane + 1), 0));
  const [expanded, setExpanded] = createSignal(false);
  const shownLanes = () => (expanded() ? laneCount() : Math.min(laneCount(), MAX_LANES));
  const hiddenOn = (i: number) => bars().filter((b) => b.lane >= shownLanes() && b.startIdx <= i && i <= b.endIdx).length;

  // ---- timed columns ----
  const columns = createMemo(() =>
    props.days.map((d) => {
      const dayStart = startOfDayUtc(d, props.tz);
      const dayEnd = startOfDayUtc(addDays(d, 1), props.tz);
      const items = props.placed
        .filter((p) => !p.bar && covers(p, d))
        .map((p) => {
          const s = Math.max(p.startMs, dayStart);
          const e = Math.min(p.endMs, dayEnd);
          const top = s <= dayStart ? 0 : minutesIntoDay(s, props.tz);
          let bottom = e >= dayEnd ? 1440 : minutesIntoDay(e, props.tz);
          if (bottom <= top) bottom = Math.min(1440, top + SNAP);
          return { p, top, bottom, contStart: p.startMs < dayStart, contEnd: p.endMs > dayEnd };
        });
      const boxes = layoutTimed(
        items.map((it) => ({ id: it.p.key, start: it.top, end: it.bottom })),
        22,
      );
      return { date: d, items: items.map((it) => ({ ...it, box: boxes.get(it.p.key)! })) };
    }),
  );

  // ---- drag to create ----
  // `a` is the snapped minute under the press; `c` the raw minute now.
  const [drag, setDrag] = createSignal<{ date: DateText; a: number; c: number; moved: boolean; pointer: string } | null>(null);
  const minuteAt = (el: HTMLElement, y: number) => {
    const r = el.getBoundingClientRect();
    return Math.max(0, Math.min(1440, ((y - r.top) / (r.height || 1)) * 1440));
  };
  const rangeOf = (d: { a: number; c: number }) =>
    d.c >= d.a
      ? { lo: d.a, hi: Math.min(1440, Math.max(d.a + SNAP, Math.ceil(d.c / SNAP) * SNAP)) }
      : { lo: Math.floor(d.c / SNAP) * SNAP, hi: Math.min(1440, d.a + SNAP) };
  const onDown = (e: PointerEvent, date: DateText) => {
    if (e.button !== 0 || e.target !== e.currentTarget) return;
    const el = e.currentTarget as HTMLElement;
    const raw = minuteAt(el, e.clientY);
    setDrag({ date, a: Math.min(1440 - SNAP, Math.floor(raw / SNAP) * SNAP), c: raw, moved: false, pointer: e.pointerType });
    if (e.pointerType !== "touch") {
      try {
        el.setPointerCapture(e.pointerId);
      } catch {
        // Not supported (tests); the drag still ends on pointerup.
      }
      e.preventDefault();
    }
  };
  const onMove = (e: PointerEvent) => {
    const d = drag();
    if (!d || d.pointer === "touch") return;
    const c = minuteAt(e.currentTarget as HTMLElement, e.clientY);
    setDrag({ ...d, c, moved: d.moved || Math.abs(c - d.a) > SNAP });
  };
  const onUp = () => {
    const d = drag();
    setDrag(null);
    if (!d) return;
    if (d.moved) {
      const { lo, hi } = rangeOf(d);
      props.onCreate({ date: d.date, startMin: lo, endMin: hi });
    } else {
      const start = Math.min(1410, Math.floor(d.a / 30) * 30);
      props.onCreate({ date: d.date, startMin: start, endMin: start + 60 });
    }
  };
  const ghost = () => {
    const d = drag();
    if (!d || !d.moved) return null;
    return { date: d.date, ...rangeOf(d) };
  };

  onMount(() => {
    const hourPx = scroller.scrollHeight / 24 || 52;
    const showsToday = props.days.includes(today());
    const earliest = columns()
      .flatMap((c) => c.items.map((i) => i.top))
      .reduce((m, t) => Math.min(m, t), 24 * 60);
    const target = showsToday ? Math.max(0, minutesIntoDay(now(), props.tz) - 120) : Math.min(7 * 60, earliest);
    scroller.scrollTop = (target / 60) * hourPx;
  });

  const nowTop = () => (minutesIntoDay(now(), props.tz) / 1440) * 100;

  return (
    <div class="tg" style={{ "--days": props.days.length }}>
      <div class="tg-head" role="row">
        <div class="tg-gutter-head tnum" aria-hidden="true">
          {zoneAbbrev(now(), props.tz)}
        </div>
        <Index each={props.days}>
          {(d) => (
            <button
              type="button"
              class="tg-dayhead"
              classList={{ "is-today": d() === today(), "is-weekend": dayOfWeek(d()) === 0 || dayOfWeek(d()) === 6 }}
              aria-label={`${formatDate(d())}. Show this day`}
              aria-current={d() === today() ? "date" : undefined}
              onClick={() => props.onPickDay(d())}
            >
              <span class="tg-dow">{formatDate(d(), { weekday: "short" })}</span>
              <span class="tg-dnum tnum">{Number(d().slice(8))}</span>
            </button>
          )}
        </Index>
      </div>

      <div class="tg-allday" style={{ "--lanes": Math.max(1, shownLanes()) + (shownLanes() < laneCount() ? 1 : 0) }}>
        <div class="tg-allday-label">
          <Show when={laneCount() > MAX_LANES} fallback={<span>All day</span>}>
            <button type="button" class="link-btn small" aria-expanded={expanded()} onClick={() => setExpanded(!expanded())}>
              {expanded() ? "Fewer" : "All day"}
            </button>
          </Show>
        </div>
        <div class="tg-allday-grid">
          <Index each={props.days}>
            {(d, i) => (
              <div
                class="tg-allday-cell"
                classList={{ "is-today": d() === today() }}
                style={{ "grid-column": `${i + 1}` }}
                onClick={(e) => e.target === e.currentTarget && props.onCreate({ date: d(), allDay: true })}
              >
                <Show when={hiddenOn(i) > 0}>
                  <button type="button" class="more-btn" style={{ "grid-row": `${shownLanes() + 1}` }} onClick={() => setExpanded(true)}>
                    +{hiddenOn(i)} more
                  </button>
                </Show>
              </div>
            )}
          </Index>
          <For each={bars().filter((b) => b.lane < shownLanes())}>
            {(b) => (
              <button
                type="button"
                class="ev-bar"
                classList={{ ...occClasses(b.p.occ), "cont-start": b.p.startDate < props.days[0], "cont-end": b.p.lastDate > props.days[props.days.length - 1] }}
                style={{ "--ev": occColor(b.p.occ), "grid-column": `${b.startIdx + 1} / ${b.endIdx + 2}`, "grid-row": `${b.lane + 1}` }}
                onClick={() => props.onOpen(b.p.occ)}
                aria-label={`${occTitle(b.p.occ)}, ${b.p.allDay ? "all day" : formatTime(b.p.startMs, props.tz)}, ${formatDate(b.p.startDate, { month: "short", day: "numeric" })}${b.p.lastDate !== b.p.startDate ? ` to ${formatDate(b.p.lastDate, { month: "short", day: "numeric" })}` : ""}`}
              >
                <Show when={!b.p.allDay}>
                  <span class="ev-time tnum">{formatTime(b.p.startMs, props.tz)}</span>
                </Show>
                <span class="ev-title truncate">{occTitle(b.p.occ)}</span>
              </button>
            )}
          </For>
        </div>
      </div>

      <div class="tg-scroll" ref={scroller}>
        <div class="tg-body">
          <div class="tg-gutter" aria-hidden="true">
            <For each={Array.from({ length: 24 }, (_, h) => h)}>
              {(h) => (
                <div class="tg-hour tnum">
                  <span>{h === 0 ? "" : formatHourLabel(h)}</span>
                </div>
              )}
            </For>
            <Show when={props.days.includes(today())}>
              <div class="tg-now-label tnum" style={{ top: `${nowTop()}%` }}>
                {formatTime(now(), props.tz)}
              </div>
            </Show>
          </div>
          <div class="tg-cols">
            <For each={columns()}>
              {(col) => (
                <div
                  class="tg-col"
                  classList={{ "is-today": col.date === today(), "is-weekend": dayOfWeek(col.date) === 0 || dayOfWeek(col.date) === 6 }}
                  data-date={col.date}
                  onPointerDown={(e) => onDown(e, col.date)}
                  onPointerMove={onMove}
                  onPointerUp={onUp}
                  onPointerCancel={() => setDrag(null)}
                >
                  <For each={col.items}>
                    {(it) => {
                      const o = it.p.occ;
                      const short = () => it.bottom - it.top < 40;
                      return (
                        <button
                          type="button"
                          class="ev-block"
                          classList={{ ...occClasses(o), "is-short": short(), "cont-start": it.contStart, "cont-end": it.contEnd }}
                          style={{
                            "--ev": occColor(o),
                            top: `${(it.top / 1440) * 100}%`,
                            height: `calc(${((it.bottom - it.top) / 1440) * 100}% - 2px)`,
                            left: `calc(${(it.box.col / it.box.cols) * 100}% + 1px)`,
                            width: `calc(${(it.box.span / it.box.cols) * 100}% - 3px)`,
                          }}
                          onPointerDown={(e) => e.stopPropagation()}
                          onClick={() => props.onOpen(o)}
                          aria-label={`${occTitle(o)}, ${formatTime(it.p.startMs, props.tz)} to ${formatTime(it.p.endMs, props.tz)}${o.invited ? ", invitation" : ""}`}
                        >
                          <span class="ev-title">{occTitle(o)}</span>
                          <span class="ev-meta tnum">
                            {formatTime(it.p.startMs, props.tz)}
                            <Show when={!short()}>
                              {" – "}
                              {formatTime(it.p.endMs, props.tz)}
                            </Show>
                          </span>
                          <Show when={!short() && !o.busyOnly && o.event.location}>
                            <span class="ev-meta truncate">{o.event.location}</span>
                          </Show>
                        </button>
                      );
                    }}
                  </For>
                  <Show when={ghost()?.date === col.date && ghost()}>
                    {(g) => (
                      <div class="ev-ghost" style={{ top: `${(g().lo / 1440) * 100}%`, height: `${((g().hi - g().lo) / 1440) * 100}%` }} aria-hidden="true">
                        <span class="tnum">
                          {minutesToHHMM(g().lo)} – {minutesToHHMM(g().hi % 1440)}
                        </span>
                      </div>
                    )}
                  </Show>
                  <Show when={col.date === today()}>
                    <div class="now-line" style={{ top: `${nowTop()}%` }} aria-hidden="true" />
                  </Show>
                </div>
              )}
            </For>
          </div>
        </div>
      </div>
    </div>
  );
}

