import type { Calendar, Delegation, Scope } from "@catalystcal/client";
import { delegationClient } from "~/data/clients";
import { SCOPES, normalizeScopes } from "~/lib/scopes";
import { formatIdentity, parseIdentity } from "~/lib/identity";
import { identityLabel } from "~/lib/people";
import { addDays, isDateText } from "~/lib/dates";
import { civilDate, parseTimestamp, toTimestamp, todayIn, zonedToUtc } from "~/lib/tz";
import { session } from "~/stores/auth";
import { displayZone } from "~/stores/prefs";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { h, setText, syncKeyed } from "~/ui/dom";
import type { CheckControl, PushButton, ValueControl } from "~/ui/controls";

/** End of `date` in `tz`, as the delegation expiry instant. */
export function expiryFromDate(date: string, tz: string): string {
  return toTimestamp(zonedToUtc(addDays(date, 1), 0, tz));
}
export function dateFromExpiry(ts: string, tz: string): string {
  return civilDate(parseTimestamp(ts) - 1000, tz);
}

/**
 * Add or change a delegation. Sends `delegation-saved` (detail: the saved
 * Delegation) after a save, and `delegation-cancel` on Cancel.
 */
export class DelegationForm extends CalElement {
  static properties = {
    calendars: { type: Array, attribute: false as const, default: () => [] },
    initial: { type: Object, attribute: false as const },
  };
  declare calendars: Calendar[];
  declare initial: Delegation | undefined;

  #form!: HTMLFormElement;
  #who!: ValueControl;
  #scopeBoxes = new Map<Scope, CheckControl>();
  #scopeHints = new Map<Scope, HTMLElement>();
  #restrict!: ValueControl;
  #calChecks!: HTMLElement;
  #calSection!: HTMLElement;
  #expires!: ValueControl;
  #note!: ValueControl;
  #error!: HTMLElement;
  #submit!: PushButton;
  #scopes: Scope[] = [];
  #calIds: string[] = [];
  #tz = displayZone();

