/** Test helpers: put an element on the page and wait for its render. */
import { flushRenders } from "@catalystcommunity/base-components";

/** Add `el` to the page, then let microtask and frame renders run. */
export async function mount<T extends Element>(el: T): Promise<T> {
  document.body.append(el);
  await settle();
  return el;
}

/** Let the queued renders run. */
export async function settle(): Promise<void> {
  for (let i = 0; i < 3; i++) {
    await Promise.resolve();
    flushRenders();
  }
}
