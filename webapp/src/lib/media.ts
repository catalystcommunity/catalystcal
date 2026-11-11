/** True when the CSS media query matches now. */
export function matchesMedia(query: string): boolean {
  try {
    return typeof matchMedia === "function" ? matchMedia(query).matches : false;
  } catch {
    return false;
  }
}

/** Call `onChange` when the query starts or stops matching. Returns the removal function. */
export function watchMedia(query: string, onChange: (matches: boolean) => void): () => void {
  let mql: MediaQueryList | null = null;
  try {
    mql = typeof matchMedia === "function" ? matchMedia(query) : null;
  } catch {
    mql = null;
  }
  if (!mql?.addEventListener) return () => {};
  const on = (e: MediaQueryListEvent) => onChange(e.matches);
  mql.addEventListener("change", on);
  return () => mql?.removeEventListener("change", on);
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
