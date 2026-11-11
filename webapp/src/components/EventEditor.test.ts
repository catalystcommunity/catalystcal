import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, waitFor, within } from "@testing-library/dom";
import type { Calendar, Event } from "@catalystcal/client";

const createEvent = vi.fn();
vi.mock("~/data/clients", () => ({
  eventClient: { createEvent: (...a: unknown[]) => createEvent(...a) },
  userClient: { lookupPerson: vi.fn() },
  freeBusyClient: { query: vi.fn(async () => ({ people: [] })) },
}));

import { EventEditor } from "./EventEditor";
import { signIn, signOut } from "~/stores/auth";
import { upsertCalendar } from "~/stores/calendars";
import { editorState, openEditor } from "~/stores/editor";
import { newEventForm } from "~/lib/eventForm";
import { mount, settle } from "~/test/render";

const me = { userId: "u-me", domain: "example.com", subject: "me", displayName: "Me" };
const cal = { calendarId: "c1", name: "Work", owner: me, kind: "local", isDefault: true, access: "owner", accessPath: "owner", createdAt: "", updatedAt: "" } as Calendar;

describe("EventEditor", () => {
  beforeEach(() => {
    createEvent.mockReset();
    signOut();
    signIn({ token: "t", expiresAt: "2099-01-01T00:00:00Z", user: me, via: "self" });
    upsertCalendar(cal);
  });

  it("creates an event with the typed title and closes", async () => {
    createEvent.mockImplementation(async (e: Event) => ({ ...e, eventId: "e1" }));
    const state = { mode: "create" as const, form: newEventForm({ date: "2026-11-10", startMin: 600, endMin: 660, calendarId: "c1", timezone: "America/Denver" }) };
    openEditor(state);
    const editor = new EventEditor();
    editor.state = state;
    await mount(editor);

    const dialog = await waitFor(() => document.querySelector("dialog-window") as HTMLElement);
    const title = within(dialog).getByRole("textbox", { name: "Title", hidden: true }) as HTMLInputElement;
    title.value = "Planning";
    fireEvent.input(title);
    await settle();
    fireEvent.click(within(dialog).getByRole("button", { name: "Create event", hidden: true }));

    await waitFor(() => expect(createEvent).toHaveBeenCalledTimes(1));
    const sent = createEvent.mock.calls[0][0] as Event;
    expect(sent.title).toBe("Planning");
    expect(sent.calendarId).toBe("c1");
    expect(sent.timezone).toBe("America/Denver");
    await waitFor(() => expect(editorState()).toBeNull());
  });

  it("shows a problem and does not save an end before the start", async () => {
    const form = newEventForm({ date: "2026-11-10", startMin: 600, endMin: 660, calendarId: "c1", timezone: "UTC" });
    form.endDate = "2026-11-09";
    const editor = new EventEditor();
    editor.state = { mode: "create", form };
    await mount(editor);
    const dialog = await waitFor(() => document.querySelector("dialog-window") as HTMLElement);
    fireEvent.click(within(dialog).getByRole("button", { name: "Create event", hidden: true }));
    await waitFor(() => expect(dialog.querySelector("p.alert:not([hidden])")).toHaveTextContent(/./));
    expect(createEvent).not.toHaveBeenCalled();
  });
});
