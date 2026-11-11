import { describe, expect, it } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/dom";
import { askScope, confirmAction } from "~/stores/dialogs";
import { settle } from "~/test/render";

// Each prompt is a Zest dialog-window in document.body; it is removed when it closes.
describe("dialogs", () => {
  it("resolves a confirm with true and closes", async () => {
    const p = confirmAction({ title: "Revoke delegate", message: "m", confirmLabel: "Revoke", danger: true });
    await settle();
    fireEvent.click(await screen.findByRole("button", { name: "Revoke" }));
    await expect(p).resolves.toBe(true);
    await waitFor(() => expect(document.querySelector("dialog-window")).toBeNull());
  });

  it("resolves a confirm with false on cancel", async () => {
    const p = confirmAction({ title: "Delete", message: "m", confirmLabel: "Delete" });
    await settle();
    fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    await expect(p).resolves.toBe(false);
  });

  it("resolves a scope prompt with the chosen scope", async () => {
    const p = askScope("delete");
    await settle();
    fireEvent.click(await screen.findByRole("radio", { name: "All events" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await expect(p).resolves.toBe("all");
  });

  it("resolves a scope prompt with null on cancel", async () => {
    const p = askScope("save");
    await settle();
    fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    await expect(p).resolves.toBeNull();
  });
});
