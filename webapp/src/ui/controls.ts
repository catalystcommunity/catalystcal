/**
 * Types of the Zest controls and containers that catalystcal makes, and a
 * modal helper. Make a control with `h("text-field", {...})`.
 */
import type { DialogWindow } from "@catalystcommunity/containers";
import type { ControlOptionInput, PushButton } from "@catalystcommunity/controls";
import { h, type Child } from "~/ui/dom";

export type { DialogWindow, PushButton };

/**
 * What the app uses of text-field, text-area, combo-box, spin-box,
 * radio-group, and date-time-picker. The picker has no `label` property.
 */
export interface ValueControl extends HTMLElement {
  value: string;
  label?: string;
  disabled: boolean;
  required: boolean;
  checkValidity(): boolean;
  setCustomValidity?(message: string): void;
}

/** check-box, toggle-switch. */
export interface CheckControl extends HTMLElement {
  checked: boolean;
  disabled: boolean;
}

export type Option = ControlOptionInput;

/** combo-box, list-box, radio-group with options. */
export interface OptionControl extends ValueControl {
  options: Option[];
}

/**
 * A modal dialog-window over the page. It is removed when it closes.
 * `result` gives the close value: a button value, or "cancel" (Escape, ×).
 * A plain `button` in `buttons` closes the dialog with its value. A
 * `push-button` gets `data-keep-open`, so its handler can save first and
 * then call `dialog.close()`.
 */
export function openModal(opts: { label: string; className?: string; children: Child[]; buttons?: Child[] }): { dialog: DialogWindow; result: Promise<string> } {
  const dialog = h("dialog-window", { label: opts.label, class: opts.className }, opts.children) as DialogWindow;
  for (const b of opts.buttons ?? []) {
    if (!(b instanceof Element)) continue;
    b.setAttribute("slot", "buttons");
    if (b.localName === "push-button") b.setAttribute("data-keep-open", "");
    dialog.append(b);
  }
  document.body.append(dialog);
  const result = dialog.showModal().finally(() => dialog.remove());
  return { dialog, result };
}
