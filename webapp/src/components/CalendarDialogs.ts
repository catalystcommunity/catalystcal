/** New calendar, subscribe to a URL, and import .ics dialogs. */
import type { ImportResult } from "@catalystcal/client";
import { calendarClient, icsClient } from "~/data/clients";
import { SWATCHES, accessAtLeast, bumpEvents, calendars, upsertCalendar } from "~/stores/calendars";
import { displayZone } from "~/stores/prefs";
import { zoneList } from "~/lib/tz";
import { errorMessage } from "~/transport/csilrpc";
import { toast } from "~/stores/toast";
import { CalElement, define } from "~/ui/element";
import { fill, h } from "~/ui/dom";
import { openModal, type OptionControl, type PushButton, type ValueControl } from "~/ui/controls";
import type { ColorPicker } from "~/components/ColorPicker";
import "~/components/ColorPicker";

/** Time zone options, with "_" shown as spaces. */
export function zoneOptions(current: string): { value: string; label: string }[] {
  return zoneList(current).map((z) => ({ value: z, label: z.replace(/_/g, " ") }));
}

function alertBox(): HTMLElement {
  return h("p", { class: "alert", role: "alert", hidden: true });
}
function showError(el: HTMLElement, msg: string | null): void {
  el.hidden = !msg;
  el.textContent = msg ?? "";
}

/**
 * A modal with a form and a submit push-button. `submit` returns true to
 * close the dialog.
 */
function formModal(label: string, okLabel: string, fields: Node[], submit: (error: HTMLElement) => Promise<boolean>): void {
  const error = alertBox();
  const ok = h("push-button", { variant: "primary", "data-default": true }, okLabel) as PushButton;
  const cancel = h("button", { type: "button", value: "cancel" }, "Cancel");
  let dialog: { close(v?: string): void } | null = null;
  const run = async () => {
    if (ok.disabled) return;
    ok.disabled = true;
    try {
      if (await submit(error)) dialog?.close("ok");
    } finally {
      ok.disabled = false;
    }
  };
  const form = h(
    "form",
    {
      class: "stack",
      novalidate: true,
      onSubmit: (e: SubmitEvent) => {
        e.preventDefault();
        void run();
      },
    },
    fields,
    error,
  );
  ok.addEventListener("click", () => void run());
  dialog = openModal({ label, children: [form], buttons: [ok, cancel] }).dialog;
}

export function openNewCalendarDialog(): void {
  const name = h("text-field", { label: "Name", maxlength: "256", required: true, autofocus: true }) as ValueControl;
  const color = h("cal-color-picker", { label: "Calendar color", value: SWATCHES[1] }) as ColorPicker;
  const tz = h("combo-box", { label: "Time zone", editable: true, value: displayZone() }) as OptionControl;
  tz.options = zoneOptions(displayZone());
  const description = h("text-area", { label: "Description" }) as ValueControl;
  formModal("New calendar", "Create calendar", [name, h("div", { class: "field" }, h("span", { class: "field-label" }, "Color"), color), tz, description], async (error) => {
    if (!name.value.trim()) {
      showError(error, "Give the calendar a name.");
      return false;
    }
    try {
      const c = await calendarClient.createCalendar({ name: name.value.trim(), color: color.value, timezone: tz.value, description: description.value.trim() || undefined });
      upsertCalendar(c);
      toast(`Created “${c.name}”.`);
      return true;
    } catch (err) {
      showError(error, errorMessage(err));
      return false;
    }
  });
}

export function openSubscribeDialog(): void {
  const url = h("text-field", {
    label: "Calendar address",
    type: "url",
    inputmode: "url",
    placeholder: "https://example.com/team.ics",
    required: true,
    autofocus: true,
    helper: "An iCalendar (.ics) address. catalystcal reads it again from time to time; you cannot change its events.",
  }) as ValueControl;
  const name = h("text-field", { label: "Name", maxlength: "256", placeholder: "Uses the address host if empty" }) as ValueControl;
  const color = h("cal-color-picker", { label: "Calendar color", value: SWATCHES[7] }) as ColorPicker;
  formModal("Subscribe to a calendar", "Subscribe", [url, name, h("div", { class: "field" }, h("span", { class: "field-label" }, "Color"), color)], async (error) => {
    let src = url.value.trim();
    if (src.startsWith("webcal://")) src = "https://" + src.slice("webcal://".length);
    if (!/^https?:\/\//i.test(src)) {
      showError(error, "Enter an http, https, or webcal address.");
      return false;
    }
    const fallback = (() => {
      try {
        return new URL(src).hostname;
      } catch {
        return "Subscription";
      }
    })();
    try {
      const c = await calendarClient.subscribeCalendar({ name: name.value.trim() || fallback, sourceUrl: src, color: color.value });
      upsertCalendar(c);
      bumpEvents();
      toast(`Subscribed to “${c.name}”.`);
      return true;
    } catch (err) {
      showError(error, errorMessage(err));
      return false;
    }
  });
}

