/**
 * /auth/callback: LinkKeys sends the browser here with a sealed
 * `encrypted_token`. AuthService.complete opens it and returns our bearer.
 * `domain` is the identity domain that /api/v1/auth/start put on the
 * callback URL.
 */
import { authClient } from "~/data/clients";
import { finishLogin, takeAuthReturnTo } from "~/lib/session";
import { currentRoute, navigate } from "~/app/router";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { fill, h } from "~/ui/dom";

export class AuthCallback extends CalElement {
  #page!: HTMLElement;
  #started = false;

  protected build(): void {
    this.#page = h(
      "div",
      { class: "center-page" },
      h("div", { class: "center-card", role: "status" }, h("div", { class: "spinner", "aria-hidden": "true" }), h("p", null, "Signing you in…")),
    );
    this.append(this.#page);
  }

  protected connected(): void {
    if (this.#started) return;
    this.#started = true;
    void this.#complete();
  }

  async #complete(): Promise<void> {
    const query = currentRoute().query;
    const token = query.get("encrypted_token") ?? "";
    if (!token) {
      this.#fail(query.get("error") || "The identity provider did not send a sign-in token.");
      return;
    }
    try {
      const domain = query.get("domain") || undefined;
      const resp = await authClient.complete({ encryptedToken: token, domain });
      await finishLogin(resp);
      navigate(takeAuthReturnTo(), { replace: true });
    } catch (e) {
      if (this.isActive()) this.#fail(errorMessage(e));
    }
  }

  #fail(message: string): void {
    fill(
      this.#page,
      h(
        "div",
        { class: "center-card" },
        h("h1", null, "Sign-in did not finish"),
        h("p", { class: "alert", role: "alert" }, message),
        h("a", { class: "btn btn-primary auth-retry", href: "/login" }, "Try again"),
      ),
    );
  }
}
define("cal-auth-callback", AuthCallback);
