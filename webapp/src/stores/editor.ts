/** Open state of the event editor and the event detail dialog. */
import type { Occurrence } from "@catalystcal/client";
import type { EventForm } from "~/lib/eventForm";
import { cell } from "~/state/store";

/** The store collection of this module. */
export const EDITOR = "editor";

export type EditorState = { mode: "create"; form: EventForm } | { mode: "edit"; form: EventForm; occ: Occurrence };

const editorCell = cell<EditorState | null>(EDITOR, "editor", null);
const detailCell = cell<Occurrence | null>(EDITOR, "detail", null);

export const editorState = (): EditorState | null => editorCell.get();
export const detailOccurrence = (): Occurrence | null => detailCell.get();

export function openEditor(s: EditorState): void {
  detailCell.set(null);
  editorCell.set(s);
}
export const closeEditor = () => editorCell.set(null);
export const openDetail = (o: Occurrence) => detailCell.set(o);
export const closeDetail = () => detailCell.set(null);
