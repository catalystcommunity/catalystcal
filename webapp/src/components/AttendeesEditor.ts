import type { Attendee, AttendeeRole } from "@catalystcal/client";
import { userClient } from "~/data/clients";
import { isEmail, parseIdentity, sameIdentity } from "~/lib/identity";
import { identityLabel, personName } from "~/lib/people";
import { STATUS_LABEL } from "~/lib/eventVisual";
import { effectivePerson } from "~/stores/auth";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { h, setText, syncKeyed } from "~/ui/dom";
import { icon } from "~/ui/icons";
import type { OptionControl, PushButton, ValueControl } from "~/ui/controls";

export function attendeeLabel(a: Attendee): string {
  return a.name || (a.identity ? identityLabel(a.identity, a.person) : (a.email ?? "Someone"));
}

function attendeeKey(a: Attendee): string {
  if (a.identity) return `id:${a.identity.domain.toLowerCase()}/${a.identity.userId ?? ""}/${a.identity.handle ?? ""}`;
  return `mail:${(a.email ?? "").toLowerCase()}`;
}

const ROLES = [
  { value: "required", label: "Required" },
  { value: "optional", label: "Optional" },
  { value: "chair", label: "Chair" },
  { value: "non_participant", label: "For information" },
];

/**
 * Add people by LinkKeys identity (user@domain or a handle) or by email.
 * A name that resolves through UserService.lookup-person is added as an
 * identity; otherwise an email-shaped entry is added as an email.
 * `value` is the attendee list. Sends `change` (target: this element).
 */
export class AttendeesEditor extends CalElement {
  static properties = {
    value: { type: Array, attribute: false as const, default: () => [] },
  };
  declare value: Attendee[];
  #text!: ValueControl;
  #add!: PushButton;
  #error!: HTMLElement;
  #list!: HTMLElement;
  #busy = false;

  protected build(): void {
    this.#text = h("text-field", {
      label: "People",
      placeholder: "user@domain or email",
      autocomplete: "off",
      class: "grow",
      onInput: () => this.#updateAdd(),
      onKeyDown: (e: KeyboardEvent) => {
        if (e.key === "Enter") {
          e.preventDefault();
          void this.#addPerson();
        }
      },
    }) as ValueControl;
    this.#add = h("push-button", { onClick: () => void this.#addPerson() }, "Add") as PushButton;
    this.#error = h("p", { class: "alert small", role: "alert", hidden: true });
    this.#list = h("ul", { class: "list attendee-list" });
    this.append(h("div", { class: "field" }, h("div", { class: "hrow attendee-add" }, this.#text, this.#add), this.#error, this.#list));
  }

  render(): void {
    this.#updateAdd();
    const list = this.value ?? [];
    this.#list.hidden = list.length === 0;
    syncKeyed(this.#list, list, attendeeKey, (a) => this.#row(a), (el, a) => this.#updateRow(el, a));
  }

  #updateAdd(): void {
    this.#add.disabled = this.#busy || !this.#text.value.trim();
  }

  #row(a: Attendee): HTMLElement {
    const key = attendeeKey(a);
    const role = h("combo-box", {
      class: "select-sm",
      ".options": ROLES,
      onChange: () => this.#emit(this.value.map((x) => (attendeeKey(x) === key ? { ...x, role: role.value as AttendeeRole } : x))),
    }) as OptionControl;
    return h(
      "li",
      null,
      h("span", { class: "grow" }, h("span", { class: "truncate attendee-name", style: { display: "block" } }), h("span", { class: "small muted attendee-sub" })),
      h("span", { class: "pill attendee-status" }),
      role,
      h(
        "button",
        { type: "button", class: "icon-btn icon-btn-sm attendee-remove", onClick: () => this.#emit(this.value.filter((x) => attendeeKey(x) !== key)) },
        icon("close", 14),
      ),
    );
  }

  #updateRow(el: HTMLElement, a: Attendee): void {
    const label = attendeeLabel(a);
    setText(el.querySelector(".attendee-name")!, label);
    const sub = a.identity ? (a.person ? `${a.identity.handle ?? a.person.subject}@${a.identity.domain}` : "catalystcal account") : "by email";
    setText(el.querySelector(".attendee-sub")!, a.respondedBy ? `${sub} · answered by ${personName(a.respondedBy)}` : sub);
    const pill = el.querySelector(".attendee-status")!;
    pill.classList.toggle("pill-ok", a.status === "accepted");
    pill.classList.toggle("pill-danger", a.status === "declined");
    pill.classList.toggle("pill-warn", a.status === "tentative");
    setText(pill, STATUS_LABEL[a.status ?? "needs_action"]);
    const role = el.querySelector("combo-box") as OptionControl;
    role.setAttribute("aria-label", `Role for ${label}`);
    if (role.value !== (a.role ?? "required")) role.value = a.role ?? "required";
    el.querySelector(".attendee-remove")!.setAttribute("aria-label", `Remove ${label}`);
  }

  #emit(next: Attendee[]): void {
    this.value = next;
    this.dispatchEvent(new Event("change", { bubbles: true }));
  }

  #showError(msg: string | null): void {
    this.#error.hidden = !msg;
    setText(this.#error, msg ?? "");
  }

  async #addPerson(): Promise<void> {
    const raw = this.#text.value.trim();
    if (!raw || this.#busy) return;
    this.#showError(null);
    const ref = parseIdentity(raw, effectivePerson()?.domain);
    if (!ref && !isEmail(raw)) return this.#showError("Enter someone as user@domain, a handle, or an email address.");
    const list = this.value ?? [];
    if (ref && list.some((a) => sameIdentity(a.identity, ref))) return this.#showError("That person is already invited.");
    if (!ref && list.some((a) => a.email?.toLowerCase() === raw.toLowerCase())) return this.#showError("That address is already invited.");
    this.#busy = true;
    this.#updateAdd();
    try {
      let next: Attendee | null = null;
      if (ref) {
        try {
          const person = await userClient.lookupPerson({ identity: ref });
          next = { identity: { domain: person.domain, userId: person.userId, handle: person.handle }, person, name: person.displayName, role: "required", status: "needs_action", rsvp: true };
        } catch (e) {
          if (!isEmail(raw)) throw e;
        }
      }
      if (!this.isActive()) return;
      if (!next) next = { email: raw, role: "required", status: "needs_action", rsvp: true };
      this.#emit([...(this.value ?? []), next]);
      this.#text.value = "";
    } catch (e) {
      if (this.isActive()) this.#showError(errorMessage(e));
    } finally {
      this.#busy = false;
      if (this.isActive()) this.#updateAdd();
    }
  }
}
define("cal-attendees-editor", AttendeesEditor);
