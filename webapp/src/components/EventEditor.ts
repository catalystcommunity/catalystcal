import type { EventStatus, EventTransparency, EventVisibility, Reminder } from "@catalystcal/client";
import { eventClient } from "~/data/clients";
import { CALENDARS, bumpEvents, calendarColor, calendars, canAddEvents } from "~/stores/calendars";
import { AUTH, effectivePerson, isActing } from "~/stores/auth";
import { askScope } from "~/stores/dialogs";
import { closeEditor, type EditorState } from "~/stores/editor";
import { toast } from "~/stores/toast";
import { displayZone } from "~/stores/prefs";
import { EventForm, buildUpdate, formToEvent, isSeriesOccurrence, validateForm } from "~/lib/eventForm";
import { defaultRecurrenceForm, type RecurrenceForm } from "~/lib/rrule";
import { addDays, daysBetween } from "~/lib/dates";
import { hhmmToMinutes, isValidZone, minutesToHHMM, offsetLabel, zoneList, zonedToUtc } from "~/lib/tz";
import { personName } from "~/lib/people";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { h, setAttr, setText, syncKeyed, uid } from "~/ui/dom";
import { icon } from "~/ui/icons";
import { openModal, type CheckControl, type DialogWindow, type OptionControl, type PushButton, type ValueControl } from "~/ui/controls";
import "~/components/ColorPicker";
import "~/components/RecurrenceEditor";
import "~/components/AttendeesEditor";
import "~/components/FreeBusyPanel";
import type { ColorPicker } from "~/components/ColorPicker";
import type { RecurrenceEditor } from "~/components/RecurrenceEditor";
import type { AttendeesEditor } from "~/components/AttendeesEditor";
import type { FreeBusyPanel } from "~/components/FreeBusyPanel";

type Unit = "minutes" | "hours" | "days";
const UNIT_MIN: Record<Unit, number> = { minutes: 1, hours: 60, days: 1440 };
const unitOf = (m: number): Unit => (m > 0 && m % 1440 === 0 ? "days" : m > 0 && m % 60 === 0 ? "hours" : "minutes");

interface NumberControl extends HTMLElement {
  value: number | null;
}

/** A reminder row of the form, with a stable key for its row element. */
interface ReminderRow {
  key: string;
  reminder: Reminder;
}

/**
 * The event editor dialog. Set `state` before the element connects; it
 * opens a modal at the first connect, and calls closeEditor() when the
 * modal closes.
 */
export class EventEditor extends CalElement {
  static watches = [CALENDARS, AUTH];
  declare state: EditorState;
  #form!: EventForm;
  #reminders: ReminderRow[] = [];
  #dialog: DialogWindow | null = null;
  #opened = false;
  #saving = false;

  #title!: ValueControl;
  #calendar!: OptionControl;
  #calSwatch!: HTMLElement;
  #moveHint!: HTMLElement;
  #color!: ColorPicker;
  #allDay!: CheckControl;
  #startDate!: ValueControl;
  #startTime!: ValueControl;
  #endTime!: ValueControl;
  #endDate!: ValueControl;
  #zoneField!: HTMLElement;
  #zone!: OptionControl;
  #zoneHint!: HTMLElement;
  #recurrence!: RecurrenceEditor;
  #location!: ValueControl;
  #conference!: ValueControl;
  #attendees!: AttendeesEditor;
  #reminderList!: HTMLElement;
  #description!: ValueControl;
  #moreButton!: HTMLButtonElement;
  #more!: HTMLElement;
  #status!: OptionControl;
  #transparency!: OptionControl;
  #visibility!: OptionControl;
  #categories!: ValueControl;
  #url!: ValueControl;
  #error!: HTMLElement;
  #side!: HTMLElement;
  #freeBusy!: FreeBusyPanel;
  #actingNote!: HTMLElement;
  #save!: PushButton & { label: string };
  #content!: HTMLElement;

