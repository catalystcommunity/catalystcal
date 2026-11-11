import { DateText, WEEKDAY_CODES, WeekdayCode, dayOfWeek, nthWeekdayOfMonth, parseDate } from "~/lib/dates";
import { RecurrenceForm, RecurrenceFreq, buildRRule, defaultRecurrenceForm, describeRecurrence, ordinalWord, weekdayName, weekdayShortName } from "~/lib/rrule";
import { PREFS, weekStart } from "~/stores/prefs";
import { CalElement, define } from "~/ui/element";
import { h, setText, syncKeyed } from "~/ui/dom";
import type { OptionControl, PushButton, ValueControl } from "~/ui/controls";

const FREQS: { value: RecurrenceFreq; label: string }[] = [
  { value: "none", label: "Does not repeat" },
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "monthly", label: "Monthly" },
  { value: "yearly", label: "Yearly" },
  { value: "custom", label: "Custom rule (RRULE)" },
];
const UNIT: Record<string, string> = { daily: "day", weekly: "week", monthly: "month", yearly: "year" };

interface NumberControl extends HTMLElement {
  value: number | null;
  disabled: boolean;
}

/**
 * The repeat rule of an event. `value` is a RecurrenceForm. Sends `change`
 * (target: this element) when the user changes the rule.
 */
export class RecurrenceEditor extends CalElement {
  static watches = [PREFS];
  static properties = {
    value: { type: Object, attribute: false as const },
    start: { type: String, default: "" },
    tz: { type: String, default: "" },
    allDay: { type: Boolean },
  };
  declare value: RecurrenceForm | undefined;
  declare start: DateText;
  declare tz: string;
  declare allDay: boolean;

  #freq!: OptionControl;
  #intervalRow!: HTMLElement;
  #interval!: NumberControl;
  #unit!: HTMLElement;
  #weekdays!: HTMLElement;
  #monthly!: OptionControl;
  #custom!: ValueControl;
  #customField!: HTMLElement;
  #ends!: HTMLElement;
  #endsGroup!: OptionControl;
  #until!: ValueControl;
  #count!: NumberControl;
  #summary!: HTMLElement;

  get form(): RecurrenceForm {
    return this.value ?? defaultRecurrenceForm(this.start);
  }

