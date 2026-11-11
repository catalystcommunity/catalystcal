/**
 * "Find a time": busy blocks for you and each attendee with a catalystcal
 * identity, side by side for one day, with the proposed time drawn across.
 * A click on a column sends `free-busy-pick` with `{startMin}`.
 */
import type { Attendee, IdentityRef, PersonFreeBusy } from "@catalystcal/client";
import { freeBusyClient } from "~/data/clients";
import { effectivePerson } from "~/stores/auth";
import { DateText, addDays } from "~/lib/dates";
import { formatDate, formatHourLabel, minutesIntoDay, parseTimestamp, startOfDayUtc, toTimestamp } from "~/lib/tz";
import { personName } from "~/lib/people";
import { errorMessage } from "~/transport/csilrpc";
import { attendeeLabel } from "~/components/AttendeesEditor";
import { CalElement, define } from "~/ui/element";
import { h, fill, setText } from "~/ui/dom";
import { icon } from "~/ui/icons";

const FROM = 6;
const TO = 22;

const pct = (min: number) => `${((Math.min(TO * 60, Math.max(FROM * 60, min)) - FROM * 60) / ((TO - FROM) * 60)) * 100}%`;

export class FreeBusyPanel extends CalElement {
  static properties = {
    attendees: { type: Array, attribute: false as const, default: () => [] },
    date: { type: String, default: "" },
    endDate: { type: String, default: "" },
    startMin: { type: Number, default: 0 },
    endMin: { type: Number, default: 0 },
    tz: { type: String, default: "UTC" },
    allDay: { type: Boolean },
  };
  declare attendees: Attendee[];
  declare date: DateText;
  declare endDate: DateText;
  declare startMin: number;
  declare endMin: number;
  declare tz: string;
  declare allDay: boolean;

  #day = "";
  #shownDate = "";
  #key = "";
  #seq = 0;
  #data: PersonFreeBusy[] = [];
  #error: string | null = null;
  #dayLabel!: HTMLElement;
  #alert!: HTMLElement;
  #grid!: HTMLElement;
  #pickHint!: HTMLElement;

