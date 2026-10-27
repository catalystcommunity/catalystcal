import { createSignal, onCleanup } from "solid-js";

/** A signal that follows a CSS media query. Call inside a component. */
export function createMediaQuery(query: string): () => boolean {
  let mql: MediaQueryList | null = null;
  try {
    mql = typeof matchMedia === "function" ? matchMedia(query) : null;
  } catch {
    mql = null;
  }
  const [matches, setMatches] = createSignal(mql?.matches ?? false);
  if (mql) {
    const on = (e: MediaQueryListEvent) => setMatches(e.matches);
    mql.addEventListener?.("change", on);
    onCleanup(() => mql?.removeEventListener?.("change", on));
  }
  return matches;
}

/** True when a key press should not be a shortcut (typing, dialogs open). */
export function shortcutBlocked(e: KeyboardEvent): boolean {
  if (e.metaKey || e.ctrlKey || e.altKey || e.defaultPrevented) return true;
  const t = e.target as HTMLElement | null;
  if (t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName))) return true;
  if (document.querySelector('[aria-modal="true"]')) return true;
  if (t?.closest?.('[role="menu"]')) return true;
  return false;
}
