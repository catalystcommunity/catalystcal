import { For, Show, createMemo, createSignal } from "solid-js";
import type { EventStatus, EventTransparency, EventVisibility, Reminder } from "@catalystcal/client";
import { Modal } from "~/components/Modal";
import { Icon } from "~/components/Icons";
import { ColorPicker } from "~/components/ColorPicker";
import { RecurrenceEditor } from "~/components/RecurrenceEditor";
import { AttendeesEditor } from "~/components/AttendeesEditor";
import { FreeBusyPanel } from "~/components/FreeBusyPanel";
import { eventClient } from "~/data/clients";
import { accessAtLeast, bumpEvents, calendarColor, useCalendars } from "~/stores/calendars";
import { isActing, effectivePerson } from "~/stores/auth";
import { askScope } from "~/stores/dialogs";
import { toast } from "~/stores/toast";
import type { EditorState } from "~/stores/editor";
import { EventForm, buildUpdate, formToEvent, isSeriesOccurrence, validateForm } from "~/lib/eventForm";
import { defaultRecurrenceForm } from "~/lib/rrule";
import { addDays, daysBetween } from "~/lib/dates";
import { hhmmToMinutes, minutesToHHMM, offsetLabel, zoneList, zonedToUtc } from "~/lib/tz";
import { displayZone } from "~/stores/prefs";
import { personName } from "~/lib/people";
import { errorMessage } from "~/transport/csilrpc";

type Unit = "minutes" | "hours" | "days";
const UNIT_MIN: Record<Unit, number> = { minutes: 1, hours: 60, days: 1440 };
const unitOf = (m: number): Unit => (m > 0 && m % 1440 === 0 ? "days" : m > 0 && m % 60 === 0 ? "hours" : "minutes");