/**
 * File input -> IcsService.import-ics, then the counts. With no
 * `calendarId`, it shows a calendar box. With `done-button`, it has a
 * Cancel/Done button that sends `import-ics-done`.
 */
export class ImportIcsForm extends CalElement {
  static properties = {
    calendarId: { type: String, default: "" },
    doneButton: { type: Boolean },
  };
  declare calendarId: string;
  declare doneButton: boolean;
  #cal!: OptionControl;
  #calField!: HTMLElement;
  #file!: HTMLInputElement;
  #error!: HTMLElement;
  #result!: HTMLElement;
  #done!: PushButton;
  #doneText!: HTMLElement;
  #submit!: PushButton;
  #submitText!: HTMLElement;
  #busy = false;
  #imported = false;

  protected build(): void {
    this.#cal = h("combo-box", { label: "Into calendar" }) as OptionControl;
    this.#calField = h("div", { class: "field" }, this.#cal);
    this.#file = h("input", { id: "imp-file", class: "input input-file", type: "file", accept: ".ics,text/calendar", onChange: () => this.render() });
    this.#error = alertBox();
    this.#result = h("div", { class: "notice", role: "status", hidden: true });
    // A push-button moves its children into its own button: change the span, not the host.
    this.#doneText = h("span", null, "Cancel");
    this.#done = h("push-button", { onClick: () => this.emit("import-ics-done") }, this.#doneText) as PushButton;
    this.#submitText = h("span", null, "Import");
    this.#submit = h("push-button", { type: "submit", variant: "primary" }, this.#submitText) as PushButton;
    this.append(
      h(
        "form",
        { class: "stack", onSubmit: (e: SubmitEvent) => void this.#import(e) },
        this.#calField,
        h(
          "div",
          { class: "field" },
          h("label", { for: "imp-file" }, "iCalendar file"),
          this.#file,
          h("span", { class: "hint" }, "Events with a UID that is already in the calendar are updated, not copied."),
        ),
        this.#error,
        this.#result,
        h("div", { class: "hrow", style: { "justify-content": "flex-end" } }, this.#done, this.#submit),
      ),
    );
    const writable = calendars().filter((c) => c.kind === "local" && accessAtLeast(c.access, "write"));
    this.#cal.options = writable.map((c) => ({ value: c.calendarId, label: c.name }));
    this.#cal.value = writable.find((c) => c.isDefault)?.calendarId ?? writable[0]?.calendarId ?? "";
  }

  render(): void {
    this.#calField.hidden = !!this.calendarId;
    this.#done.hidden = !this.doneButton;
    this.#doneText.textContent = this.#imported ? "Done" : "Cancel";
    this.#submit.disabled = this.#busy || !this.#file.files?.length;
    this.#submitText.textContent = this.#busy ? "Importing…" : "Import";
  }

  async #import(e: SubmitEvent): Promise<void> {
    e.preventDefault();
    const f = this.#file.files?.[0];
    const calId = this.calendarId || this.#cal.value;
    if (!f) return showError(this.#error, "Choose an .ics file.");
    if (!calId) return showError(this.#error, "Choose a calendar.");
    this.#busy = true;
    showError(this.#error, null);
    this.render();
    try {
      const text = await f.text();
      if (!/BEGIN:VCALENDAR/i.test(text)) throw new Error("That file is not an iCalendar file (no BEGIN:VCALENDAR).");
      const r = await icsClient.importIcs({ calendarId: calId, ics: text });
      if (!this.isActive()) return;
      this.#showResult(r);
      this.#imported = true;
      bumpEvents();
    } catch (err) {
      if (this.isActive()) showError(this.#error, errorMessage(err));
    } finally {
      this.#busy = false;
      if (this.isActive()) this.render();
    }
  }

  #showResult(r: ImportResult): void {
    this.#result.hidden = false;
    fill(
      this.#result,
      h("p", null, h("strong", null, String(r.created)), " created, ", h("strong", null, String(r.updated)), " updated, ", h("strong", null, String(r.skipped)), " skipped."),
      (r.warnings ?? []).length ? h("ul", { class: "warnings" }, (r.warnings ?? []).map((w) => h("li", null, w))) : null,
    );
  }
}
define("cal-import-ics-form", ImportIcsForm);

export function openImportDialog(): void {
  const form = h("cal-import-ics-form", { "done-button": true });
  const { dialog } = openModal({ label: "Import an .ics file", children: [form] });
  form.addEventListener("import-ics-done", () => dialog.close("done"));
}
