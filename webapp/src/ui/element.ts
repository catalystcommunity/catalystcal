/**
 * The base class of every catalystcal element. It is a Zest BaseComponent
 * in the light DOM, so the app styles apply.
 *
 * - `build()` makes the elements one time, at the first connect.
 * - `render()` changes the parts that changed. It runs after each connect,
 *   and after each change of a watched store collection.
 * - `connected()` adds the subscriptions of one connect (`own`, `listen`).
 */
import { BaseComponent } from "@catalystcommunity/base-components";
import { store } from "~/state/store";

export class CalElement extends BaseComponent {
  /** Store collections whose changes render this element again. */
  static watches: readonly string[] = [];
  #built = false;

  connectedCallback(): void {
    super.connectedCallback();
    if (!this.#built) {
      this.#built = true;
      this.build();
    }
    for (const collection of (this.constructor as typeof CalElement).watches) {
      this.own(store.subscribe(() => this.invalidate(), { collection, isActive: () => this.isActive() }));
    }
    this.connected();
    this.render();
  }

  /** Make the elements. Runs one time. */
  protected build(): void {}

  /** Add listeners and subscriptions. Runs at each connect. */
  protected connected(): void {}

  render(): void {}
}

/** Define a tag one time. */
export function define(tag: string, cls: CustomElementConstructor): void {
  if (!customElements.get(tag)) customElements.define(tag, cls);
}

/** Make an element of a defined tag, and set its properties. */
export function make<T extends HTMLElement>(tag: string, props: Partial<T> = {}): T {
  const el = document.createElement(tag) as T;
  Object.assign(el, props);
  return el;
}