export function EventEditor(props: { state: EditorState; onClose: () => void }) {
  const [form, setForm] = createSignal<EventForm>(structuredClone(props.state.form));
  const [error, setError] = createSignal<string | null>(null);
  const [saving, setSaving] = createSignal(false);
  const [more, setMore] = createSignal(false);
  const editing = () => props.state.mode === "edit";
  const original = props.state.mode === "edit" ? props.state.occ : null;
  const cals = useCalendars();
  const zones = zoneList(displayZone());

  const writable = createMemo(() => cals().filter((c) => c.kind === "local" && accessAtLeast(c.access, "write")));
  const set = (patch: Partial<EventForm>) => setForm({ ...form(), ...patch });
  const f = form;

  const duration = () => {
    const x = f();
    return daysBetween(x.startDate, x.endDate) * 1440 + (x.endMin - x.startMin);
  };

  const setStartDate = (d: string) => {
    if (!d) return;
    const x = f();
    const span = daysBetween(x.startDate, x.endDate);
    const patch: Partial<EventForm> = { startDate: d, endDate: addDays(d, Math.max(0, span)) };
    if (x.recurrence.freq === "none") patch.recurrence = defaultRecurrenceForm(d);
    set(patch);
  };
  const setStartTime = (v: string) => {
    const m = hhmmToMinutes(v);
    if (Number.isNaN(m)) return;
    const x = f();
    const dur = Math.max(15, duration());
    const end = m + dur;
    set({ startMin: m, endMin: end % 1440, endDate: addDays(x.startDate, Math.floor(end / 1440)) });
  };
  const setEndTime = (v: string) => {
    const m = hhmmToMinutes(v);
    if (Number.isNaN(m)) return;
    const x = f();
    // An end before the start on the same day means the next day.
    if (x.endDate === x.startDate && m <= x.startMin) set({ endMin: m, endDate: addDays(x.startDate, 1) });
    else set({ endMin: m });
  };
  const pickTime = (startMin: number) => {
    const dur = Math.max(15, duration());
    const end = startMin + dur;
    set({ startMin, endMin: end % 1440, endDate: addDays(f().startDate, Math.floor(end / 1440)) });
  };

  const setReminder = (i: number, r: Reminder) => set({ reminders: f().reminders.map((x, j) => (j === i ? r : x)) });

  const save = async (e?: SubmitEvent) => {
    e?.preventDefault();
    const x = f();
    const problem = validateForm(x);
    if (problem) return setError(problem);
    setError(null);
    setSaving(true);
    try {
      if (props.state.mode === "create") {
        await eventClient.createEvent(formToEvent(x));
        toast(isActing() ? `Event created as ${personName(effectivePerson())}.` : "Event created.");
      } else {
        const occ = props.state.occ;
        let scope: "all" | "this" | "this_and_future" = "all";
        if (isSeriesOccurrence(occ)) {
          const s = await askScope("save");
          if (!s) return;
          scope = s;
        }
        const moving = x.calendarId !== occ.event.calendarId;
        const req = buildUpdate({ ...x, calendarId: occ.event.calendarId }, occ, scope);
        await eventClient.updateEvent(req);
        if (moving) await eventClient.moveEvent({ eventId: occ.event.eventId, calendarId: x.calendarId });
        toast("Event saved.");
      }
      bumpEvents();
      props.onClose();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const tzOffset = () => offsetLabel(zonedToUtc(f().startDate, f().startMin, f().timezone), f().timezone);
  const identityAttendees = () => f().attendees.filter((a) => a.identity);

  return (
    <Modal
      title={editing() ? "Edit event" : "New event"}
      wide
      onClose={props.onClose}
      initialFocus={() => document.getElementById("ev-title") ?? undefined}
      footer={
        <>
          <Show when={isActing()}>
            <span class="small muted grow">Saved as {personName(effectivePerson())}.</span>
          </Show>
          <button type="button" class="btn" onClick={props.onClose}>
            Cancel
          </button>
          <button type="submit" form="event-form" class="btn btn-primary" disabled={saving()}>
            {saving() ? "Saving…" : editing() ? "Save" : "Create event"}
          </button>
        </>
      }
    >
      <form id="event-form" class="editor" onSubmit={save} novalidate>
        <div class="editor-main">
          <label for="ev-title" class="sr-only">
            Title
          </label>
          <input id="ev-title" class="input input-title" placeholder="Add a title" maxLength={1024} value={f().title} onInput={(e) => set({ title: e.currentTarget.value })} />

          <div class="grid-2">
            <div class="field">
              <label for="ev-cal">Calendar</label>
              <div class="row cal-select">
                <span class="swatch" style={{ "--sw": f().color || calendarColor(writable().find((c) => c.calendarId === f().calendarId)) }} />
                <select id="ev-cal" class="select grow" value={f().calendarId} onChange={(e) => set({ calendarId: e.currentTarget.value })}>
                  <Show when={!writable().some((c) => c.calendarId === f().calendarId)}>
                    <option value={f().calendarId}>(current calendar)</option>
                  </Show>
                  <For each={writable()}>
                    {(c) => (
                      <option value={c.calendarId}>
                        {c.name}
                        {c.owner.userId !== effectivePerson()?.userId ? ` — ${personName(c.owner)}` : ""}
                      </option>
                    )}
                  </For>
                </select>
              </div>
              <Show when={original && f().calendarId !== original.event.calendarId}>
                <span class="hint">The event moves to this calendar when you save.</span>
              </Show>
            </div>
            <div class="field">
              <span class="field-label">Color</span>
              <ColorPicker label="Event color" allowNone value={f().color} onChange={(c) => set({ color: c })} />
            </div>
          </div>

          <fieldset class="when">
            <legend class="field-label">When</legend>
            <label class="check">
              <input type="checkbox" checked={f().allDay} onChange={(e) => set({ allDay: e.currentTarget.checked })} />
              All day
            </label>
            <div class="when-grid">
              <label class="sr-only" for="ev-sd">
                Start date
              </label>
              <input id="ev-sd" class="input input-date" type="date" value={f().startDate} onInput={(e) => setStartDate(e.currentTarget.value)} />
              <Show when={!f().allDay}>
                <label class="sr-only" for="ev-st">
                  Start time
                </label>
                <input id="ev-st" class="input input-time" type="time" step="300" value={minutesToHHMM(f().startMin)} onInput={(e) => setStartTime(e.currentTarget.value)} />
              </Show>
              <span class="muted" aria-hidden="true">
                to
              </span>
              <Show when={!f().allDay}>
                <label class="sr-only" for="ev-et">
                  End time
                </label>
                <input id="ev-et" class="input input-time" type="time" step="300" value={minutesToHHMM(f().endMin)} onInput={(e) => setEndTime(e.currentTarget.value)} />
              </Show>
              <label class="sr-only" for="ev-ed">
                End date
              </label>
              <input id="ev-ed" class="input input-date" type="date" value={f().endDate} min={f().startDate} onInput={(e) => e.currentTarget.value && set({ endDate: e.currentTarget.value })} />
            </div>
            <Show when={!f().allDay}>
              <div class="field">
                <label for="ev-tz">Time zone</label>
                <select id="ev-tz" class="select" value={f().timezone} onChange={(e) => set({ timezone: e.currentTarget.value })}>
                  <For each={zones}>{(z) => <option value={z}>{z.replace(/_/g, " ")}</option>}</For>
                </select>
                <span class="hint tnum">{tzOffset()}. Repeats keep this zone’s wall-clock time.</span>
              </div>
            </Show>
          </fieldset>

          <RecurrenceEditor value={f().recurrence} start={f().startDate} tz={f().timezone} allDay={f().allDay} onChange={(r) => set({ recurrence: r })} />

          <div class="grid-2">
            <div class="field">
              <label for="ev-loc">Location</label>
              <input id="ev-loc" class="input" value={f().location} onInput={(e) => set({ location: e.currentTarget.value })} />
            </div>
            <div class="field">
              <label for="ev-conf">Video call link</label>
              <input id="ev-conf" class="input" type="url" inputMode="url" placeholder="https://" value={f().conferenceUrl} onInput={(e) => set({ conferenceUrl: e.currentTarget.value })} />
            </div>
          </div>

          <AttendeesEditor value={f().attendees} onChange={(a) => set({ attendees: a })} />

          <div class="field">
            <span class="field-label">Reminders</span>
            <For each={f().reminders}>
              {(r, i) => {
                const unit = () => unitOf(r.minutesBefore);
                return (
                  <div class="row reminder-row">
                    <input
                      class="input input-num"
                      type="number"
                      min="0"
                      aria-label="Reminder amount"
                      value={r.minutesBefore / UNIT_MIN[unit()]}
                      onInput={(e) => setReminder(i(), { ...r, minutesBefore: Math.max(0, Math.round(Number(e.currentTarget.value) || 0)) * UNIT_MIN[unit()] })}
                    />
                    <select class="select select-sm" aria-label="Reminder unit" value={unit()} onChange={(e) => setReminder(i(), { ...r, minutesBefore: (r.minutesBefore / UNIT_MIN[unit()]) * UNIT_MIN[e.currentTarget.value as Unit] })}>
                      <option value="minutes">minutes</option>
                      <option value="hours">hours</option>
                      <option value="days">days</option>
                    </select>
                    <span class="small muted">before, as</span>
                    <select class="select select-sm" aria-label="Reminder method" value={r.method ?? "display"} onChange={(e) => setReminder(i(), { ...r, method: e.currentTarget.value as Reminder["method"] })}>
                      <option value="display">notification</option>
                      <option value="email">email</option>
                    </select>
                    <button type="button" class="icon-btn icon-btn-sm" aria-label="Remove reminder" onClick={() => set({ reminders: f().reminders.filter((_, j) => j !== i()) })}>
                      <Icon name="close" size={14} />
                    </button>
                  </div>
                );
              }}
            </For>
            <div>
              <button type="button" class="link-btn small" onClick={() => set({ reminders: [...f().reminders, { minutesBefore: 30, method: "display" }] })}>
                + Add reminder
              </button>
            </div>
          </div>

          <div class="field">
            <label for="ev-desc">Description</label>
            <textarea id="ev-desc" class="textarea" value={f().description} onInput={(e) => set({ description: e.currentTarget.value })} />
          </div>

          <button type="button" class="link-btn small" aria-expanded={more()} aria-controls="ev-more" onClick={() => setMore(!more())}>
            {more() ? "Fewer options" : "More options: status, show as, privacy, categories"}
          </button>
          <Show when={more()}>
            <div id="ev-more" class="grid-2">
              <div class="field">
                <label for="ev-status">Status</label>
                <select id="ev-status" class="select" value={f().status} onChange={(e) => set({ status: e.currentTarget.value as EventStatus })}>
                  <option value="confirmed">Confirmed</option>
                  <option value="tentative">Tentative</option>
                  <option value="cancelled">Cancelled</option>
                </select>
              </div>
              <div class="field">
                <label for="ev-transp">Show me as</label>
                <select id="ev-transp" class="select" value={f().transparency} onChange={(e) => set({ transparency: e.currentTarget.value as EventTransparency })}>
                  <option value="opaque">Busy</option>
                  <option value="transparent">Free</option>
                </select>
              </div>
              <div class="field">
                <label for="ev-vis">Privacy</label>
                <select id="ev-vis" class="select" value={f().visibility} onChange={(e) => set({ visibility: e.currentTarget.value as EventVisibility })}>
                  <option value="default">Calendar default</option>
                  <option value="public">Public</option>
                  <option value="private">Private (others see “busy”)</option>
                  <option value="confidential">Confidential</option>
                </select>
              </div>
              <div class="field">
                <label for="ev-cats">Categories</label>
                <input
                  id="ev-cats"
                  class="input"
                  placeholder="work, travel"
                  value={f().categories.join(", ")}
                  onChange={(e) =>
                    set({
                      categories: e.currentTarget.value
                        .split(",")
                        .map((s) => s.trim())
                        .filter(Boolean),
                    })
                  }
                />
              </div>
              <div class="field" style={{ "grid-column": "1 / -1" }}>
                <label for="ev-url">Web page</label>
                <input id="ev-url" class="input" type="url" inputMode="url" placeholder="https://" value={f().url} onInput={(e) => set({ url: e.currentTarget.value })} />
              </div>
            </div>
          </Show>

          <Show when={error()}>
            <p class="alert" role="alert">
              {error()}
            </p>
          </Show>
        </div>

        <Show when={identityAttendees().length > 0}>
          <aside class="editor-side">
            <FreeBusyPanel
              attendees={f().attendees}
              date={f().startDate}
              endDate={f().endDate}
              startMin={f().startMin}
              endMin={f().endMin}
              tz={f().timezone}
              allDay={f().allDay}
              onPick={pickTime}
            />
          </aside>
        </Show>
      </form>
    </Modal>
  );
}
