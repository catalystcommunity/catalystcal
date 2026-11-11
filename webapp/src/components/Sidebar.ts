import type { Calendar } from "@catalystcal/client";
import { openImportDialog, openNewCalendarDialog, openSubscribeDialog } from "~/components/CalendarDialogs";
import { AUTH, effectivePerson, hasScope, isActing } from "~/stores/auth";
import { CALENDARS, accessAtLeast, calendarColor, calendars, calendarsError, calendarsLoaded, hiddenCalendars, toggleHidden } from "~/stores/calendars";
import { NAV, cursor, goToDate, setSidebarOpen } from "~/stores/nav";
import { PREFS, displayZone, weekStart } from "~/stores/prefs";
import { groupCalendars } from "~/lib/calendarGroups";
import { ROUTE, currentRoute, navigate } from "~/app/router";
import { CalElement, define } from "~/ui/element";
import { h, setAttr, setText, syncKeyed } from "~/ui/dom";
import { icon, type IconName } from "~/ui/icons";
import type { ValueControl } from "~/ui/controls";

export const NAV_LINKS: { href: string; label: string; icon: IconName }[] = [
  { href: "/", label: "Calendar", icon: "calendar" },
  { href: "/invitations", label: "Invitations", icon: "inbox" },
  { href: "/delegates", label: "Delegates", icon: "users" },
  { href: "/activity", label: "Activity", icon: "activity" },
];

export class Sidebar extends CalElement {
  static watches = [AUTH, CALENDARS, NAV, PREFS, ROUTE];
  #links: HTMLAnchorElement[] = [];
  #picker!: ValueControl;
  #error!: HTMLElement;
  #groups!: HTMLElement;
  #newButton!: HTMLButtonElement;
  #subscribeButton!: HTMLButtonElement;
  #importButton!: HTMLButtonElement;

  protected build(): void {
    this.#links = NAV_LINKS.map((n) => h("a", { href: n.href, class: "side-link", onClick: () => setSidebarOpen(false) }, icon(n.icon, 16), n.label));
    this.#picker = h("date-time-picker", {
      inline: true,
      mode: "date",
      label: "Go to a date",
      clearable: "false",
      "show-now": "false",
      class: "side-picker",
    }) as ValueControl;
    this.#picker.addEventListener("date-changed", () => {
      const d = this.#picker.value;
      if (!d) return;
      goToDate(d);
      setSidebarOpen(false);
      if (currentRoute().name !== "calendar") navigate("/");
    });
    this.#error = h("p", { class: "alert small", role: "alert", hidden: true });
    this.#groups = h("div", { class: "cal-groups" });
    const action = (label: string, name: IconName, run: () => void) =>
      h("button", { type: "button", class: "side-action", onClick: run }, icon(name, 16), label);
    this.#newButton = action("New calendar", "plus", openNewCalendarDialog);
    this.#subscribeButton = action("Subscribe to URL", "link", openSubscribeDialog);
    this.#importButton = action("Import .ics", "upload", openImportDialog);
    this.append(
      h(
        "nav",
        { class: "sidebar", "aria-label": "Calendars and navigation" },
        h("div", { class: "sidebar-nav" }, this.#links),
        this.#picker,
        this.#error,
        this.#groups,
        h("div", { class: "side-actions" }, this.#newButton, this.#subscribeButton, this.#importButton),
      ),
    );
  }

  render(): void {
    const path = currentRoute().path;
    for (const a of this.#links) {
      const here = a.getAttribute("href") === path;
      a.classList.toggle("is-active", here);
      setAttr(a, "aria-current", here ? "page" : null);
    }
    setAttr(this.#picker, "first-day-of-week", weekStart());
    setAttr(this.#picker, "time-zone", displayZone());
    if (this.#picker.value !== cursor()) this.#picker.value = cursor();

    const error = calendarsError();
    this.#error.hidden = !error;
    setText(this.#error, error ? `Calendars did not load: ${error}` : "");

    const cals = calendars();
    const groups = groupCalendars(cals, effectivePerson(), isActing());
    syncKeyed(this.#groups, groups, (g) => g.key, (g) => this.#group(g.key), (el, g) => this.#updateGroup(el, g.label, g.calendars));

    const manage = !isActing() || hasScope("calendars.manage");
    this.#newButton.disabled = !manage;
    this.#subscribeButton.disabled = !manage;
    this.#importButton.disabled = !cals.some((c) => c.kind === "local" && accessAtLeast(c.access, "write"));
  }

  #group(key: string): HTMLElement {
    return h("section", { class: "cal-group", "aria-labelledby": `grp-${key}` }, h("h3", { id: `grp-${key}`, class: "cal-group-title" }), h("ul", { class: "cal-list" }));
  }

  #updateGroup(el: HTMLElement, label: string, list: Calendar[]): void {
    const title = el.querySelector("h3")!;
    setText(title, label);
    title.classList.toggle("is-acting", label.startsWith("Acting as"));
    const ul = el.querySelector("ul")!;
    const hidden = hiddenCalendars();
    const rows: (Calendar | null)[] = list.length === 0 && calendarsLoaded() ? [null] : list;
    syncKeyed(
      ul,
      rows,
      (c) => c?.calendarId ?? "",
      (c) => (c ? this.#row(c) : h("li", { class: "small muted cal-empty" }, "No calendars yet.")),
      (li, c) => {
        if (!c) return;
        const box = li.querySelector<HTMLInputElement>("input")!;
        box.checked = !hidden.has(c.calendarId);
        setAttr(box, "aria-label", `Show ${c.name}`);
        (li.querySelector(".cal-box") as HTMLElement).style.setProperty("--sw", calendarColor(c));
        setText(li.querySelector(".cal-name")!, c.name);
        const sync = li.querySelector<HTMLElement>(".pill")!;
        sync.hidden = !c.syncError;
        setAttr(sync, "title", c.syncError);
        const gear = li.querySelector("a")!;
        setAttr(gear, "href", `/calendars/${encodeURIComponent(c.calendarId)}`);
        setAttr(gear, "aria-label", `Settings for ${c.name}`);
      },
    );
  }

  #row(c: Calendar): HTMLElement {
    return h(
      "li",
      { class: "cal-item" },
      h(
        "label",
        { class: "cal-toggle" },
        h("input", { type: "checkbox", onChange: () => toggleHidden(c.calendarId) }),
        h("span", { class: "cal-box", "aria-hidden": "true" }, icon("check", 12)),
        h("span", { class: "cal-name truncate" }),
      ),
      h("span", { class: "pill pill-danger", hidden: true }, "sync"),
      h("a", { class: "icon-btn icon-btn-sm cal-gear", onClick: () => setSidebarOpen(false) }, icon("settings", 14)),
    );
  }
}
define("cal-sidebar", Sidebar);
