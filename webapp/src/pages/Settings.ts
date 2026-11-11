import { userClient } from "~/data/clients";
import { AUTH, isActing, session } from "~/stores/auth";
import { type ThemeChoice, setTheme, theme } from "~/stores/theme";
import { PREFS, setWeekStart, weekStart } from "~/stores/prefs";
import { toast } from "~/stores/toast";
import { endSession, loadMe } from "~/lib/session";
import { browserZone, zoneList } from "~/lib/tz";
import { personAddress } from "~/lib/people";
import { navigate } from "~/app/router";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { h, setText } from "~/ui/dom";
import type { OptionControl, PushButton, ValueControl } from "~/ui/controls";

const THEMES: { v: ThemeChoice; l: string }[] = [
  { v: "light", l: "Light" },
  { v: "dark", l: "Dark" },
  { v: "system", l: "Match system" },
];

const zoneOptions = (current: string) => zoneList(current).map((z) => ({ value: z, label: z.replace(/_/g, " ") }));

export class SettingsPage extends CalElement {
  static watches = [AUTH, PREFS];
  #token: string | null = null;
  #seq = 0;
  #busy = false;
  #actingNotice!: HTMLElement;
  #profileError!: HTMLElement;
  #signedIn!: HTMLElement;
  #name!: ValueControl;
  #tz!: OptionControl;
  #error!: HTMLElement;
  #save!: PushButton;
  #theme!: ValueControl;
  #week!: ValueControl;

  protected build(): void {
    this.#actingNotice = h(
      "p",
      { class: "notice", hidden: true },
      "You are acting as someone else. A delegate cannot change their profile. These are your own settings: stop acting to change them.",
    );
    this.#profileError = h("p", { class: "alert", role: "alert", hidden: true });
    this.#signedIn = h("p", { class: "small muted" });
    this.#name = h("text-field", { label: "Display name" }) as ValueControl;
    this.#tz = h("combo-box", {
      label: "Time zone",
      editable: true,
      value: browserZone(),
      ".options": zoneOptions(browserZone()),
    }) as OptionControl;
    this.#error = h("p", { class: "alert", role: "alert", hidden: true });
    this.#save = h("push-button", { variant: "primary", onClick: () => void this.#saveProfile() }, "Save profile") as PushButton;
    this.#theme = h(
      "radio-group",
      { label: "Theme", orientation: "horizontal", onChange: () => setTheme(this.#theme.value as ThemeChoice) },
      THEMES.map((t) => h("radio-button", { value: t.v }, t.l)),
    ) as ValueControl;
    this.#week = h(
      "radio-group",
      { label: "Weeks start on", orientation: "horizontal", onChange: () => setWeekStart(this.#week.value === "1" ? 1 : 0) },
      h("radio-button", { value: "0" }, "Sunday"),
      h("radio-button", { value: "1" }, "Monday"),
    ) as ValueControl;

    const form = h(
      "form",
      {
        class: "stack",
        onSubmit: (e: Event) => {
          e.preventDefault();
          void this.#saveProfile();
        },
      },
      h("div", { class: "grid-2" }, h("div", { class: "field" }, this.#name), h("div", { class: "field" }, this.#tz, h("span", { class: "hint" }, `The calendar shows times in this zone. This device is in ${browserZone().replace(/_/g, " ")}.`))),
      this.#error,
      h("div", null, this.#save),
    );

    this.append(
      h(
        "div",
        { class: "page" },
        h("header", { class: "page-head-text" }, h("h1", null, "Settings")),
        h("section", { class: "card", "aria-labelledby": "prof-h" }, h("h2", { id: "prof-h" }, "Profile"), this.#actingNotice, this.#profileError, this.#signedIn, form),
        h(
          "section",
          { class: "card", "aria-labelledby": "look-h" },
          h("h2", { id: "look-h" }, "On this device"),
          h("div", { class: "field" }, this.#theme),
          h("div", { class: "field" }, this.#week),
          h("p", { class: "hint" }, `Keyboard: T today, ← → or J K to move, M W D A for views, C for a new event. Version ${__APP_VERSION__}.`),
        ),
        h(
          "section",
          { class: "card", "aria-labelledby": "out-h" },
          h("h2", { id: "out-h" }, "Sign out"),
          h("p", { class: "muted" }, "Ends your catalystcal session on this browser."),
          h(
            "div",
            null,
            h(
              "push-button",
              {
                variant: "danger",
                onClick: async () => {
                  await endSession();
                  navigate("/login", { replace: true });
                },
              },
              "Sign out",
            ),
          ),
        ),
      ),
    );
  }

  render(): void {
    const s = session();
    const acting = isActing();
    this.#actingNotice.hidden = !acting;
    setText(this.#signedIn, `Signed in as ${personAddress(s?.user)}`);
    this.#name.disabled = acting;
    this.#tz.disabled = acting;
    this.#save.disabled = this.#busy || acting;
    if (this.#theme.value !== theme()) this.#theme.value = theme();
    if (this.#week.value !== String(weekStart())) this.#week.value = String(weekStart());
    const token = s?.token ?? null;
    if (token && token !== this.#token) {
      this.#token = token;
      void this.#loadProfile();
    }
  }

  async #loadProfile(): Promise<void> {
    const my = ++this.#seq;
    try {
      const p = await userClient.getProfile({});
      if (my !== this.#seq || !this.isActive()) return;
      this.#profileError.hidden = true;
      this.#name.value = p.displayName ?? "";
      const zone = p.timezone || browserZone();
      this.#tz.options = zoneOptions(zone);
      this.#tz.value = zone;
    } catch (e) {
      if (my !== this.#seq || !this.isActive()) return;
      setText(this.#profileError, `Your profile did not load: ${errorMessage(e)}`);
      this.#profileError.hidden = false;
    }
  }

  async #saveProfile(): Promise<void> {
    if (this.#busy || isActing()) return;
    this.#busy = true;
    this.#error.hidden = true;
    this.render();
    try {
      await userClient.updateProfile({ displayName: this.#name.value.trim() || undefined, timezone: this.#tz.value });
      await loadMe();
      toast("Profile saved.");
    } catch (err) {
      if (this.isActive()) {
        setText(this.#error, errorMessage(err));
        this.#error.hidden = false;
      }
    } finally {
      this.#busy = false;
      if (this.isActive()) this.render();
    }
  }
}
define("cal-settings-page", SettingsPage);