  protected build(): void {
    this.#dayLabel = h("span", { class: "small" });
    this.#alert = h("p", { class: "alert small", role: "alert", hidden: true });
    this.#grid = h("div", { class: "fb-grid" });
    this.#pickHint = h("span", null, " Click a column to move the event there.");
    this.append(
      h(
        "section",
        { class: "fb", "aria-labelledby": "fb-h" },
        h(
          "div",
          { class: "row-between" },
          h("h3", { id: "fb-h" }, "Find a time"),
          h(
            "div",
            { class: "hrow fb-nav" },
            h("button", { type: "button", class: "icon-btn icon-btn-sm", "aria-label": "Previous day", onClick: () => this.#move(-1) }, icon("chevronLeft", 14)),
            this.#dayLabel,
            h("button", { type: "button", class: "icon-btn icon-btn-sm", "aria-label": "Next day", onClick: () => this.#move(1) }, icon("chevronRight", 14)),
          ),
        ),
        this.#alert,
        this.#grid,
        h("p", { class: "hint" }, "Hatched columns are people whose time you cannot see.", this.#pickHint),
      ),
    );
  }

  #people(): { ref: IdentityRef; label: string }[] {
    const me = effectivePerson();
    const list: { ref: IdentityRef; label: string }[] = [];
    if (me) list.push({ ref: { domain: me.domain, userId: me.userId }, label: `${personName(me)} (you)` });
    for (const a of this.attendees ?? []) if (a.identity && (a.identity.userId || a.identity.handle)) list.push({ ref: a.identity, label: attendeeLabel(a) });
    return list;
  }

  #move(days: number): void {
    this.#day = addDays(this.#day || this.date, days);
    this.invalidate();
  }

  render(): void {
    // A new event date moves the panel to it.
    if (this.date !== this.#shownDate) {
      this.#shownDate = this.date;
      this.#day = this.date;
    }
    const day = this.#day;
    if (!day) return;
    setText(this.#dayLabel, formatDate(day, { weekday: "short", month: "short", day: "numeric" }));
    const people = this.#people();
    const key = JSON.stringify({ d: day, p: people.map((p) => p.ref), tz: this.tz });
    if (key !== this.#key) {
      this.#key = key;
      void this.#load(day, people.map((p) => p.ref));
    }
    this.#alert.hidden = !this.#error;
    setText(this.#alert, this.#error ? `Free and busy times did not load: ${this.#error}` : "");
    this.#pickHint.hidden = this.allDay;
    this.#drawGrid(day, people);
  }

  async #load(day: DateText, refs: IdentityRef[]): Promise<void> {
    const my = ++this.#seq;
    if (refs.length === 0) {
      this.#data = [];
      return;
    }
    try {
      const start = startOfDayUtc(day, this.tz);
      const end = startOfDayUtc(addDays(day, 1), this.tz);
      const r = await freeBusyClient.query({ people: refs, rangeStart: toTimestamp(start), rangeEnd: toTimestamp(end) });
      if (my !== this.#seq || !this.isActive()) return;
      this.#data = r.people ?? [];
      this.#error = null;
    } catch (e) {
      if (my !== this.#seq || !this.isActive()) return;
      this.#data = [];
      this.#error = errorMessage(e);
    }
    this.invalidate();
  }

  #blocks(day: DateText, pf: PersonFreeBusy | undefined) {
    if (!pf) return [];
    const ds = startOfDayUtc(day, this.tz);
    const de = startOfDayUtc(addDays(day, 1), this.tz);
    return pf.busy
      .map((b) => ({ s: Math.max(ds, parseTimestamp(b.start)), e: Math.min(de, parseTimestamp(b.end)), tentative: !!b.tentative }))
      .filter((b) => b.e > b.s)
      .map((b) => ({ top: b.s <= ds ? 0 : minutesIntoDay(b.s, this.tz), bottom: b.e >= de ? 1440 : minutesIntoDay(b.e, this.tz), tentative: b.tentative }));
  }

  #drawGrid(day: DateText, people: { ref: IdentityRef; label: string }[]): void {
    this.#grid.style.setProperty("--cols", String(people.length));
    const proposal = this.allDay || day !== this.date ? null : { top: this.startMin, bottom: this.endDate === this.date ? this.endMin : 1440 };
    const hours = h("div", { class: "fb-hours", "aria-hidden": "true" }, Array.from({ length: TO - FROM }, (_, i) => h("div", { class: "fb-hour tnum" }, formatHourLabel(FROM + i))));
    const cols = people.map((p, i) => {
      const result = this.#data[i];
      const hidden = result?.visible === false;
      const name = result?.person ? personName(result.person) : p.label;
      const blocks = this.#blocks(day, result);
      const track = h(
        "div",
        {
          class: "fb-track",
          role: "img",
          "aria-label": hidden ? `${p.label}: time not shared with you` : `${p.label}: ${blocks.length} busy blocks`,
          onClick: (e: MouseEvent) => {
            if (this.allDay || day !== this.date) return;
            const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
            const m = FROM * 60 + Math.round((((e.clientY - r.top) / (r.height || 1)) * (TO - FROM) * 60) / 30) * 30;
            this.emit("free-busy-pick", { startMin: m });
          },
        },
        hidden ? h("span", { class: "fb-hidden-label" }, "Time not shared") : null,
        blocks.map((b) => h("div", { class: b.tentative ? "fb-busy tentative" : "fb-busy", style: { top: pct(b.top), height: `calc(${pct(b.bottom)} - ${pct(b.top)})` } })),
        proposal ? h("div", { class: "fb-proposal", style: { top: pct(proposal.top), height: `calc(${pct(proposal.bottom)} - ${pct(proposal.top)})` } }) : null,
      );
      return h("div", { class: hidden ? "fb-col is-hidden" : "fb-col" }, h("div", { class: "fb-name truncate", title: name }, name), track);
    });
    fill(this.#grid, hours, cols);
  }
}
define("cal-free-busy-panel", FreeBusyPanel);
