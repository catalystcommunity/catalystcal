import { createSignal } from "solid-js";

export interface Toast {
  id: number;
  text: string;
  kind: "info" | "error";
}

const [toasts, setToasts] = createSignal<Toast[]>([]);
let next = 1;

export const useToasts = () => toasts;

export function toast(text: string, kind: Toast["kind"] = "info", ms = 4000): void {
  const id = next++;
  setToasts((t) => [...t, { id, text, kind }]);
  setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), ms);
}
