/**
 * Promise-based confirm and recurrence-scope prompts. Each one makes a Zest
 * dialog-window, opens it as a modal, and removes it when it closes.
 */
import type { RecurrenceScope } from "@catalystcal/client";
import { askConfirm, type DialogWindow } from "@catalystcommunity/containers";
import { h } from "~/ui/dom";

export function confirmAction(opts: { title: string; message: string; confirmLabel: string; danger?: boolean }): Promise<boolean> {
  return askConfirm({ title: opts.title, text: opts.message, okLabel: opts.confirmLabel, kind: opts.danger ? "warning" : "question", danger: opts.danger });
}

const SCOPES: { value: RecurrenceScope; label: string }[] = [
  { value: "this", label: "This event" },
  { value: "this_and_future", label: "This and following events" },
  { value: "all", label: "All events" },
];

interface RadioGroup extends HTMLElement {
  value: string;
}

export async function askScope(action: "save" | "delete"): Promise<RecurrenceScope | null> {
  const group = h(
    "radio-group",
    { label: "Which events", value: "this" },
    SCOPES.map((s) => h("radio-button", { value: s.value }, s.label)),
  ) as RadioGroup;
  const hint = h("p", { class: "hint", hidden: true }, "A time change moves every event in the series by the same amount.");
  group.addEventListener("change", () => {
    hint.hidden = !(action === "save" && group.value === "all");
  });
  const dialog = h(
    "dialog-window",
    { label: action === "delete" ? "Delete a repeating event" : "Change a repeating event" },
    group,
    hint,
    h("button", { slot: "buttons", type: "button", value: "ok", "data-default": true, "data-danger": action === "delete" }, action === "delete" ? "Delete" : "Save"),
    h("button", { slot: "buttons", type: "button", value: "cancel" }, "Cancel"),
  ) as DialogWindow;
  document.body.append(dialog);
  try {
    const result = await dialog.showModal();
    return result === "ok" ? (group.value as RecurrenceScope) || "this" : null;
  } finally {
    dialog.remove();
  }
}
