/** Event search: a combobox over EventService.search-events. */
import type { Event } from "@catalystcal/client";
import { eventClient } from "~/data/clients";
import { calendarById, calendarColor } from "~/stores/calendars";
import { displayZone } from "~/stores/prefs";
import { goToDate } from "~/stores/nav";
import { openDetail } from "~/stores/editor";
import { eventDate, occurrenceOf } from "~/lib/eventRef";
import { formatDate, formatTime, parseTimestamp } from "~/lib/tz";
import { errorMessage } from "~/transport/csilrpc";
import { navigate } from "~/app/router";
import { CalElement, define } from "~/ui/element";
import { fill, h, setAttr, setText } from "~/ui/dom";
import { icon } from "~/ui/icons";

function whenText(e: Event): string {
  const tz = displayZone();
  const d = eventDate(e, tz);
  if (!d) return "";
  const day = formatDate(d, { weekday: "short", month: "short", day: "numeric", year: "numeric" });
  return e.allDay || !e.start ? day : `${day}, ${formatTime(parseTimestamp(e.start), tz)}`;
}

export class SearchBox extends CalElement {
  #input!: HTMLInputElement;
  #pop!: HTMLElement;
  #error!: HTMLElement;
  #list!: HTMLElement;
  #empty!: HTMLElement;
  #results: Event[] = [];
  #active = -1;
  #open = false;
  #loading = false;
  #problem: string | null = null;
  #timer: ReturnType<typeof setTimeout> | undefined;
  #seq = 0;

  protected build(): void {
    this.#input = h("input", {
      class: "search-input",
      type: "search",
      placeholder: "Search events",
      "aria-label": "Search events",
      role: "combobox",
      "aria-controls": "search-results",
      "aria-autocomplete": "list",
      autocomplete: "off",
      onInput: () => {
        this.#open = true;
        this.#run(this.#input.value);
        this.render();
      },
      onFocus: () => {
        this.#open = true;
        this.render();
      },
      onKeyDown: (e: KeyboardEvent) => this.#onKey(e),
    });
    this.#error = h("p", { class: "alert", role: "alert", hidden: true });
    this.#list = h("ul", { id: "search-results", role: "listbox", class: "search-list", "aria-label": "Search results" });
    this.#empty = h("p", { class: "empty small", hidden: true });
    this.#pop = h("div", { class: "search-pop", hidden: true }, this.#error, this.#list, this.#empty);
    const wrap = h("div", { class: "search" }, icon("search", 16, "search-icon"), this.#input, this.#pop);
    wrap.addEventListener("focusout", (e) => {
      if (!wrap.contains(e.relatedTarget as Node)) {
        this.#open = false;
        this.render();
      }
    });
    this.append(wrap);
  }

  protected connected(): void {
    this.own(() => clearTimeout(this.#timer));
  }

  render(): void {
    const q = this.#input.value.trim();
    const shown = this.#open && q !== "";
    this.#pop.hidden = !shown;
    setAttr(this.#input, "aria-expanded", String(shown));
    setAttr(this.#input, "aria-activedescendant", this.#active >= 0 ? `search-opt-${this.#active}` : null);
    this.#error.hidden = !this.#problem;
    setText(this.#error, this.#problem ?? "");
    fill(
      this.#list,
      this.#results.map((e, i) =>
        h(
          "li",
          {
            id: `search-opt-${i}`,
            role: "option",
            "aria-selected": String(this.#active === i),
            class: "search-opt",
            onMouseDown: (ev: MouseEvent) => {
              ev.preventDefault();
              this.#choose(e);
            },
            onMouseEnter: () => {
              this.#active = i;
              this.render();
            },
          },
          h("span", { class: "swatch", style: { "--sw": e.color || calendarColor(calendarById(e.calendarId)) } }),
          h("span", { class: "grow" }, h("span", { class: "truncate", style: { display: "block" } }, e.title), h("span", { class: "small muted" }, whenText(e))),
        ),
      ),
    );
    this.#empty.hidden = this.#loading || !!this.#problem || this.#results.length > 0;
    setText(this.#empty, `No events match “${q}”.`);
  }

  #run(text: string): void {
    clearTimeout(this.#timer);
    if (!text.trim()) {
      this.#results = [];
      this.#problem = null;
      return;
    }
    this.#timer = setTimeout(async () => {
      const seq = ++this.#seq;
      this.#loading = true;
      try {
        const r = await eventClient.searchEvents({ query: text.trim().slice(0, 256), limit: 20 });
        if (seq !== this.#seq || !this.isActive()) return;
        this.#results = r.events ?? [];
        this.#problem = null;
        this.#active = this.#results.length ? 0 : -1;
      } catch (e) {
        if (seq === this.#seq) this.#problem = errorMessage(e);
      } finally {
        if (seq === this.#seq) {
          this.#loading = false;
          if (this.isActive()) this.render();
        }
      }
    }, 220);
  }

  #choose(e: Event): void {
    const d = eventDate(e, displayZone());
    if (d) goToDate(d, "day");
    this.#open = false;
    this.#input.value = "";
    this.#results = [];
    this.#active = -1;
    navigate("/");
    openDetail(occurrenceOf(e));
    this.render();
  }

  #onKey(ev: KeyboardEvent): void {
    const list = this.#results;
    if (ev.key === "ArrowDown") {
      ev.preventDefault();
      this.#open = true;
      this.#active = Math.min(list.length - 1, this.#active + 1);
    } else if (ev.key === "ArrowUp") {
      ev.preventDefault();
      this.#active = Math.max(0, this.#active - 1);
    } else if (ev.key === "Enter") {
      const e = list[this.#active];
      if (e) {
        ev.preventDefault();
        this.#choose(e);
        return;
      }
    } else if (ev.key === "Escape") {
      this.#open = false;
      this.#input.blur();
    } else return;
    this.render();
  }
}
define("cal-search", SearchBox);