  get #editing(): boolean {
    return this.state.mode === "edit";
  }

  protected build(): void {
    this.#form = structuredClone(this.state.form);
    this.#reminders = this.#form.reminders.map((r) => ({ key: uid("rem"), reminder: r }));
    const f = this.#form;
    const mine = <T extends Event>(handler: (e: T) => void) => (e: T) => {
      // Events of nested controls also bubble through our child elements.
      if (e.target === e.currentTarget) handler(e);
    };

    this.#title = h("text-field", {
      "aria-label": "Title",
      class: "input-title",
      placeholder: "Add a title",
      maxlength: "1024",
      value: f.title,
      onInput: () => this.#set({ title: this.#title.value }),
    }) as ValueControl;

    this.#calSwatch = h("span", { class: "swatch" });
    this.#calendar = h("combo-box", { label: "Calendar", class: "grow", onChange: () => this.#set({ calendarId: this.#calendar.value }) }) as OptionControl;
    this.#moveHint = h("span", { class: "hint", hidden: true }, "The event moves to this calendar when you save.");
    this.#color = h("cal-color-picker", {
      label: "Event color",
      "allow-none": true,
      value: f.color,
      onChange: mine(() => this.#set({ color: this.#color.value })),
    }) as unknown as ColorPicker;

    this.#allDay = h("check-box", { checked: f.allDay, onChange: () => this.#set({ allDay: this.#allDay.checked }) }, "All day") as CheckControl;
    this.#startDate = h("date-time-picker", { mode: "date", label: "Start date", class: "input-date", onChange: () => this.#setStartDate(this.#startDate.value) }) as ValueControl;
    this.#startTime = h("date-time-picker", { mode: "time", "minute-step": "5", label: "Start time", class: "input-time", onChange: () => this.#setStartTime(this.#startTime.value) }) as ValueControl;
    this.#endTime = h("date-time-picker", { mode: "time", "minute-step": "5", label: "End time", class: "input-time", onChange: () => this.#setEndTime(this.#endTime.value) }) as ValueControl;
    this.#endDate = h("date-time-picker", {
      mode: "date",
      label: "End date",
      class: "input-date",
      onChange: () => this.#endDate.value && this.#set({ endDate: this.#endDate.value }),
    }) as ValueControl;
    this.#zone = h("combo-box", {
      label: "Time zone",
      editable: true,
      ".options": zoneList(displayZone()).map((z) => ({ value: z, label: z.replace(/_/g, " ") })),
      onChange: () => {
        if (isValidZone(this.#zone.value)) this.#set({ timezone: this.#zone.value });
        else this.#zone.value = this.#form.timezone;
      },
    }) as OptionControl;
    this.#zoneHint = h("span", { class: "hint tnum" });
    this.#zoneField = h("div", { class: "field" }, this.#zone, this.#zoneHint);

    this.#recurrence = h("cal-recurrence-editor", {
      onChange: mine(() => this.#set({ recurrence: this.#recurrence.value as RecurrenceForm })),
    }) as unknown as RecurrenceEditor;
    this.#recurrence.value = f.recurrence;

    this.#location = h("text-field", { label: "Location", value: f.location, onInput: () => this.#set({ location: this.#location.value }) }) as ValueControl;
    this.#conference = h("text-field", {
      label: "Video call link",
      type: "url",
      inputmode: "url",
      placeholder: "https://",
      value: f.conferenceUrl,
      onInput: () => this.#set({ conferenceUrl: this.#conference.value }),
    }) as ValueControl;

    this.#attendees = h("cal-attendees-editor", { onChange: mine(() => this.#set({ attendees: this.#attendees.value })) }) as unknown as AttendeesEditor;
    this.#attendees.value = f.attendees;

    this.#reminderList = h("div", { class: "reminder-list" });
    this.#description = h("text-area", { label: "Description", value: f.description, onInput: () => this.#set({ description: this.#description.value }) }) as ValueControl;

    const moreId = uid("ev-more");
    this.#moreButton = h("button", { type: "button", class: "link-btn small", "aria-controls": moreId, onClick: () => this.#toggleMore() });
    this.#status = h("combo-box", {
      label: "Status",
      ".options": [
        { value: "confirmed", label: "Confirmed" },
        { value: "tentative", label: "Tentative" },
        { value: "cancelled", label: "Cancelled" },
      ],
      value: f.status,
      onChange: () => this.#set({ status: this.#status.value as EventStatus }),
    }) as OptionControl;
    this.#transparency = h("combo-box", {
      label: "Show me as",
      ".options": [
        { value: "opaque", label: "Busy" },
        { value: "transparent", label: "Free" },
      ],
      value: f.transparency,
      onChange: () => this.#set({ transparency: this.#transparency.value as EventTransparency }),
    }) as OptionControl;
    this.#visibility = h("combo-box", {
      label: "Privacy",
      ".options": [
        { value: "default", label: "Calendar default" },
        { value: "public", label: "Public" },
        { value: "private", label: "Private (others see “busy”)" },
        { value: "confidential", label: "Confidential" },
      ],
      value: f.visibility,
      onChange: () => this.#set({ visibility: this.#visibility.value as EventVisibility }),
    }) as OptionControl;
    this.#categories = h("text-field", {
      label: "Categories",
      placeholder: "work, travel",
      value: f.categories.join(", "),
      onChange: () =>
        this.#set({
          categories: this.#categories.value
            .split(",")
            .map((s) => s.trim())
            .filter(Boolean),
        }),
    }) as ValueControl;
    this.#url = h("text-field", {
      label: "Web page",
      type: "url",
      inputmode: "url",
      placeholder: "https://",
      value: f.url,
      onInput: () => this.#set({ url: this.#url.value }),
    }) as ValueControl;
    this.#more = h(
      "div",
      { id: moreId, class: "grid-2", hidden: true },
      h("div", { class: "field" }, this.#status),
      h("div", { class: "field" }, this.#transparency),
      h("div", { class: "field" }, this.#visibility),
      h("div", { class: "field" }, this.#categories),
      h("div", { class: "field", style: { "grid-column": "1 / -1" } }, this.#url),
    );

    this.#error = h("p", { class: "alert", role: "alert", hidden: true });
    this.#freeBusy = h("cal-free-busy-panel", {
      "onFree-busy-pick": (e: CustomEvent<{ startMin: number }>) => this.#pickTime(e.detail.startMin),
    }) as unknown as FreeBusyPanel;
    this.#side = h("aside", { class: "editor-side", hidden: true }, this.#freeBusy);
    this.#actingNote = h("p", { class: "small muted editor-acting", hidden: true });

    const form = h(
      "form",
      { class: "editor", novalidate: true, onSubmit: (e: SubmitEvent) => (e.preventDefault(), void this.#onSave()) },
      h(
        "div",
        { class: "editor-main" },
        this.#title,
        h(
          "div",
          { class: "grid-2" },
          h("div", { class: "field" }, h("div", { class: "hrow cal-select" }, this.#calSwatch, this.#calendar), this.#moveHint),
          h("div", { class: "field" }, h("span", { class: "field-label" }, "Color"), this.#color),
        ),
        h(
          "fieldset",
          { class: "when" },
          h("legend", { class: "field-label" }, "When"),
          this.#allDay,
          h("div", { class: "when-grid" }, this.#startDate, this.#startTime, h("span", { class: "muted", "aria-hidden": "true" }, "to"), this.#endTime, this.#endDate),
          this.#zoneField,
        ),
        this.#recurrence,
        h("div", { class: "grid-2" }, h("div", { class: "field" }, this.#location), h("div", { class: "field" }, this.#conference)),
        this.#attendees,
        h(
          "div",
          { class: "field" },
          h("span", { class: "field-label" }, "Reminders"),
          this.#reminderList,
          h(
            "div",
            null,
            h(
              "button",
              {
                type: "button",
                class: "link-btn small",
                onClick: () => {
                  this.#reminders = [...this.#reminders, { key: uid("rem"), reminder: { minutesBefore: 30, method: "display" } }];
                  this.#syncReminders();
                },
              },
              "+ Add reminder",
            ),
          ),
        ),
        h("div", { class: "field" }, this.#description),
        this.#moreButton,
        this.#more,
        this.#error,
        this.#actingNote,
      ),
      this.#side,
    );
    this.#content = form;
    this.#save = h("push-button", { variant: "primary", onClick: () => void this.#onSave() }) as PushButton & { label: string };
    this.#syncReminders();
    this.#toggleMore(false);
  }

  protected connected(): void {
    if (this.#opened) return;
    this.#opened = true;
    const cancel = h("button", { type: "button", value: "cancel" }, "Cancel");
    const { dialog, result } = openModal({
      label: this.#editing ? "Edit event" : "New event",
      className: "editor-dialog",
      children: [this.#content],
      buttons: [cancel, this.#save],
    });
    this.#dialog = dialog;
    void result.then(() => {
      this.#dialog = null;
      closeEditor();
    });
    requestAnimationFrame(() => this.#title.focus());
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.#dialog?.close("cancel");
  }

  render(): void {
    const f = this.#form;
    const original = this.state.mode === "edit" ? this.state.occ : null;
    const writable = calendars().filter((c) => c.kind === "local" && canAddEvents(c.access));
    const me = effectivePerson();
    const options = writable.map((c) => ({ value: c.calendarId, label: c.owner.userId !== me?.userId ? `${c.name} — ${personName(c.owner)}` : c.name }));
    if (!writable.some((c) => c.calendarId === f.calendarId)) options.unshift({ value: f.calendarId, label: "(current calendar)" });
    if (JSON.stringify(this.#calendar.options) !== JSON.stringify(options)) this.#calendar.options = options;
    if (this.#calendar.value !== f.calendarId) this.#calendar.value = f.calendarId;
    this.#calSwatch.style.setProperty("--sw", f.color || calendarColor(writable.find((c) => c.calendarId === f.calendarId)));
    this.#moveHint.hidden = !(original && f.calendarId !== original.event.calendarId);
    if (this.#color.value !== f.color) this.#color.value = f.color;

    if (this.#allDay.checked !== f.allDay) this.#allDay.checked = f.allDay;
    if (this.#startDate.value !== f.startDate) this.#startDate.value = f.startDate;
    if (this.#endDate.value !== f.endDate) this.#endDate.value = f.endDate;
    setAttr(this.#endDate, "min", f.startDate);
    const st = minutesToHHMM(f.startMin);
    const et = minutesToHHMM(f.endMin);
    if (this.#startTime.value !== st) this.#startTime.value = st;
    if (this.#endTime.value !== et) this.#endTime.value = et;
    this.#startTime.hidden = f.allDay;
    this.#endTime.hidden = f.allDay;
    this.#zoneField.hidden = f.allDay;
    if (this.#zone.value !== f.timezone) this.#zone.value = f.timezone;
    setText(this.#zoneHint, `${offsetLabel(zonedToUtc(f.startDate, f.startMin, f.timezone), f.timezone)}. Repeats keep this zone’s wall-clock time.`);

    this.#recurrence.start = f.startDate;
    this.#recurrence.tz = f.timezone;
    this.#recurrence.allDay = f.allDay;
    if (this.#recurrence.value !== f.recurrence) this.#recurrence.value = f.recurrence;
    if (this.#attendees.value !== f.attendees) this.#attendees.value = f.attendees;

    const identities = f.attendees.filter((a) => a.identity);
    this.#side.hidden = identities.length === 0;
    if (identities.length) {
      Object.assign(this.#freeBusy, { attendees: f.attendees, date: f.startDate, endDate: f.endDate, startMin: f.startMin, endMin: f.endMin, tz: f.timezone, allDay: f.allDay });
    }

    this.#actingNote.hidden = !isActing();
    setText(this.#actingNote, isActing() ? `Saved as ${personName(effectivePerson())}.` : "");
    this.#save.disabled = this.#saving;
    const saveLabel = this.#saving ? "Saving…" : this.#editing ? "Save" : "Create event";
    if (this.#save.label !== saveLabel) this.#save.label = saveLabel;
  }

  #set(patch: Partial<EventForm>): void {
    Object.assign(this.#form, patch);
    this.invalidate();
  }

  #duration(): number {
    const x = this.#form;
    return daysBetween(x.startDate, x.endDate) * 1440 + (x.endMin - x.startMin);
  }

  #setStartDate(d: string): void {
    if (!d) return;
    const x = this.#form;
    const span = daysBetween(x.startDate, x.endDate);
    const patch: Partial<EventForm> = { startDate: d, endDate: addDays(d, Math.max(0, span)) };
    if (x.recurrence.freq === "none") patch.recurrence = defaultRecurrenceForm(d);
    this.#set(patch);
  }

  #setStartTime(v: string): void {
    const m = hhmmToMinutes(v);
    if (Number.isNaN(m)) return;
    const dur = Math.max(15, this.#duration());
    const end = m + dur;
    this.#set({ startMin: m, endMin: end % 1440, endDate: addDays(this.#form.startDate, Math.floor(end / 1440)) });
  }

  #setEndTime(v: string): void {
    const m = hhmmToMinutes(v);
    if (Number.isNaN(m)) return;
    const x = this.#form;
    // An end before the start on the same day means the next day.
    if (x.endDate === x.startDate && m <= x.startMin) this.#set({ endMin: m, endDate: addDays(x.startDate, 1) });
    else this.#set({ endMin: m });
  }

  #pickTime(startMin: number): void {
    const dur = Math.max(15, this.#duration());
    const end = startMin + dur;
    this.#set({ startMin, endMin: end % 1440, endDate: addDays(this.#form.startDate, Math.floor(end / 1440)) });
  }

  #toggleMore(open = this.#more.hidden): void {
    this.#more.hidden = !open;
    setAttr(this.#moreButton, "aria-expanded", String(open));
    setText(this.#moreButton, open ? "Fewer options" : "More options: status, show as, privacy, categories");
  }

  #syncReminders(): void {
    this.#form.reminders = this.#reminders.map((r) => r.reminder);
    syncKeyed(
      this.#reminderList,
      this.#reminders,
      (r) => r.key,
      (r) => this.#reminderRow(r.key),
      (el, r) => {
        const unit = unitOf(r.reminder.minutesBefore);
        const amount = el.querySelector("spin-box") as unknown as NumberControl;
        const [unitBox, methodBox] = el.querySelectorAll("combo-box") as unknown as OptionControl[];
        const n = r.reminder.minutesBefore / UNIT_MIN[unit];
        if (amount.value !== n) amount.value = n;
        if (unitBox.value !== unit) unitBox.value = unit;
        const method = r.reminder.method ?? "display";
        if (methodBox.value !== method) methodBox.value = method;
      },
    );
  }

  #reminderRow(key: string): HTMLElement {
    const at = () => this.#reminders.find((r) => r.key === key)!;
    const update = (reminder: Reminder) => {
      this.#reminders = this.#reminders.map((r) => (r.key === key ? { key, reminder } : r));
      this.#syncReminders();
    };
    const amount = h("spin-box", {
      "aria-label": "Reminder amount",
      min: "0",
      class: "input-num",
      onChange: () => {
        const r = at().reminder;
        const unit = unitOf(r.minutesBefore);
        update({ ...r, minutesBefore: Math.max(0, Math.round(amount.value ?? 0)) * UNIT_MIN[unit] });
      },
    }) as unknown as NumberControl;
    const unitBox = h("combo-box", {
      "aria-label": "Reminder unit",
      class: "select-sm",
      ".options": ["minutes", "hours", "days"],
      onChange: () => {
        const r = at().reminder;
        const unit = unitOf(r.minutesBefore);
        update({ ...r, minutesBefore: (r.minutesBefore / UNIT_MIN[unit]) * UNIT_MIN[unitBox.value as Unit] });
      },
    }) as OptionControl;
    const methodBox = h("combo-box", {
      "aria-label": "Reminder method",
      class: "select-sm",
      ".options": [
        { value: "display", label: "notification" },
        { value: "email", label: "email" },
      ],
      onChange: () => update({ ...at().reminder, method: methodBox.value as Reminder["method"] }),
    }) as OptionControl;
    return h(
      "div",
      { class: "hrow reminder-row" },
      amount,
      unitBox,
      h("span", { class: "small muted" }, "before, as"),
      methodBox,
      h(
        "button",
        {
          type: "button",
          class: "icon-btn icon-btn-sm",
          "aria-label": "Remove reminder",
          onClick: () => {
            this.#reminders = this.#reminders.filter((r) => r.key !== key);
            this.#syncReminders();
          },
        },
        icon("close", 14),
      ),
    );
  }

  #showError(msg: string | null): void {
    this.#error.hidden = !msg;
    setText(this.#error, msg ?? "");
  }

  async #onSave(): Promise<void> {
    if (this.#saving) return;
    if (await this.#trySave()) this.#dialog?.close("saved");
  }

  /** Save the form. True when it saved. */
  async #trySave(): Promise<boolean> {
    const x = this.#form;
    const problem = validateForm(x);
    if (problem) {
      this.#showError(problem);
      return false;
    }
    this.#showError(null);
    this.#saving = true;
    this.render();
    try {
      if (this.state.mode === "create") {
        await eventClient.createEvent(formToEvent(x));
        toast(isActing() ? `Event created as ${personName(effectivePerson())}.` : "Event created.");
      } else {
        const occ = this.state.occ;
        let scope: "all" | "this" | "this_and_future" = "all";
        if (isSeriesOccurrence(occ)) {
          const s = await askScope("save");
          if (!s) return false;
          scope = s;
        }
        const moving = x.calendarId !== occ.event.calendarId;
        await eventClient.updateEvent(buildUpdate({ ...x, calendarId: occ.event.calendarId }, occ, scope));
        if (moving) await eventClient.moveEvent({ eventId: occ.event.eventId, calendarId: x.calendarId });
        toast("Event saved.");
      }
      bumpEvents();
      return true;
    } catch (err) {
      if (this.isActive()) this.#showError(errorMessage(err));
      return false;
    } finally {
      this.#saving = false;
      if (this.isActive()) this.render();
    }
  }
}
define("cal-event-editor", EventEditor);
