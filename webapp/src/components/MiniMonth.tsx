import { For, createMemo, createSignal, createEffect } from "solid-js";
import { Icon } from "~/components/Icons";
import { DateText, addMonths, monthGrid, sameMonth, startOfMonth } from "~/lib/dates";
import { formatDate, formatMonthYear, todayIn, weekdayShort } from "~/lib/tz";
import { useWeekStart, displayZone } from "~/stores/prefs";

/** Small month navigator. Picking a day calls onPick. */
export function MiniMonth(props: { selected: DateText; rangeDays?: DateText[]; onPick: (d: DateText) => void }) {
  const weekStart = useWeekStart();
  const [month, setMonth] = createSignal(startOfMonth(props.selected));
  createEffect(() => setMonth(startOfMonth(props.selected)));
  const weeks = createMemo(() => monthGrid(month(), weekStart()));
  const today = () => todayIn(displayZone());
  const inRange = (d: DateText) => props.rangeDays?.includes(d) ?? false;

  return (
    <div class="mini-month">
      <div class="mini-head">
        <span class="mini-title">{formatMonthYear(month())}</span>
        <button type="button" class="icon-btn icon-btn-sm" aria-label="Previous month" onClick={() => setMonth(addMonths(month(), -1))}>
          <Icon name="chevronLeft" size={16} />
        </button>
        <button type="button" class="icon-btn icon-btn-sm" aria-label="Next month" onClick={() => setMonth(addMonths(month(), 1))}>
          <Icon name="chevronRight" size={16} />
        </button>
      </div>
      <table class="mini-grid" role="grid" aria-label={formatMonthYear(month())}>
        <thead>
          <tr>
            <For each={weeks()[0]}>
              {(d) => (
                <th scope="col" abbr={weekdayShort(d)}>
                  {weekdayShort(d).slice(0, 2)}
                </th>
              )}
            </For>
          </tr>
        </thead>
        <tbody>
          <For each={weeks()}>
            {(w) => (
              <tr>
                <For each={w}>
                  {(d) => (
                    <td>
                      <button
                        type="button"
                        class="mini-day tnum"
                        classList={{
                          "is-other": !sameMonth(d, month()),
                          "is-today": d === today(),
                          "is-selected": d === props.selected,
                          "in-range": inRange(d),
                        }}
                        aria-label={formatDate(d)}
                        aria-current={d === today() ? "date" : undefined}
                        onClick={() => props.onPick(d)}
                      >
                        {Number(d.slice(8))}
                      </button>
                    </td>
                  )}
                </For>
              </tr>
            )}
          </For>
        </tbody>
      </table>
    </div>
  );
}
