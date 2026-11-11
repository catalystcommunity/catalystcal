import { readStore, writeStore } from "~/lib/storage";
import { cell } from "~/state/store";
import { PREFS } from "~/stores/prefs";

export type ThemeChoice = "light" | "dark" | "system";
const KEY = "catalystcal.theme";

const initial = ((): ThemeChoice => {
  const v = readStore(KEY);
  return v === "light" || v === "dark" ? v : "system";
})();

const themeCell = cell<ThemeChoice>(PREFS, "theme", initial);
export const theme = (): ThemeChoice => themeCell.get();

function systemDark(): boolean {
  try {
    return matchMedia("(prefers-color-scheme: dark)").matches;
  } catch {
    return false;
  }
}

export function applyTheme(choice: ThemeChoice = theme()): void {
  const t = choice === "system" ? (systemDark() ? "dark" : "light") : choice;
  document.documentElement.setAttribute("data-theme", t);
}

export function setTheme(choice: ThemeChoice): void {
  themeCell.set(choice);
  writeStore(KEY, choice);
  applyTheme(choice);
}

export function watchSystemTheme(): void {
  try {
    matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
      if (theme() === "system") applyTheme("system");
    });
  } catch {
    // matchMedia is not available.
  }
}
