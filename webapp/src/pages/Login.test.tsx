import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import { Route, Router } from "@solidjs/router";

const listDevUsers = vi.fn();
const devLogin = vi.fn();
vi.mock("~/data/clients", () => ({
  devAuthClient: { listDevUsers: (...a: unknown[]) => listDevUsers(...a), devLogin: (...a: unknown[]) => devLogin(...a) },
  authClient: { me: vi.fn(async () => ({ user: {}, via: "self", principals: [] })), resumeSession: vi.fn(), endSession: vi.fn() },
  calendarClient: { listCalendars: vi.fn(async () => ({ calendars: [] })) },
}));

import { LoginPage } from "./Login";
import { signOut } from "~/stores/auth";

const renderLogin = () =>
  render(() => (
    <Router>
      <Route path="*" component={LoginPage} />
    </Router>
  ));

describe("LoginPage", () => {
  beforeEach(() => {
    // The session signal outlives a test; a signed-in session redirects away.
    signOut();
    listDevUsers.mockReset();
    devLogin.mockReset();
  });

  it("starts the LinkKeys sign-in", async () => {
    listDevUsers.mockRejectedValue(new Error("unknown service"));
    const assign = vi.spyOn(window.location, "assign").mockImplementation(() => {});
    renderLogin();
    fireEvent.click(screen.getByRole("button", { name: "Sign in with LinkKeys" }));
    expect(assign).toHaveBeenCalledWith("/api/v1/auth/start");
    assign.mockRestore();
  });

  it("hides the dev panel when the dev service does not answer", async () => {
    listDevUsers.mockRejectedValue(new Error("unknown service"));
    renderLogin();
    await waitFor(() => expect(listDevUsers).toHaveBeenCalled());
    expect(screen.queryByText("Development sign-in")).toBeNull();
  });

  it("shows dev users and signs in as one", async () => {
    listDevUsers.mockResolvedValue({ users: [{ userId: "u1", handle: "alice", displayName: "Alice", domain: "dev.local" }] });
    devLogin.mockResolvedValue({ token: "t", expiresAt: "2099-01-01T00:00:00Z", user: { userId: "u1", domain: "dev.local", subject: "alice" }, via: "self" });
    renderLogin();
    const btn = await screen.findByRole("button", { name: /Alice/ });
    fireEvent.click(btn);
    await waitFor(() => expect(devLogin).toHaveBeenCalledWith({ userId: "u1" }));
  });

  it("signs in a new dev user by handle", async () => {
    listDevUsers.mockResolvedValue({ users: [] });
    devLogin.mockResolvedValue({ token: "t", expiresAt: "2099-01-01T00:00:00Z", user: { userId: "u2", domain: "dev.local", subject: "bob" }, via: "self" });
    renderLogin();
    const input = await screen.findByLabelText("Handle");
    fireEvent.input(input, { target: { value: "bob" } });
    fireEvent.click(screen.getByRole("button", { name: "Sign in as dev user" }));
    await waitFor(() => expect(devLogin).toHaveBeenCalledWith({ handle: "bob" }));
  });
});
