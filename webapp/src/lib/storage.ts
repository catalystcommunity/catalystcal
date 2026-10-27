/**
 * localStorage wrappers. Private windows and blocked site data make the
 * accessor throw, so every read and write is guarded.
 */
export function readStore(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function writeStore(key: string, value: string | null): void {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // Storage is not available. The value stays in memory only.
  }
}

export function readJson<T>(key: string, fallback: T): T {
  const raw = readStore(key);
  if (!raw) return fallback;
  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

export function writeJson(key: string, value: unknown): void {
  writeStore(key, JSON.stringify(value));
}
