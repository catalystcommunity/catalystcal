import type { DevUser } from "@catalystcal/client";
import { devAuthClient } from "~/data/clients";
import { finishLogin, rememberedLoginName, setRememberedLoginName, startLinkKeysLogin, takeAuthReturnTo } from "~/lib/session";
import { currentRoute, navigate } from "~/app/router";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { fill, h, setText } from "~/ui/dom";
import type { CheckControl, PushButton, ValueControl } from "~/ui/controls";

/** The decorative week on the sign-in page. */
function weekArt(): HTMLElement {
  const ev = (style: Record<string, string>, extra = "") => h("div", { class: `art-ev ${extra}`.trim(), style });
  return h(
    "div",
    { class: "login-art", "aria-hidden": "true" },
    h(
      "div",
      { class: "art-grid" },
      h("div", { class: "art-col" }, ev({ "--ev": "#4f6bd8", top: "12%", height: "14%" }), ev({ "--ev": "#2f9e8f", top: "46%", height: "22%" })),
      h("div", { class: "art-col" }, ev({ top: "20%", height: "26%" }, "art-busy"), ev({ "--ev": "#d9822b", top: "58%", height: "10%" })),
      h(
        "div",
        { class: "art-col is-today" },
        ev({ "--ev": "#4f6bd8", top: "8%", height: "12%", width: "48%" }),
        ev({ "--ev": "#8a5cd6", top: "8%", height: "20%", left: "52%", width: "46%" }, "art-invite"),
        ev({ "--ev": "#c2415d", top: "62%", height: "16%" }),
        h("div", { class: "art-now", style: { top: "44%" } }),
      ),
      h("div", { class: "art-col" }, ev({ "--ev": "#2f9e8f", top: "30%", height: "32%" })),
      h("div", { class: "art-col" }, ev({ top: "14%", height: "12%" }, "art-busy"), ev({ "--ev": "#4f6bd8", top: "50%", height: "14%" })),
    ),
  );
}

/** Dev sign-in, shown only when the server answers DevAuthService. */
export class DevLoginPanel extends CalElement {
  #users: DevUser[] | null = null;
  #busy = false;
  #section!: HTMLElement;
  #list!: HTMLUListElement;
  #handle!: ValueControl;
  #submit!: PushButton;
  #error!: HTMLElement;

  protected build(): void {
    this.#list = h("ul", { class: "dev-users" });
    this.#handle = h("text-field", { label: "Handle", class: "grow", placeholder: "Handle for a new dev user", onInput: () => this.render() }) as ValueControl;
    const submitHandle = () => {
      const handle = this.#handle.value.trim();
      if (handle) void this.#login({ handle });
    };
    // A click on the button and Enter in the field both submit.
    this.#submit = h("push-button", { onClick: submitHandle }, "Sign in as dev user") as PushButton;
    this.#error = h("p", { class: "alert", role: "alert", hidden: true });
    this.#section = h(
      "section",
      { class: "dev-panel", "aria-labelledby": "dev-h" },
      h("h2", { id: "dev-h" }, "Development sign-in"),
      h("p", { class: "small muted" }, "This server runs with dev sign-in turned on. Do not use it with real data."),
      this.#list,
      h(
        "form",
        {
          class: "hrow dev-handle",
          onSubmit: (e: Event) => {
            e.preventDefault();
            submitHandle();
          },
        },
        this.#handle,
        this.#submit,
      ),
      this.#error,
    );
  }

  protected connected(): void {
    void this.#load();
  }

  async #load(): Promise<void> {
    try {
      const r = await devAuthClient.listDevUsers({});
      this.#users = r.users ?? [];
    } catch {
      this.#users = null; // No dev service on this server.
    }
    if (!this.isActive() || this.#users === null) return;
    // Only a server with dev sign-in gets the panel.
    if (!this.#section.isConnected) this.append(this.#section);
    fill(
      this.#list,
      (this.#users ?? []).map((u) =>
        h(
          "li",
          null,
          h(
            "push-button",
            { class: "dev-user", onClick: () => void this.#login({ userId: u.userId }) },
            h("span", { class: "grow truncate" }, u.displayName || u.handle || u.userId),
            h("span", { class: "small muted" }, ` ${u.handle ?? u.userId}@${u.domain}`),
          ),
        ),
      ),
    );
    this.render();
  }

  render(): void {
    this.#list.hidden = !this.#users?.length;
    this.#submit.disabled = this.#busy || !this.#handle.value?.trim();
    for (const b of this.#list.querySelectorAll<PushButton>("push-button")) b.disabled = this.#busy;
  }

  async #login(req: { userId?: string; handle?: string }): Promise<void> {
    this.#busy = true;
    this.#error.hidden = true;
    this.render();
    try {
      const resp = await devAuthClient.devLogin(req);
      await finishLogin(resp);
      navigate(takeAuthReturnTo(), { replace: true });
    } catch (e) {
      if (!this.isActive()) return;
      setText(this.#error, errorMessage(e));
      this.#error.hidden = false;
    } finally {
      this.#busy = false;
      if (this.isActive()) this.render();
    }
  }
}
define("cal-dev-login", DevLoginPanel);

export class LoginPage extends CalElement {
  #name!: ValueControl;
  #remember!: CheckControl;
  #submit!: PushButton;

  protected build(): void {
    const query = currentRoute().query;
    const saved = rememberedLoginName();
    const initial = query.get("identity") || saved;
    const loginError = query.get("login_error") ?? "";
    this.#name = h("text-field", {
      label: "LinkKeys login name",
      name: "identity",
      autocomplete: "username",
      placeholder: "you@example.com",
      value: initial,
      onInput: () => this.render(),
    }) as ValueControl;
    this.#remember = h("check-box", { class: "small login-remember", checked: saved !== "" }, "Remember my login name") as CheckControl;
    const submit = () => {
      const value = this.#name.value.trim();
      if (!value) return;
      setRememberedLoginName(value, this.#remember.checked);
      startLinkKeysLogin(value);
    };
    // A click on the button and Enter in the field both submit.
    this.#submit = h("push-button", { variant: "primary", class: "login-btn", onClick: submit }, "Sign in with LinkKeys") as PushButton;
    const form = h(
      "form",
      {
        class: "login-form",
        onSubmit: (e: Event) => {
          e.preventDefault();
          submit();
        },
      },
      this.#name,
      this.#remember,
      loginError ? h("p", { class: "alert", role: "alert" }, loginError) : null,
      this.#submit,
    );
    this.append(
      h(
        "div",
        { class: "login" },
        weekArt(),
        h(
          "main",
          { class: "login-panel" },
          h(
            "div",
            { class: "login-brand" },
            h("svg", { class: "brand-mark", viewBox: "0 0 24 24", "aria-hidden": "true" }, h("rect", { x: "2.5", y: "4", width: "19", height: "17", rx: "4" }), h("circle", { cx: "15.5", cy: "14.5", r: "2.4" })),
            "CatalystCal",
          ),
          h("h1", { class: "login-title" }, "Your calendar, and the people you let into it."),
          h(
            "p",
            { class: "login-lede" },
            "Share a calendar so someone can work with it as themselves, or let a person act for you. Every change made in your name shows in your activity.",
          ),
          form,
          h("p", { class: "small muted" }, "You sign in at your own LinkKeys domain. CatalystCal does not see your password."),
          h("cal-dev-login"),
        ),
      ),
    );
  }

  render(): void {
    this.#submit.disabled = !this.#name.value?.trim();
  }
}
define("cal-login-page", LoginPage);
