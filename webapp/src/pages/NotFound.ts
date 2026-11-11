import { CalElement, define } from "~/ui/element";
import { h } from "~/ui/dom";

export class NotFoundPage extends CalElement {
  protected build(): void {
    this.append(
      h(
        "div",
        { class: "page" },
        h("h1", null, "Page not found"),
        h("p", { class: "lede" }, "This address is not part of catalystcal. ", h("a", { href: "/" }, "Go to your calendar"), "."),
      ),
    );
  }
}
define("cal-not-found", NotFoundPage);
