import { createRoot, createSignal } from "solid-js";

/** Wall clock, updated each 30 s (drives the now line and "today"). */
const [now, setNow] = createSignal(Date.now());
createRoot(() => {
  if (typeof window !== "undefined") setInterval(() => setNow(Date.now()), 30_000);
});
export const useNow = () => now;
