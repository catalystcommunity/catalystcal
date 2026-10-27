/**
 * Promise-based confirm and recurrence-scope prompts, drawn once by
 * <GlobalDialogs/> in the app shell.
 */
import { createSignal } from "solid-js";
import type { RecurrenceScope } from "@catalystcal/client";

export interface ConfirmRequest {
  kind: "confirm";
  title: string;
  message: string;
  confirmLabel: string;
  danger?: boolean;
  resolve: (ok: boolean) => void;
}

export interface ScopeRequest {
  kind: "scope";
  action: "save" | "delete";
  resolve: (scope: RecurrenceScope | null) => void;
}

const [pending, setPending] = createSignal<ConfirmRequest | ScopeRequest | null>(null);
export const usePendingDialog = () => pending;
export const clearPendingDialog = () => setPending(null);

export function confirmAction(opts: { title: string; message: string; confirmLabel: string; danger?: boolean }): Promise<boolean> {
  return new Promise((resolve) => setPending({ kind: "confirm", ...opts, resolve }));
}

export function askScope(action: "save" | "delete"): Promise<RecurrenceScope | null> {
  return new Promise((resolve) => setPending({ kind: "scope", action, resolve }));
}
