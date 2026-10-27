/**
 * Month view. Multi-day and all-day events are bars in lanes across the
 * week row; one-day timed events are lines in the day cell. A cell that
 * cannot show everything offers "+N more" (opens the day).
 */
import { For, Index, Show, createMemo } from "solid-js";
import type { Occurrence } from "@catalystcal/client";
import { DateText, daysBetween, sameMonth, dayOfWeek } from "~/lib/dates";
import { Placed, covers } from "~/lib/occurrence";
import { assignLanes } from "~/lib/layout";
import { formatDate, formatTime, todayIn } from "~/lib/tz";
import { occClasses, occColor, occTitle } from "~/lib/eventVisual";
import { useNow } from "~/stores/clock";
import type { CreateSeed } from "~/components/TimeGrid";

interface Props {
  weeks: DateText[][];
  cursor: DateText;
  placed: Placed[];
  tz: string;
  onCreate: (seed: CreateSeed) => void;
  onOpen: (o: Occurrence) => void;
  onPickDay: (d: DateText) => void;
}

const LINES = 4;

export function MonthView(props: Props) {
  const now = useNow();
  const today = () => todayIn(props.tz, now());

  const rows = createMemo(() =>
    props.weeks.map((week) => {
      const first = week[0];
      const last = week[6];
      const spanning = props.placed.filter((p) => (p.bar || p.startDate !== p.lastDate) && p.startDate <= last && p.lastDate >= first);
      const segs = spanning.map((p) => ({
        id: p.key,
        p,
        startIdx: Math.max(0, daysBetween(first, p.startDate)),
        endIdx: Math.min(6, daysBetween(first, p.lastDate)),
      }));
      const laneMap = assignLanes(segs);
      const bars = segs.map((s) => ({ ...s, lane: laneMap.get(s.id) ?? 0 }));
      const laneCount = bars.reduce((m, b) => Math.max(m, b.lane + 1), 0);
      const shownLanes = Math.min(laneCount, LINES - 1);
      const cells = week.map((d, i) => {
        const timed = props.placed.filter((p) => !p.bar && p.startDate === p.lastDate && covers(p, d));
        const room = Math.max(0, LINES - shownLanes);
        const hiddenBars = bars.filter((b) => b.lane >= shownLanes && b.startIdx <= i && i <= b.endIdx).length;
        const fit = timed.length + hiddenBars <= room ? timed.length : Math.max(0, room - 1);
        return { date: d, timed: timed.slice(0, fit), more: timed.length - fit + hiddenBars };
      });
      return { week, bars: bars.filter((b) => b.lane < shownLanes), shownLanes, cells };
    }),
  );

  return (
    <div class="month" role="grid" aria-label={formatDate(props.cursor, { month: "long", year: "numeric" })}>
      <div class="month-dows" role="row">
        <Index each={props.weeks[0]}>
          {(d) => (
            <div class="month-dow" role="columnheader" classList={{ "is-weekend": dayOfWeek(d()) === 0 || dayOfWeek(d()) === 6 }}>
              {formatDate(d(), { weekday: "short" })}
            </div>
          )}
        </Index>
      </div>
      <div class="month-rows" style={{ "--weeks": props.weeks.length }}>
        <For each={rows()}>
          {(row) => (
            <div class="month-week" role="row" style={{ "--lanes": row.shownLanes }}>
              <Index each={row.cells}>
                {(cell, i) => (
                  <div
                    class="month-cell"
                    role="gridcell"
                    classList={{
                      "is-other": !sameMonth(cell().date, props.cursor),
                      "is-today": cell().date === today(),
                      "is-weekend": dayOfWeek(cell().date) === 0 || dayOfWeek(cell().date) === 6,
                    }}
                    style={{ "grid-column": `${i + 1}` }}
                    onClick={(e) => e.target === e.currentTarget && props.onCreate({ date: cell().date })}
                  >
                    <button type="button" class="month-dnum tnum" aria-label={`${formatDate(cell().date)}. Show this day`} onClick={() => props.onPickDay(cell().date)}>
                      {cell().date.endsWith("-01") ? formatDate(cell().date, { month: "short", day: "numeric" }) : Number(cell().date.slice(8))}
                    </button>
                    <div class="month-items">
                      <For each={cell().timed}>
                        {(p) => (
                          <button
                            type="button"
                            class="month-ev"
                            classList={occClasses(p.occ)}
                            style={{ "--ev": occColor(p.occ) }}
                            onClick={() => props.onOpen(p.occ)}
                            aria-label={`${occTitle(p.occ)}, ${formatTime(p.startMs, props.tz)}`}
                          >
                            <span class="dot" aria-hidden="true" />
                            <span class="ev-time tnum">{formatTime(p.startMs, props.tz)}</span>
                            <span class="truncate">{occTitle(p.occ)}</span>
                          </button>
                        )}
                      </For>
                      <Show when={cell().more > 0}>
                        <button type="button" class="more-btn" onClick={() => props.onPickDay(cell().date)}>
                          +{cell().more} more
                        </button>
                      </Show>
                    </div>
                  </div>
                )}
              </Index>
              <For each={row.bars}>
                {(b) => (
                  <button
                    type="button"
                    class="ev-bar month-bar"
                    classList={{ ...occClasses(b.p.occ), "cont-start": b.p.startDate < row.week[0], "cont-end": b.p.lastDate > row.week[6] }}
                    style={{ "--ev": occColor(b.p.occ), "grid-column": `${b.startIdx + 1} / ${b.endIdx + 2}`, "--lane": b.lane }}
                    onClick={() => props.onOpen(b.p.occ)}
                    aria-label={`${occTitle(b.p.occ)}, ${formatDate(b.p.startDate, { month: "short", day: "numeric" })}${b.p.lastDate !== b.p.startDate ? ` to ${formatDate(b.p.lastDate, { month: "short", day: "numeric" })}` : ""}`}
                  >
                    <Show when={!b.p.allDay && b.p.startDate >= row.week[0]}>
                      <span class="ev-time tnum">{formatTime(b.p.startMs, props.tz)}</span>
                    </Show>
                    <span class="ev-title truncate">{occTitle(b.p.occ)}</span>
                  </button>
                )}
              </For>
            </div>
          )}
        </For>
      </div>
    </div>
  );
}