  protected build(): void {
    this.#freq = h("combo-box", { label: "Repeat", class: "recur-freq", ".options": FREQS, onChange: () => this.#changeFreq(this.#freq.value as RecurrenceFreq) }) as OptionControl;
    this.#interval = h("spin-box", {
      label: "Every",
      min: "1",
      max: "999",
      class: "input-num",
      onChange: () => this.#set({ interval: Math.max(1, Math.round(this.#interval.value ?? 1)) }),
    }) as unknown as NumberControl;
    this.#unit = h("span", { class: "small recur-unit" });
    this.#intervalRow = h("div", { class: "hrow recur-interval" }, this.#interval, this.#unit);
    this.#weekdays = h("div", { class: "weekday-toggles", role: "group", "aria-label": "Days of the week" });
    this.#monthly = h("radio-group", { label: "Day of the month", onChange: () => this.#changeMonthly(this.#monthly.value) }) as OptionControl;
    this.#custom = h("text-field", {
      label: "RRULE",
      class: "mono",
      spellcheck: "false",
      placeholder: "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1",
      helper: "An RFC 5545 RRULE, without “RRULE:”. catalystcal keeps it as you write it.",
      onInput: () => this.#set({ custom: this.#custom.value }),
    }) as ValueControl;
    this.#customField = h("div", { class: "field" }, this.#custom);
    this.#endsGroup = h("radio-group", {
      label: "Ends",
      orientation: "horizontal",
      ".options": [
        { value: "never", label: "Never" },
        { value: "on", label: "On a date" },
        { value: "after", label: "After a number of times" },
      ],
      onChange: () => this.#set({ ends: this.#endsGroup.value as RecurrenceForm["ends"] }),
    }) as OptionControl;
    this.#until = h("date-time-picker", { mode: "date", label: "End date", onChange: () => this.#until.value && this.#set({ until: this.#until.value }) }) as ValueControl;
    this.#count = h("spin-box", {
      label: "Number of times",
      min: "1",
      max: "999",
      class: "input-num",
      onChange: () => this.#set({ count: Math.max(1, Math.round(this.#count.value ?? 1)) }),
    }) as unknown as NumberControl;
    this.#ends = h("div", { class: "recur-ends" }, this.#endsGroup, h("div", { class: "hrow" }, this.#until, this.#count));
    this.#summary = h("p", { class: "hint recur-summary" });
    this.append(
      h(
        "fieldset",
        { class: "recur" },
        h("legend", { class: "field-label sr-only" }, "Repeat"),
        h("div", { class: "hrow" }, this.#freq, this.#intervalRow),
        this.#weekdays,
        this.#monthly,
        this.#customField,
        this.#ends,
        this.#summary,
      ),
    );
  }

  render(): void {
    const f = this.form;
    const start = this.start;
    if (this.#freq.value !== f.freq) this.#freq.value = f.freq;

    const repeating = f.freq !== "none" && f.freq !== "custom";
    this.#intervalRow.hidden = !repeating;
    if (this.#interval.value !== f.interval) this.#interval.value = f.interval;
    setText(this.#unit, repeating ? `${UNIT[f.freq]}${f.interval === 1 ? "" : "s"}` : "");

    this.#weekdays.hidden = f.freq !== "weekly";
    const ws = weekStart();
    const days = [...WEEKDAY_CODES.slice(ws), ...WEEKDAY_CODES.slice(0, ws)];
    syncKeyed(
      this.#weekdays,
      days,
      (c) => c,
      (c) => h("push-button", { toggle: true, class: "recur-day", "aria-label": weekdayName(c), onClick: () => this.#toggleDay(c) }, weekdayShortName(c).slice(0, 2)),
      (el, c) => {
        const b = el as unknown as PushButton;
        const on = f.byDay.includes(c);
        if (b.pressed !== on) b.pressed = on;
      },
    );

    this.#monthly.hidden = f.freq !== "monthly";
    if (start) {
      const info = nthWeekdayOfMonth(start);
      const code = WEEKDAY_CODES[dayOfWeek(start)];
      const options = [
        { value: "day", label: `On day ${f.monthlyMode === "day" ? f.monthDay : parseDate(start).d}` },
        { value: "nth", label: `On the ${ordinalWord(Math.min(4, info.nth))} ${weekdayName(code)}` },
      ];
      if (info.isLast) options.push({ value: "last", label: `On the last ${weekdayName(code)}` });
      if (JSON.stringify(this.#monthly.options) !== JSON.stringify(options)) this.#monthly.options = options;
    }
    const monthly = f.monthlyMode === "day" ? "day" : f.nth === -1 ? "last" : "nth";
    if (this.#monthly.value !== monthly) this.#monthly.value = monthly;

    this.#customField.hidden = f.freq !== "custom";
    if (this.#custom.value !== f.custom) this.#custom.value = f.custom;

    this.#ends.hidden = !repeating;
    if (this.#endsGroup.value !== f.ends) this.#endsGroup.value = f.ends;
    if (this.#until.value !== f.until) this.#until.value = f.until;
    this.#until.setAttribute("min", start);
    this.#until.disabled = f.ends !== "on";
    if (this.#count.value !== f.count) this.#count.value = f.count;
    this.#count.disabled = f.ends !== "after";

    this.#summary.hidden = f.freq === "none";
    setText(this.#summary, f.freq === "none" ? "" : describeRecurrence(f, start));
  }

  #set(patch: Partial<RecurrenceForm>): void {
    this.value = { ...this.form, ...patch };
    this.dispatchEvent(new Event("change", { bubbles: true }));
  }

  #changeFreq(freq: RecurrenceFreq): void {
    const f = this.form;
    if (freq === f.freq) return;
    if (freq === "custom" && !f.custom) {
      const current = buildRRule({ ...f, freq: f.freq === "none" ? "weekly" : f.freq }, { tz: this.tz, allDay: this.allDay });
      this.#set({ freq, custom: current ?? "FREQ=WEEKLY" });
    } else this.#set({ freq });
  }

  #changeMonthly(mode: string): void {
    const start = this.start;
    const code: WeekdayCode = WEEKDAY_CODES[dayOfWeek(start)];
    if (mode === "day") this.#set({ monthlyMode: "day", monthDay: parseDate(start).d });
    else if (mode === "last") this.#set({ monthlyMode: "nth", nth: -1, nthDay: code });
    else this.#set({ monthlyMode: "nth", nth: Math.min(4, nthWeekdayOfMonth(start).nth), nthDay: code });
  }

  #toggleDay(c: WeekdayCode): void {
    const f = this.form;
    const has = f.byDay.includes(c);
    this.#set({ byDay: has ? f.byDay.filter((d) => d !== c) : [...f.byDay, c] });
  }
}
define("cal-recurrence-editor", RecurrenceEditor);
