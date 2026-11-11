import type { AuditEntry } from "@catalystcal/client";
import { auditClient } from "~/data/clients";
import { AUTH, session } from "~/stores/auth";
import { displayZone } from "~/stores/prefs";
import { civilDate, formatDate, formatTime, parseTimestamp } from "~/lib/tz";
import { personName, samePerson } from "~/lib/people";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { fill, h, setText } from "~/ui/dom";
import type { CheckControl } from "~/ui/controls";

const VIA: Record<string, string> = { self: "Directly", delegation: "As a delegate", act_as_grant: "Through an app" };

/** "EventService/create-event" or "create-event" -> "Create event". */
export function operationLabel(op: string): string {
  const last = op.split(/[/.]/).pop() ?? op;
  const words = last.replace(/([a-z])([A-Z])/g, "$1 $2").replace(/[-_]/g, " ").toLowerCase();
  return words.charAt(0).toUpperCase() + words.slice(1);
}

export class ActivityPage extends CalElement {
  static watches = [AUTH];
  #onlyOthers!: CheckControl;
  #spinner!: HTMLElement;
  #error!: HTMLElement;
  #body!: HTMLTableSectionElement;
  #empty!: HTMLElement;
  /** The request key of the shown list; a new key loads again. */
  #key = "";
  #seq = 0;

  protected build(): void {
    this.#onlyOthers = h("check-box", { onChange: () => this.render() }, "Only actions others took as me") as CheckControl;
    this.#spinner = h("span", { class: "spinner spinner-sm", role: "status", "aria-label": "Loading", hidden: true });
    this.#error = h("p", { class: "alert", role: "alert", hidden: true });
    this.#body = h("tbody");
    this.#empty = h("p", { class: "empty", hidden: true });
    this.append(
      h(
        "div",
        { class: "page" },
        h(
          "header",
          { class: "page-head-text" },
          h("h1", null, "Activity"),
          h("p", { class: "lede" }, "Every change made in your name, and who made it. A delegate’s changes show their own name here."),
        ),
        h(
          "section",
          { class: "card" },
          h("div", { class: "row-between" }, this.#onlyOthers, this.#spinner),
          this.#error,
          h(
            "div",
            { class: "table-wrap" },
            h(
              "table",
              { class: "table activity-table" },
              h("thead", null, h("tr", null, ["When", "Who", "For", "How", "What", "Result"].map((t) => h("th", { scope: "col" }, t)))),
              this.#body,
            ),
          ),
          this.#empty,
        ),
      ),
    );
  }

  render(): void {
    const s = session();
    if (!s) return;
    const delegated = this.#onlyOthers.checked;
    const key = `${s.actingAs?.userId ?? s.user.userId}|${delegated}`;
    if (key === this.#key) return;
    this.#key = key;
    void this.#load(delegated);
  }

  async #load(delegated: boolean): Promise<void> {
    const my = ++this.#seq;
    this.#spinner.hidden = false;
    this.#empty.hidden = true;
    try {
      const r = await auditClient.listAudit({ limit: 200, delegatedOnly: delegated });
      if (my !== this.#seq || !this.isActive()) return;
      this.#error.hidden = true;
      this.#show(r.entries ?? [], delegated);
    } catch (e) {
      if (my !== this.#seq || !this.isActive()) return;
      setText(this.#error, `Activity did not load: ${errorMessage(e)}`);
      this.#error.hidden = false;
      fill(this.#body);
    } finally {
      if (my === this.#seq && this.isActive()) this.#spinner.hidden = true;
    }
  }

  #show(list: AuditEntry[], delegated: boolean): void {
    const tz = displayZone();
    const when = (e: AuditEntry) => {
      const ms = parseTimestamp(e.at);
      return `${formatDate(civilDate(ms, tz), { month: "short", day: "numeric", year: "numeric" })}, ${formatTime(ms, tz)}`;
    };
    fill(
      this.#body,
      list.map((e) => {
        const tr = h(
          "tr",
          null,
          h("td", { class: "tnum nowrap" }, when(e)),
          h("td", null, personName(e.actor)),
          h("td", null, samePerson(e.actor, e.principal) ? h("span", { class: "muted" }, "themselves") : personName(e.principal)),
          h(
            "td",
            null,
            h("span", { class: e.via !== "self" ? "pill pill-warn" : "pill" }, VIA[e.via] ?? e.via),
            e.application ? h("span", { class: "small muted" }, ` ${e.application}`) : null,
          ),
          h("td", null, operationLabel(e.operation), e.target ? h("span", { class: "small muted mono break" }, ` ${e.target}`) : null),
          h("td", null, h("span", { class: e.outcome === "ok" ? "pill pill-ok" : "pill pill-danger" }, e.outcome)),
        );
        tr.classList.toggle("is-delegated", !samePerson(e.actor, e.principal) || e.via !== "self");
        return tr;
      }),
    );
    setText(this.#empty, delegated ? "Nobody has acted as you." : "No activity yet.");
    this.#empty.hidden = list.length > 0;
  }
}
define("cal-activity-page", ActivityPage);
