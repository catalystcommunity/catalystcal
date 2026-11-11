import { describe, expect, it, vi, beforeEach } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/dom";

const listDevUsers = vi.fn();
const devLogin = vi.fn();
vi.mock("~/data/clients", () => ({
  devAuthClient: { listDevUsers: (...a: unknown[]) => listDevUsers(...a), devLogin: (...a: unknown[]) => devLogin(...a) },
  authClient: { me: vi.fn(async () => ({ user: {}, via: "self", principals: [] })), resumeSession: vi.fn(), endSession: vi.fn() },
  calendarClient: { listCalendars: vi.fn(async () => ({ calendars: [] })) },
}));

import { LoginPage } from "./Login";
import { signOut } from "~/stores/auth";
import { mount } from "~/test/render";

const renderLogin = () => mount(new LoginPage());

describe("LoginPage", () => {
  beforeEach(() => {
    // The session outlives a test; a signed-in session redirects away.
    signOut();
    localStorage.clear();
    listDevUsers.mockReset();
    devLogin.mockReset();
  });

  it("starts the LinkKeys sign-in at the login name's domain", async () => {
    listDevUsers.mockRejectedValue(new Error("unknown service"));
    const assign = vi.spyOn(window.location, "assign").mockImplementation(() => {});
    await renderLogin();
    const button = screen.getByRole("button", { name: "Sign in with LinkKeys" }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    fireEvent.input(screen.getByLabelText("LinkKeys login name"), { target: { value: " tod@todandlorna.com " } });
    await waitFor(() => expect(button.disabled).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(assign).toHaveBeenCalledWith("/api/v1/auth/start?identity=tod%40todandlorna.com"));
    expect(localStorage.getItem("catalystcal.login.name")).toBeNull();
    assign.mockRestore();
  });

  it("remembers the login name when asked", async () => {
    listDevUsers.mockRejectedValue(new Error("unknown service"));
    const assign = vi.spyOn(window.location, "assign").mockImplementation(() => {});
    const first = await renderLogin();
    fireEvent.input(screen.getByLabelText("LinkKeys login name"), { target: { value: "tod@todandlorna.com" } });
    fireEvent.click(screen.getByLabelText("Remember my login name"));
    // The push-button enables its inner button at its next render.
    const button = screen.getByRole("button", { name: "Sign in with LinkKeys" }) as HTMLButtonElement;
    await waitFor(() => expect(button.disabled).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(localStorage.getItem("catalystcal.login.name")).toBe("tod@todandlorna.com"));
    first.remove();

    // The next visit fills the name in and keeps the box checked.
    await renderLogin();
    expect((screen.getByLabelText("LinkKeys login name") as HTMLInputElement).value).toBe("tod@todandlorna.com");
    const box = screen.getByLabelText("Remember my login name") as HTMLInputElement;
    expect(box.checked).toBe(true);

    // Clearing the box forgets the name at the next sign-in.
    fireEvent.click(box);
    fireEvent.click(screen.getByRole("button", { name: "Sign in with LinkKeys" }));
    await waitFor(() => expect(localStorage.getItem("catalystcal.login.name")).toBeNull());
    assign.mockRestore();
  });

  it("hides the dev panel when the dev service does not answer", async () => {
    listDevUsers.mockRejectedValue(new Error("unknown service"));
    await renderLogin();
    await waitFor(() => expect(listDevUsers).toHaveBeenCalled());
    expect(screen.queryByText("Development sign-in")).toBeNull();
  });

  it("shows dev users and signs in as one", async () => {
    listDevUsers.mockResolvedValue({ users: [{ userId: "u1", handle: "alice", displayName: "Alice", domain: "dev.local" }] });
    devLogin.mockResolvedValue({ token: "t", expiresAt: "2099-01-01T00:00:00Z", user: { userId: "u1", domain: "dev.local", subject: "alice" }, via: "self" });
    await renderLogin();
    const btn = await screen.findByRole("button", { name: /Alice/ });
    fireEvent.click(btn);
    await waitFor(() => expect(devLogin).toHaveBeenCalledWith({ userId: "u1" }));
  });

  it("signs in a new dev user by handle", async () => {
    listDevUsers.mockResolvedValue({ users: [] });
    devLogin.mockResolvedValue({ token: "t", expiresAt: "2099-01-01T00:00:00Z", user: { userId: "u2", domain: "dev.local", subject: "bob" }, via: "self" });
    await renderLogin();
    const input = await screen.findByLabelText("Handle");
    fireEvent.input(input, { target: { value: "bob" } });
    const submit = screen.getByRole("button", { name: "Sign in as dev user" }) as HTMLButtonElement;
    await waitFor(() => expect(submit.disabled).toBe(false));
    fireEvent.click(submit);
    await waitFor(() => expect(devLogin).toHaveBeenCalledWith({ handle: "bob" }));
  });
});
