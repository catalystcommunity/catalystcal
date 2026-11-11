import "@testing-library/jest-dom/vitest";
import { afterEach, beforeEach } from "vitest";
import { defineZest } from "~/ui/zest";

// happy-dom under Node 26 has no localStorage. Modules read it at import
// time, and setupFiles run first, so install an in-memory one here.
if (typeof globalThis.localStorage === "undefined") {
  class MemStorage implements Storage {
    private m = new Map<string, string>();
    get length() {
      return this.m.size;
    }
    clear() {
      this.m.clear();
    }
    getItem(k: string) {
      return this.m.has(k) ? this.m.get(k)! : null;
    }
    setItem(k: string, v: string) {
      this.m.set(k, String(v));
    }
    removeItem(k: string) {
      this.m.delete(k);
    }
    key(i: number) {
      return [...this.m.keys()][i] ?? null;
    }
    [name: string]: unknown;
  }
  Object.defineProperty(globalThis, "localStorage", { value: new MemStorage(), configurable: true });
}

// The app defines the Zest tags at boot; tests do the same.
defineZest();

// The router reads real history, so reset it for each test.
beforeEach(() => {
  window.history.pushState({}, "", "/");
});
afterEach(() => {
  document.body.replaceChildren();
  localStorage.clear();
});
