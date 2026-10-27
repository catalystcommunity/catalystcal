/** Open state of the event editor and the event detail sheet. */
import { createSignal } from "solid-js";
import type { Occurrence } from "@catalystcal/client";
import type { EventForm } from "~/lib/eventForm";

export type EditorState = { mode: "create"; form: EventForm } | { mode: "edit"; form: EventForm; occ: Occurrence };

const [editor, setEditor] = createSignal<EditorState | null>(null);
const [detail, setDetail] = createSignal<Occurrence | null>(null);

export const useEditor = () => editor;
export const useDetail = () => detail;

export function openEditor(s: EditorState): void {
  setDetail(null);
  setEditor(s);
}
export const closeEditor = () => setEditor(null);
export const openDetail = (o: Occurrence) => setDetail(o);
export const closeDetail = () => setDetail(null);
