import { For, Show, createUniqueId } from "solid-js";
import { DateText, WEEKDAY_CODES, WeekdayCode, dayOfWeek, nthWeekdayOfMonth, parseDate } from "~/lib/dates";
import { RecurrenceForm, RecurrenceFreq, buildRRule, describeRecurrence, ordinalWord, weekdayName, weekdayShortName } from "~/lib/rrule";
import { useWeekStart } from "~/stores/prefs";

const FREQS: { value: RecurrenceFreq; label: string }[] = [
  { value: "none", label: "Does not repeat" },
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "monthly", label: "Monthly" },
  { value: "yearly", label: "Yearly" },
  { value: "custom", label: "Custom rule (RRULE)" },
];
const UNIT: Record<string, string> = { daily: "day", weekly: "week", monthly: "month", yearly: "year" };

export function RecurrenceEditor(props: { value: RecurrenceForm; start: DateText; tz: string; allDay: boolean; onChange: (f: RecurrenceForm) => void }) {
  const id = createUniqueId();
  const weekStart = useWeekStart();
  const f = () => props.value;
  const set = (patch: Partial<RecurrenceForm>) => props.onChange({ ...f(), ...patch });
  const orderedDays = () => [...WEEKDAY_CODES.slice(weekStart()), ...WEEKDAY_CODES.slice(0, weekStart())];
  const startCode = (): WeekdayCode => WEEKDAY_CODES[dayOfWeek(props.start)];
  const nthInfo = () => nthWeekdayOfMonth(props.start);

  const changeFreq = (freq: RecurrenceFreq) => {
    if (freq === "custom" && !f().custom) {
      const current = buildRRule({ ...f(), freq: f().freq === "none" ? "weekly" : f().freq }, { tz: props.tz, allDay: props.allDay });
      set({ freq, custom: current ?? "FREQ=WEEKLY" });
    } else set({ freq });
  };

  const toggleDay = (c: WeekdayCode) => {
    const has = f().byDay.includes(c);
    set({ byDay: has ? f().byDay.filter((d) => d !== c) : [...f().byDay, c] });
  };

  return (
    <fieldset class="recur">
      <legend class="field-label">Repeat</legend>
      <div class="row">
        <label class="sr-only" for={`${id}-freq`}>
          Repeat
        </label>
        <select id={`${id}-freq`} class="select recur-freq" value={f().freq} onChange={(e) => changeFreq(e.currentTarget.value as RecurrenceFreq)}>
          <For each={FREQS}>{(o) => <option value={o.value}>{o.label}</option>}</For>
        </select>
        <Show when={f().freq !== "none" && f().freq !== "custom"}>
          <label class="row-inline small">
            every
            <input
              class="input input-num"
              type="number"
              min="1"
              max="999"
              value={f().interval}
              aria-label="Interval"
              onInput={(e) => set({ interval: Math.max(1, Number(e.currentTarget.value) || 1) })}
            />
            {UNIT[f().freq]}
            {f().interval === 1 ? "" : "s"}
          </label>
        </Show>
      </div>

      <Show when={f().freq === "weekly"}>
        <div class="weekday-toggles" role="group" aria-label="Days of the week">
          <For each={orderedDays()}>
            {(c) => (
              <button type="button" class="weekday" aria-pressed={f().byDay.includes(c)} aria-label={weekdayName(c)} onClick={() => toggleDay(c)}>
                {weekdayShortName(c).slice(0, 2)}
              </button>
            )}
          </For>
        </div>
      </Show>

      <Show when={f().freq === "monthly"}>
        <div class="stack-tight" role="radiogroup" aria-label="Day of the month">
          <label class="check">
            <input type="radio" name={`${id}-mm`} checked={f().monthlyMode === "day"} onChange={() => set({ monthlyMode: "day", monthDay: parseDate(props.start).d })} />
            On day {f().monthlyMode === "day" ? f().monthDay : parseDate(props.start).d}
          </label>
          <label class="check">
            <input type="radio" name={`${id}-mm`} checked={f().monthlyMode === "nth" && f().nth !== -1} onChange={() => set({ monthlyMode: "nth", nth: Math.min(4, nthInfo().nth), nthDay: startCode() })} />
            On the {ordinalWord(Math.min(4, nthInfo().nth))} {weekdayName(startCode())}
          </label>
          <Show when={nthInfo().isLast}>
            <label class="check">
              <input type="radio" name={`${id}-mm`} checked={f().monthlyMode === "nth" && f().nth === -1} onChange={() => set({ monthlyMode: "nth", nth: -1, nthDay: startCode() })} />
              On the last {weekdayName(startCode())}
            </label>
          </Show>
        </div>
      </Show>

      <Show when={f().freq === "custom"}>
        <div class="field">
          <label for={`${id}-custom`} class="sr-only">
            RRULE
          </label>
          <input id={`${id}-custom`} class="input mono" spellcheck={false} value={f().custom} placeholder="FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1" onInput={(e) => set({ custom: e.currentTarget.value })} />
          <span class="hint">An RFC 5545 RRULE, without “RRULE:”. catalystcal keeps it as you write it.</span>
        </div>
      </Show>

      <Show when={f().freq !== "none" && f().freq !== "custom"}>
        <div class="recur-ends" role="radiogroup" aria-label="Ends">
          <span class="small muted">Ends</span>
          <label class="check">
            <input type="radio" name={`${id}-end`} checked={f().ends === "never"} onChange={() => set({ ends: "never" })} />
            Never
          </label>
          <label class="check">
            <input type="radio" name={`${id}-end`} checked={f().ends === "on"} onChange={() => set({ ends: "on" })} />
            On
            <input class="input input-date" type="date" value={f().until} min={props.start} aria-label="End date" disabled={f().ends !== "on"} onInput={(e) => set({ until: e.currentTarget.value })} />
          </label>
          <label class="check">
            <input type="radio" name={`${id}-end`} checked={f().ends === "after"} onChange={() => set({ ends: "after" })} />
            After
            <input class="input input-num" type="number" min="1" max="999" value={f().count} aria-label="Number of times" disabled={f().ends !== "after"} onInput={(e) => set({ count: Math.max(1, Number(e.currentTarget.value) || 1) })} />
            times
          </label>
        </div>
      </Show>

      <Show when={f().freq !== "none"}>
        <p class="hint recur-summary">{describeRecurrence(f(), props.start)}</p>
      </Show>
    </fieldset>
  );
}
