/** New calendar, subscribe to a URL, and import .ics dialogs. */
import { For, Show, createSignal } from "solid-js";
import type { Calendar, ImportResult } from "@catalystcal/client";
import { Modal } from "~/components/Modal";
import { ColorPicker } from "~/components/ColorPicker";
import { calendarClient, icsClient } from "~/data/clients";
import { SWATCHES, accessAtLeast, bumpEvents, upsertCalendar, useCalendars } from "~/stores/calendars";
import { displayZone } from "~/stores/prefs";
import { zoneList } from "~/lib/tz";
import { errorMessage } from "~/transport/csilrpc";
import { toast } from "~/stores/toast";

export function NewCalendarDialog(props: { onClose: () => void; onCreated?: (c: Calendar) => void }) {
  const [name, setName] = createSignal("");
  const [color, setColor] = createSignal(SWATCHES[1]);
  const [tz, setTz] = createSignal(displayZone());
  const [description, setDescription] = createSignal("");
  const [error, setError] = createSignal<string | null>(null);
  const [busy, setBusy] = createSignal(false);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    if (!name().trim()) return setError("Give the calendar a name.");
    setBusy(true);
    try {
      const c = await calendarClient.createCalendar({ name: name().trim(), color: color(), timezone: tz(), description: description().trim() || undefined });
      upsertCalendar(c);
      toast(`Created “${c.name}”.`);
      props.onCreated?.(c);
      props.onClose();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="New calendar" onClose={props.onClose}>
      <form class="stack" onSubmit={submit}>
        <div class="field">
          <label for="nc-name">Name</label>
          <input id="nc-name" class="input" maxLength={256} value={name()} onInput={(e) => setName(e.currentTarget.value)} required data-autofocus />
        </div>
        <div class="field">
          <span class="field-label">Color</span>
          <ColorPicker label="Calendar color" value={color()} onChange={setColor} />
        </div>
        <div class="field">
          <label for="nc-tz">Time zone</label>
          <select id="nc-tz" class="select" value={tz()} onChange={(e) => setTz(e.currentTarget.value)}>
            <For each={zoneList(displayZone())}>{(z) => <option value={z}>{z.replace(/_/g, " ")}</option>}</For>
          </select>
        </div>
        <div class="field">
          <label for="nc-desc">Description</label>
          <textarea id="nc-desc" class="textarea" value={description()} onInput={(e) => setDescription(e.currentTarget.value)} />
        </div>
        <Show when={error()}>
          <p class="alert" role="alert">
            {error()}
          </p>
        </Show>
        <div class="row" style={{ "justify-content": "flex-end" }}>
          <button type="button" class="btn" onClick={props.onClose}>
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" disabled={busy()}>
            Create calendar
          </button>
        </div>
      </form>
    </Modal>
  );
}

