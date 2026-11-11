import { AUTH, isActing, session } from "~/stores/auth";
import { personName } from "~/lib/people";
import { stopActing } from "~/lib/session";
import { errorMessage } from "~/transport/csilrpc";
import { toast } from "~/stores/toast";
import { CalElement, define } from "~/ui/element";
import { h, setText } from "~/ui/dom";
import type { PushButton } from "~/ui/controls";

/** Shown on every page while a delegated bearer is in use. */
export class ActingBanner extends CalElement {
  static watches = [AUTH];
  #name!: HTMLElement;
  #stop!: PushButton;

  protected build(): void {
    this.#name = h("strong");
    this.#stop = h("push-button", { class: "acting-stop", onClick: () => void this.#stopActing() }, "Stop acting") as PushButton;
    this.append(
      h(
        "section",
        { class: "acting-banner", role: "region", "aria-label": "Acting as another person" },
        h("div", { class: "acting-text" }, this.#name, h("span", null, " — changes are made in their name, and they can see what you did")),
        this.#stop,
      ),
    );
  }

  render(): void {
    this.hidden = !isActing();
    setText(this.#name, `Acting as ${personName(session()?.actingAs)}`);
  }

  async #stopActing(): Promise<void> {
    this.#stop.disabled = true;
    try {
      await stopActing();
      toast("You are acting as yourself again.");
    } catch (e) {
      toast(errorMessage(e), "error");
    } finally {
      this.#stop.disabled = false;
    }
  }
}
define("cal-acting-banner", ActingBanner);
