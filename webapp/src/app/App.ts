/**
 * The root element. It shows sign-in, the sign-in callback, or the
 * signed-in shell, from the route and the session.
 */
import { AUTH, restoring, session } from "~/stores/auth";
import { rememberAuthReturnTo } from "~/lib/session";
import { ROUTE, currentRoute, navigate } from "~/app/router";
import { CalElement, define } from "~/ui/element";
import { h } from "~/ui/dom";
import { AppShell } from "~/app/AppShell";
import "~/pages/Login";
import "~/pages/AuthCallback";

type Mode = "login" | "callback" | "restoring" | "shell" | "redirect";

export class App extends CalElement {
  static watches = [AUTH, ROUTE];
  #mode: Mode | null = null;

  render(): void {
    const route = currentRoute();
    let mode: Mode;
    if (route.name === "callback") mode = "callback";
    else if (route.name === "login") mode = session() ? "redirect" : "login";
    else if (session()) mode = "shell";
    else if (restoring()) mode = "restoring";
    else mode = "redirect";

    if (mode === "redirect") {
      // Leave this render first: navigate changes the route, which renders again.
      if (session()) queueMicrotask(() => navigate("/", { replace: true }));
      else {
        rememberAuthReturnTo(route.path + (route.query.size ? `?${route.query}` : ""));
        queueMicrotask(() => navigate("/login", { replace: true }));
      }
      return;
    }
    if (mode === this.#mode) return;
    this.#mode = mode;
    if (mode === "login") this.replaceChildren(h("cal-login-page"));
    else if (mode === "callback") this.replaceChildren(h("cal-auth-callback"));
    else if (mode === "restoring")
      this.replaceChildren(h("div", { class: "center-page", role: "status" }, h("div", { class: "spinner", "aria-hidden": "true" }), h("p", null, "Signing you in…")));
    else this.replaceChildren(new AppShell());
  }
}
define("cal-app", App);
