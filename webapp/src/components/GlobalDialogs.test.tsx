import { describe, expect, it } from "vitest";
import { render, screen, fireEvent } from "@solidjs/testing-library";
import { GlobalDialogs } from "./GlobalDialogs";
import { askScope, confirmAction, usePendingDialog } from "~/stores/dialogs";

// Each button clears the pending dialog, which unmounts the dialog. The
// promise must still resolve (a "Stale read" here once left revoke and
// delete stuck).
describe("GlobalDialogs", () => {
  it("resolves a confirm with true and closes", async () => {
    render(() => <GlobalDialogs />);
    const p = confirmAction({ title: "Revoke delegate", message: "m", confirmLabel: "Revoke", danger: true });
    fireEvent.click(await screen.findByRole("button", { name: "Revoke" }));
    await expect(p).resolves.toBe(true);
    expect(usePendingDialog()()).toBeNull();
  });

  it("resolves a confirm with false on cancel", async () => {
    render(() => <GlobalDialogs />);
    const p = confirmAction({ title: "Delete", message: "m", confirmLabel: "Delete" });
    fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    await expect(p).resolves.toBe(false);
  });

  it("resolves a scope prompt with the chosen scope", async () => {
    render(() => <GlobalDialogs />);
    const p = askScope("delete");
    fireEvent.click(await screen.findByRole("radio", { name: "All events" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await expect(p).resolves.toBe("all");
  });
});