  protected build(): void {
    const init = this.initial;
    this.#scopes = init ? normalizeScopes(init.scopes) : ["calendars.read", "freebusy.read"];
    this.#calIds = init?.calendarIds ?? [];
    this.#who = h("text-field", { label: "Delegate", placeholder: "user@domain", autocomplete: "off", helper: "Their LinkKeys handle or account id and domain, for example sam@example.com.", value: init ? formatIdentity(init.delegate) : "" }) as ValueControl;
    const scopeSet = h("fieldset", { class: "field scope-set" }, h("legend", { class: "field-label" }, "This person can"));
    for (const s of SCOPES) {
      const hint = h("span", { class: "hint", style: { display: "block" } }, s.detail);
      const box = h(
        "check-box",
        {
          name: "scope",
          value: s.scope,
          class: "scope-check",
          onChange: () => this.#toggleScope(s.scope, box.checked),
        },
        h("span", null, h("span", null, s.label), hint),
      ) as CheckControl;
      this.#scopeBoxes.set(s.scope, box);
      this.#scopeHints.set(s.scope, hint);
      scopeSet.append(box);
    }
    this.#restrict = h(
      "radio-group",
      { label: "Calendars", value: this.#calIds.length ? "only" : "all", onChange: () => this.render() },
      h("radio-button", { value: "all" }, "All my calendars, now and later"),
      h("radio-button", { value: "only" }, "Only these calendars"),
    ) as ValueControl;
    this.#calChecks = h("div", { class: "cal-checks" });
    this.#calSection = h("div", null, this.#calChecks, h("span", { class: "hint" }, "Invitations to you are not on a calendar. “Answer invitations as me” still applies to them."));
    this.#expires = h("date-time-picker", { mode: "date", label: "Ends on (optional)", min: todayIn(this.#tz), value: init?.expiresAt ? dateFromExpiry(init.expiresAt, this.#tz) : "" }) as ValueControl;
    this.#note = h("text-field", { label: "Note (optional)", placeholder: "Covering while I am on leave", value: init?.note ?? "" }) as ValueControl;
    this.#error = h("p", { class: "alert", role: "alert", hidden: true });
    this.#submit = h("push-button", { type: "submit", variant: "primary" }, init ? "Save changes" : "Add delegate") as PushButton;
    this.#form = h(
      "form",
      { class: "stack", "aria-label": init ? "Change delegate" : "Add a delegate", novalidate: true, onSubmit: (e: SubmitEvent) => void this.#save(e) },
      init ? h("div", { class: "field" }, h("span", { class: "field-label" }, "Delegate"), h("p", null, identityLabel(init.delegate, init.delegatePerson))) : this.#who,
      scopeSet,
      h("fieldset", { class: "field" }, this.#restrict, this.#calSection),
      h(
        "div",
        { class: "grid-2" },
        h("div", { class: "field" }, this.#expires, h("span", { class: "hint" }, "Access stops at the end of this day.")),
        h("div", { class: "field" }, this.#note),
      ),
      this.#error,
      h(
        "div",
        { class: "hrow", style: { "justify-content": "flex-end" } },
        h("push-button", { onClick: () => this.emit("delegation-cancel") }, "Cancel"),
        this.#submit,
      ),
    );
    this.append(this.#form);
  }

  render(): void {
    const details = this.#scopes.includes("calendars.read");
    for (const s of SCOPES) {
      const box = this.#scopeBoxes.get(s.scope)!;
      const implied = s.scope === "freebusy.read" && details;
      box.checked = this.#scopes.includes(s.scope);
      box.disabled = implied;
      setText(this.#scopeHints.get(s.scope)!, implied ? "Included in “See my calendars and event details”." : s.detail);
    }
    this.#calSection.hidden = this.#restrict.value !== "only";
    syncKeyed(
      this.#calChecks,
      this.calendars ?? [],
      (c) => c.calendarId,
      (c) => {
        const box = h(
          "check-box",
          { onChange: () => this.#toggleCal(c.calendarId, box.checked) },
          h("span", { class: "swatch", style: { "--sw": c.color || "var(--accent)" } }),
          c.name,
        ) as CheckControl;
        return box;
      },
      (el, c) => {
        (el as CheckControl).checked = this.#calIds.includes(c.calendarId);
      },
    );
  }

  #toggleScope(s: Scope, on: boolean): void {
    // Unchecking "details" leaves "busy times" checked: the smaller grant.
    this.#scopes = normalizeScopes(on ? [...this.#scopes, s] : this.#scopes.filter((x) => x !== s));
    this.render();
  }

  #toggleCal(id: string, on: boolean): void {
    this.#calIds = on ? [...this.#calIds, id] : this.#calIds.filter((x) => x !== id);
  }

  #fail(msg: string): void {
    this.#error.textContent = msg;
    this.#error.hidden = false;
  }

  async #save(e: SubmitEvent): Promise<void> {
    e.preventDefault();
    this.#error.hidden = true;
    const init = this.initial;
    const tz = this.#tz;
    const self = session()?.user;
    const delegate = init ? init.delegate : parseIdentity(this.#who.value, self?.domain);
    if (!delegate) return this.#fail("Enter the delegate as user@domain (their handle or account id at their LinkKeys domain).");
    if (self && delegate.domain === self.domain.toLowerCase() && (delegate.userId === self.userId || (delegate.handle && delegate.handle === self.handle))) {
      return this.#fail("You cannot make yourself your own delegate.");
    }
    if (this.#scopes.length === 0) return this.#fail("Choose at least one thing this person can do.");
    const restrict = this.#restrict.value === "only";
    if (restrict && this.#calIds.length === 0) return this.#fail("Choose at least one calendar, or allow all calendars.");
    const expires = this.#expires.value;
    if (expires && (!isDateText(expires) || expires < todayIn(tz))) return this.#fail("The end date must be today or later.");

    const body: Delegation = {
      delegationId: init?.delegationId ?? "",
      principal: init?.principal ?? self ?? { userId: "", domain: "", subject: "" },
      delegate,
      scopes: normalizeScopes(this.#scopes),
      calendarIds: restrict ? this.#calIds : [],
      note: this.#note.value.trim() || undefined,
      expiresAt: expires ? expiryFromDate(expires, tz) : undefined,
      createdAt: init?.createdAt ?? "",
    };
    this.#submit.disabled = true;
    try {
      const saved = init ? await delegationClient.updateDelegation(body) : await delegationClient.createDelegation(body);
      if (!this.isActive()) return;
      this.emit("delegation-saved", saved);
    } catch (err) {
      if (this.isActive()) this.#fail(errorMessage(err));
    } finally {
      this.#submit.disabled = false;
    }
  }
}
define("cal-delegation-form", DelegationForm);
