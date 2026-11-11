/** Wall clock, updated each 30 s ("today" in pages that show it). */
import { cell } from "~/state/store";

export const CLOCK = "clock";
const nowCell = cell(CLOCK, "now", Date.now());
if (typeof window !== "undefined") setInterval(() => nowCell.set(Date.now()), 30_000);
export const now = (): number => nowCell.get();
