import { SWATCHES } from "~/stores/calendars";
import { CalElement, define } from "~/ui/element";
import { h, setAttr } from "~/ui/dom";

/**
 * Swatch radio group. `allowNone` adds "use the calendar color" (value "").
 * Sends `change` when the user picks a color; `value` is the color.
 */
export class ColorPicker extends CalElement {
  static properties = {
    value: { type: String, default: "" },
    label: { type: String, default: "Color" },
    allowNone: { type: Boolean },
  };
  declare value: string;
  declare label: string;
  declare allowNone: boolean;
  #group!: HTMLElement;
  #none!: HTMLButtonElement;
  #custom!: HTMLInputElement;

  protected build(): void {
    this.#none = h("button", {
      type: "button",
      role: "radio",
      class: "swatch-btn swatch-none",
      "aria-label": "Calendar color",
      title: "Calendar color",
      onClick: () => this.#pick(""),
    });
    this.#custom = h("input", { type: "color", onInput: () => this.#pick(this.#custom.value) });
    this.#group = h(
      "div",
      { class: "swatches", role: "radiogroup" },
      this.#none,
      SWATCHES.map((c) =>
        h("button", { type: "button", role: "radio", class: "swatch-btn", style: { "--sw": c }, "aria-label": c, title: c, "data-color": c, onClick: () => this.#pick(c) }),
      ),
      h("label", { class: "swatch-custom", title: "Pick any color" }, h("span", { class: "sr-only" }, "Any color"), this.#custom),
    );
    this.append(this.#group);
  }

  render(): void {
    setAttr(this.#group, "aria-label", this.label);
    this.#none.hidden = !this.allowNone;
    setAttr(this.#none, "aria-checked", String(!this.value));
    const v = (this.value ?? "").toLowerCase();
    for (const b of this.#group.querySelectorAll<HTMLButtonElement>("[data-color]")) setAttr(b, "aria-checked", String(b.dataset.color === v));
    this.#custom.value = this.value || SWATCHES[0];
  }

  #pick(color: string): void {
    this.value = color;
    this.dispatchEvent(new Event("change", { bubbles: true }));
  }
}
define("cal-color-picker", ColorPicker);
