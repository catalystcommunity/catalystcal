/**
 * The account button: a Zest menu-button with the avatar of the effective
 * person as its face. Its menu acts as someone, opens settings, or signs out.
 */
import type { PersonSummary } from "@catalystcal/client";
import { Action, ActionGroup } from "@catalystcommunity/actions";
import type { MenuButton, MenuItem } from "@catalystcommunity/menus";
import { AUTH, effectivePerson, isActing, me, session } from "~/stores/auth";
import { initials, personAddress, personName, samePerson } from "~/lib/people";
import { actAs, endSession, stopActing } from "~/lib/session";
import { errorMessage } from "~/transport/csilrpc";
import { toast } from "~/stores/toast";
import { navigate } from "~/app/router";
import { CalElement, define } from "~/ui/element";
import { h, setText } from "~/ui/dom";

export class AccountMenu extends CalElement {
  static watches = [AUTH];
  #button!: MenuButton;
  #avatar!: HTMLElement;

  protected build(): void {
    // The initials are aria-hidden: the label names the button.
    this.#avatar = h("span", { class: "avatar", "aria-hidden": "true" });
    // Names come from user data, so "&" is text, not a mnemonic.
    this.#button = h("menu-button", { class: "account-btn", placement: "bottom-end", mnemonics: "false" }, this.#avatar) as MenuButton;
    this.append(this.#button);
  }

  render(): void {
    const name = personName(effectivePerson());
    setText(this.#avatar, initials(name));
    this.#avatar.classList.toggle("avatar-acting", isActing());
    this.#button.label = `Account: ${name}`;
    this.#button.items = this.#items();
  }

  #items(): MenuItem[] {
    const s = session();
    const principals = (me()?.principals ?? []).filter((p) => !samePerson(p, s?.user));
    const group = new ActionGroup();
    const asWho: MenuItem[] = [
      new Action({ id: "as-self", label: "Use as yourself", checkable: true, checked: !isActing(), group, run: () => void this.#switchTo(null) }),
      ...principals.map(
        (p, i) =>
          new Action({
            id: `as-${i}`,
            label: `Act as ${personName(p)}`,
            checkable: true,
            checked: samePerson(p, s?.actingAs),
            group,
            run: () => void (samePerson(p, s?.actingAs) ? undefined : this.#switchTo(p)),
          }),
      ),
    ];
    if (principals.length === 0) asWho.push({ label: "Nobody has let you act for them yet", enabled: false });
    return [
      { label: `${personName(s?.user)} (${personAddress(s?.user)})`, enabled: false },
      { separator: true },
      ...asWho,
      { separator: true },
      { label: "Settings", run: () => navigate("/settings") },
      { label: "Delegates", run: () => navigate("/delegates") },
      { label: "Sign out", run: () => void this.#signOut() },
    ];
  }

  async #switchTo(p: PersonSummary | null): Promise<void> {
    try {
      if (isActing()) await stopActing();
      if (p) {
        await actAs(p);
        toast(`You are now acting as ${personName(p)}.`);
      }
      navigate("/");
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  }

  async #signOut(): Promise<void> {
    await endSession();
    navigate("/login", { replace: true });
  }
}
define("cal-account-menu", AccountMenu);
