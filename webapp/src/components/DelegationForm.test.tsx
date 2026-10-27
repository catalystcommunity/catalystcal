import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import type { Calendar, Delegation } from "@catalystcal/client";

const createDelegation = vi.fn();
const updateDelegation = vi.fn();
vi.mock("~/data/clients", () => ({
  delegationClient: { createDelegation: (...a: unknown[]) => createDelegation(...a), updateDelegation: (...a: unknown[]) => updateDelegation(...a) },
}));

import { DelegationForm, expiryFromDate, dateFromExpiry } from "./DelegationForm";
import { signIn, signOut } from "~/stores/auth";

const me = { userId: "u-me", domain: "example.com", subject: "me", handle: "me", displayName: "Me" };
const cal = (id: string, name: string): Calendar =>
  ({ calendarId: id, name, owner: me, kind: "local", isDefault: false, access: "owner", accessPath: "owner", createdAt: "", updatedAt: "" }) as Calendar;

describe("DelegationForm", () => {
  beforeEach(() => {
    createDelegation.mockReset();
    updateDelegation.mockReset();
    signOut();
    signIn({ token: "t", expiresAt: "2099-01-01T00:00:00Z", user: me, via: "self" });
  });

  it("creates a delegation with the chosen scopes and calendars", async () => {
    createDelegation.mockImplementation(async (d: Delegation) => ({ ...d, delegationId: "d1" }));
    const onSaved = vi.fn();
    render(() => <DelegationForm calendars={[cal("c1", "Work"), cal("c2", "Home")]} onSaved={onSaved} onCancel={() => {}} />);

    fireEvent.input(screen.getByLabelText("Delegate"), { target: { value: "sam@example.org" } });
    fireEvent.click(screen.getByLabelText(/Create and change events as me/));
    fireEvent.click(screen.getByLabelText(/Answer invitations as me/));
    fireEvent.click(screen.getByLabelText("Only these calendars"));
    fireEvent.click(screen.getByLabelText("Work"));
    fireEvent.input(screen.getByLabelText("Note (optional)"), { target: { value: "While I travel" } });
    fireEvent.submit(screen.getByRole("form", { name: "Add a delegate" }));

    await waitFor(() => expect(createDelegation).toHaveBeenCalledTimes(1));
    const sent = createDelegation.mock.calls[0][0] as Delegation;
    expect(sent.delegate).toEqual({ domain: "example.org", handle: "sam" });
    expect(sent.scopes).toEqual(["calendars.read", "freebusy.read", "events.write", "events.respond"]);
    expect(sent.calendarIds).toEqual(["c1"]);
    expect(sent.note).toBe("While I travel");
    expect(sent.expiresAt).toBeUndefined();
    expect(sent.principal.userId).toBe("u-me");
    expect(onSaved).toHaveBeenCalled();
  });

  it("refuses a bad identity and sends nothing", async () => {
    render(() => <DelegationForm calendars={[]} onSaved={() => {}} onCancel={() => {}} />);
    fireEvent.input(screen.getByLabelText("Delegate"), { target: { value: "not an identity@" } });
    fireEvent.submit(screen.getByRole("form", { name: "Add a delegate" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/user@domain/);
    expect(createDelegation).not.toHaveBeenCalled();
  });

  it("refuses an empty scope list", async () => {
    render(() => <DelegationForm calendars={[]} onSaved={() => {}} onCancel={() => {}} />);
    fireEvent.input(screen.getByLabelText("Delegate"), { target: { value: "sam@example.org" } });
    // Defaults are details + busy; unchecking details leaves busy, then uncheck busy.
    fireEvent.click(screen.getByRole("checkbox", { name: /^See my calendars and event details/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /^See when I.m busy/ }));
    fireEvent.submit(screen.getByRole("form", { name: "Add a delegate" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/at least one/);
    expect(createDelegation).not.toHaveBeenCalled();
  });

  it("updates an existing delegation and keeps its id", async () => {
    const existing: Delegation = {
      delegationId: "d9",
      principal: me,
      delegate: { domain: "example.org", handle: "sam" },
      scopes: ["freebusy.read"],
      createdAt: "2026-01-01T00:00:00Z",
    };
    updateDelegation.mockImplementation(async (d: Delegation) => d);
    render(() => <DelegationForm calendars={[]} initial={existing} onSaved={() => {}} onCancel={() => {}} />);
    fireEvent.click(screen.getByLabelText(/Answer invitations as me/));
    fireEvent.submit(screen.getByRole("form", { name: "Change delegate" }));
    await waitFor(() => expect(updateDelegation).toHaveBeenCalledTimes(1));
    const sent = updateDelegation.mock.calls[0][0] as Delegation;
    expect(sent.delegationId).toBe("d9");
    expect(sent.scopes).toEqual(["freebusy.read", "events.respond"]);
    expect(sent.calendarIds).toEqual([]);
  });

  it("turns an end date into the end of that day and back", () => {
    expect(expiryFromDate("2026-11-30", "America/Denver")).toBe("2026-12-01T07:00:00Z");
    expect(dateFromExpiry("2026-12-01T07:00:00Z", "America/Denver")).toBe("2026-11-30");
  });
});
