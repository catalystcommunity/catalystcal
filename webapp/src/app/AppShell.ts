/**
 * The signed-in frame: top bar, acting banner, sidebar, the page of the
 * route, and the event editor and detail dialogs.
 */
import { AUTH, isActing } from "~/stores/auth";
import { NAV, setSidebarOpen, sidebarOpen } from "~/stores/nav";
import { EDITOR, detailOccurrence, editorState } from "~/stores/editor";
import { ROUTE, currentRoute, type RouteName } from "~/app/router";
import { CalElement, define } from "~/ui/element";
import { h, setAttr } from "~/ui/dom";
import { icon } from "~/ui/icons";
import "~/components/ActingBanner";
import "~/components/Sidebar";
import "~/components/SearchBox";
import "~/components/AccountMenu";
import "~/components/EventEditor";
import "~/components/EventDetail";
import "~/pages/CalendarPage";
import "~/pages/CalendarSettings";
import "~/pages/Delegates";
import "~/pages/Activity";
import "~/pages/Invitations";
import "~/pages/Settings";
import "~/pages/NotFound";

const PAGE_TAGS: Record<Exclude<RouteName, "login" | "callback">, string> = {
  calendar: "cal-calendar-page",
  calendarSettings: "cal-calendar-settings",
  delegates: "cal-delegates-page",
  activity: "cal-activity-page",
  invitations: "cal-invitations-page",
  settings: "cal-settings-page",
  notFound: "cal-not-found",
};

interface EditorElement extends HTMLElement {
  state: unknown;
}
interface DetailElement extends HTMLElement {
  occ: unknown;
}

export class AppShell extends CalElement {
  static watches = [AUTH, NAV, EDITOR, ROUTE];
  #frame!: HTMLElement;
  #drawerButton!: HTMLButtonElement;
  #sidebarWrap!: HTMLElement;
  #scrim!: HTMLElement;
  #main!: HTMLElement;
  #pageKey = "";
  #editorFor: unknown = null;
  #editor: EditorElement | null = null;
  #detailFor: unknown = null;
  #detail: DetailElement | null = null;

  protected build(): void {
    this.#drawerButton = h(
      "button",
      { type: "button", class: "icon-btn drawer-btn", "aria-controls": "sidebar", onClick: () => setSidebarOpen(!sidebarOpen()) },
      icon("menu"),
    );
    this.#sidebarWrap = h("div", { id: "sidebar", class: "sidebar-wrap" }, h("cal-sidebar"));
    this.#scrim = h("div", { class: "drawer-scrim", "aria-hidden": "true", hidden: true, onClick: () => setSidebarOpen(false) });
    this.#main = h("main", { id: "main", class: "main", tabindex: "-1" });
    this.#frame = h(
      "div",
      { class: "app" },
      h(
        "a",
        {
          class: "skip-link",
          href: "#main",
          // The router does not see a fragment link; move the focus here.
          onClick: (e: Event) => {
            e.preventDefault();
            this.#main.focus();
          },
        },
        "Skip to content",
      ),
      h("cal-acting-banner"),
      h(
        "header",
        { class: "topbar" },
        this.#drawerButton,
        h(
          "a",
          { href: "/", class: "brand", "aria-label": "CatalystCal, calendar" },
          h("svg", { class: "brand-mark", viewBox: "0 0 24 24", "aria-hidden": "true" }, h("rect", { x: "2.5", y: "4", width: "19", height: "17", rx: "4" }), h("circle", { cx: "15.5", cy: "14.5", r: "2.4" })),
          h("span", null, "CatalystCal"),
        ),
        h("cal-search"),
        h("div", { class: "topbar-end" }, h("cal-account-menu")),
      ),
      h("div", { class: "body" }, this.#sidebarWrap, this.#scrim, this.#main),
    );
    this.append(this.#frame);
  }

  protected connected(): void {
    this.listen(document, "keydown", (e) => {
      if ((e as KeyboardEvent).key === "Escape" && sidebarOpen()) setSidebarOpen(false);
    });
  }

  render(): void {
    const open = sidebarOpen();
    this.#frame.classList.toggle("is-acting", isActing());
    this.#frame.classList.toggle("drawer-open", open);
    this.#sidebarWrap.classList.toggle("open", open);
    this.#scrim.hidden = !open;
    setAttr(this.#drawerButton, "aria-label", open ? "Close menu" : "Open menu");
    setAttr(this.#drawerButton, "aria-expanded", String(open));
    this.#renderPage();
    this.#renderDialogs();
  }

  #renderPage(): void {
    const route = currentRoute();
    if (route.name === "login" || route.name === "callback") return;
    const key = `${route.name}|${route.id ?? ""}`;
    if (key === this.#pageKey) return;
    // A new page closes the drawer.
    if (this.#pageKey) setSidebarOpen(false);
    this.#pageKey = key;
    const page = h(PAGE_TAGS[route.name]);
    if (route.id) (page as HTMLElement & { calendarId?: string }).calendarId = route.id;
    this.#main.replaceChildren(page);
  }

  #renderDialogs(): void {
    const ed = editorState();
    if (ed !== this.#editorFor) {
      this.#editorFor = ed;
      this.#editor?.remove();
      this.#editor = null;
      if (ed) {
        this.#editor = h("cal-event-editor") as EditorElement;
        this.#editor.state = ed;
        this.append(this.#editor);
      }
    }
    const occ = detailOccurrence();
    if (occ !== this.#detailFor) {
      this.#detailFor = occ;
      this.#detail?.remove();
      this.#detail = null;
      if (occ) {
        this.#detail = h("cal-event-detail") as DetailElement;
        this.#detail.occ = occ;
        this.append(this.#detail);
      }
    }
  }
}
define("cal-app-shell", AppShell);
