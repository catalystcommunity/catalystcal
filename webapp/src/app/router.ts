/**
 * Routes over the Zest SimpleRouter. The current route goes into the store
 * (collection "route"), and the app element shows the page for it.
 */
import SimpleRouter from "@catalystcommunity/simple-router";
import { cell } from "~/state/store";

export type RouteName = "login" | "callback" | "calendar" | "calendarSettings" | "delegates" | "activity" | "invitations" | "settings" | "notFound";

export interface Route {
  name: RouteName;
  path: string;
  /** The query string of the URL. */
  query: URLSearchParams;
  /** `/calendars/:id`: the id. */
  id?: string;
}

/** The store collection of this module. */
export const ROUTE = "route";

const routeCell = cell<Route>(ROUTE, "current", { name: "notFound", path: "/", query: new URLSearchParams() });
export const currentRoute = (): Route => routeCell.get();

let router: SimpleRouter | null = null;

function setRoute(name: RouteName, id?: string): void {
  routeCell.set({ name, path: location.pathname, query: new URLSearchParams(location.search), id });
}

/** Register the routes, follow the browser history, and route the current URL. */
export function startRouter(): void {
  router?.cleanup();
  const r = new SimpleRouter();
  // The store tells the pages; no document event is needed.
  r.eventSender = () => {};
  r.registerRoute("/", () => setRoute("calendar"));
  r.registerRoute("/login", () => setRoute("login"));
  r.registerRoute("/auth/callback", () => setRoute("callback"));
  r.registerRoute("/calendars", (data) => {
    const id = decodeURIComponent(data.path.split("/")[2] ?? "");
    setRoute(id ? "calendarSettings" : "notFound", id || undefined);
  });
  r.registerRoute("/delegates", () => setRoute("delegates"));
  r.registerRoute("/activity", () => setRoute("activity"));
  r.registerRoute("/invitations", () => setRoute("invitations"));
  r.registerRoute("/settings", () => setRoute("settings"));
  r.registerRoute("/notfound", () => setRoute("notFound"), [], true);
  router = r;
  r.route("direct", {});
}

/** Go to an app path. `replace` changes the current history entry. */
export function navigate(path: string, opts: { replace?: boolean } = {}): void {
  if (!router) return;
  if (opts.replace) {
    history.replaceState({}, "", path);
    router.route("direct", {});
  } else if (path !== location.pathname + location.search) {
    router.changePage(path);
  }
}

/** True for an app path that a link click should route in the page. */
function isAppPath(url: URL): boolean {
  return url.origin === location.origin && !url.pathname.startsWith("/api/") && !url.pathname.startsWith("/ics/");
}

/** Route clicks on same-origin links in the page, as an app link does. */
export function routeLinks(root: EventTarget = document): () => void {
  const onClick = (e: Event) => {
    const ev = e as MouseEvent;
    if (ev.defaultPrevented || ev.button !== 0 || ev.metaKey || ev.ctrlKey || ev.shiftKey || ev.altKey) return;
    const a = (ev.composedPath()[0] as Element | null)?.closest?.("a[href]") as HTMLAnchorElement | null;
    if (!a || (a.target && a.target !== "_self") || a.hasAttribute("download")) return;
    const url = new URL(a.href, location.href);
    if (!isAppPath(url)) return;
    ev.preventDefault();
    navigate(url.pathname + url.search);
  };
  root.addEventListener("click", onClick);
  return () => root.removeEventListener("click", onClick);
}