export function SubscribeDialog(props: { onClose: () => void }) {
  const [url, setUrl] = createSignal("");
  const [name, setName] = createSignal("");
  const [color, setColor] = createSignal(SWATCHES[7]);
  const [error, setError] = createSignal<string | null>(null);
  const [busy, setBusy] = createSignal(false);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    let src = url().trim();
    if (src.startsWith("webcal://")) src = "https://" + src.slice("webcal://".length);
    if (!/^https?:\/\//i.test(src)) return setError("Enter an http, https, or webcal address.");
    setBusy(true);
    try {
      const fallback = (() => {
        try {
          return new URL(src).hostname;
        } catch {
          return "Subscription";
        }
      })();
      const c = await calendarClient.subscribeCalendar({ name: name().trim() || fallback, sourceUrl: src, color: color() });
      upsertCalendar(c);
      bumpEvents();
      toast(`Subscribed to “${c.name}”.`);
      props.onClose();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title="Subscribe to a calendar" onClose={props.onClose}>
      <form class="stack" onSubmit={submit}>
        <div class="field">
          <label for="sub-url">Calendar address</label>
          <input id="sub-url" class="input" type="url" inputMode="url" placeholder="https://example.com/team.ics" value={url()} onInput={(e) => setUrl(e.currentTarget.value)} required data-autofocus />
          <span class="hint">An iCalendar (.ics) address. catalystcal reads it again from time to time; you cannot change its events.</span>
        </div>
        <div class="field">
          <label for="sub-name">Name</label>
          <input id="sub-name" class="input" maxLength={256} placeholder="Uses the address host if empty" value={name()} onInput={(e) => setName(e.currentTarget.value)} />
        </div>
        <div class="field">
          <span class="field-label">Color</span>
          <ColorPicker label="Calendar color" value={color()} onChange={setColor} />
        </div>
        <Show when={error()}>
          <p class="alert" role="alert">
            {error()}
          </p>
        </Show>
        <div class="row" style={{ "justify-content": "flex-end" }}>
          <button type="button" class="btn" onClick={props.onClose}>
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" disabled={busy()}>
            Subscribe
          </button>
        </div>
      </form>
    </Modal>
  );
}

/** File input -> IcsService.import-ics, then the counts. */
export function ImportIcsForm(props: { calendarId?: string; onDone?: () => void }) {
  const cals = useCalendars();
  const writable = () => cals().filter((c) => c.kind === "local" && accessAtLeast(c.access, "write"));
  const [calId, setCalId] = createSignal(props.calendarId ?? writable().find((c) => c.isDefault)?.calendarId ?? writable()[0]?.calendarId ?? "");
  const [file, setFile] = createSignal<File | null>(null);
  const [result, setResult] = createSignal<ImportResult | null>(null);
  const [error, setError] = createSignal<string | null>(null);
  const [busy, setBusy] = createSignal(false);

  const submit = async (e: SubmitEvent) => {
    e.preventDefault();
    const f = file();
    if (!f) return setError("Choose an .ics file.");
    if (!calId()) return setError("Choose a calendar.");
    setBusy(true);
    setError(null);
    try {
      const text = await f.text();
      if (!/BEGIN:VCALENDAR/i.test(text)) throw new Error("That file is not an iCalendar file (no BEGIN:VCALENDAR).");
      const r = await icsClient.importIcs({ calendarId: calId(), ics: text });
      setResult(r);
      bumpEvents();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form class="stack" onSubmit={submit}>
      <Show when={!props.calendarId}>
        <div class="field">
          <label for="imp-cal">Into calendar</label>
          <select id="imp-cal" class="select" value={calId()} onChange={(e) => setCalId(e.currentTarget.value)}>
            <For each={writable()}>{(c) => <option value={c.calendarId}>{c.name}</option>}</For>
          </select>
        </div>
      </Show>
      <div class="field">
        <label for="imp-file">iCalendar file</label>
        <input id="imp-file" class="input input-file" type="file" accept=".ics,text/calendar" onChange={(e) => setFile(e.currentTarget.files?.[0] ?? null)} />
        <span class="hint">Events with a UID that is already in the calendar are updated, not copied.</span>
      </div>
      <Show when={error()}>
        <p class="alert" role="alert">
          {error()}
        </p>
      </Show>
      <Show when={result()}>
        {(r) => (
          <div class="notice" role="status">
            <p>
              <strong>{r().created}</strong> created, <strong>{r().updated}</strong> updated, <strong>{r().skipped}</strong> skipped.
            </p>
            <Show when={(r().warnings ?? []).length}>
              <ul class="warnings">
                <For each={r().warnings}>{(w) => <li>{w}</li>}</For>
              </ul>
            </Show>
          </div>
        )}
      </Show>
      <div class="row" style={{ "justify-content": "flex-end" }}>
        <Show when={props.onDone}>
          <button type="button" class="btn" onClick={() => props.onDone?.()}>
            {result() ? "Done" : "Cancel"}
          </button>
        </Show>
        <button type="submit" class="btn btn-primary" disabled={busy() || !file()}>
          {busy() ? "Importing…" : "Import"}
        </button>
      </div>
    </form>
  );
}

export function ImportDialog(props: { onClose: () => void }) {
  return (
    <Modal title="Import an .ics file" onClose={props.onClose}>
      <ImportIcsForm onDone={props.onClose} />
    </Modal>
  );
}
