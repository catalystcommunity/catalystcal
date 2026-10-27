import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";

const releasePrincipal = vi.fn();
vi.mock("~/data/clients", () => ({
  authClient: {
    releasePrincipal: (...a: unknown[]) => releasePrincipal(...a),
    me: vi.fn(async () => ({ user: { userId: "u-del" }, via: "self", principals: [] })),
    resumeSession: vi.fn(),
    endSession: vi.fn(),
  },
  calendarClient: { listCalendars: vi.fn(async () => ({ calendars: [] })) },
}));

import { ActingBanner } from "./ActingBanner";
import { isActing, signIn, signOut } from "~/stores/auth";

const delegate = { userId: "u-del", domain: "example.com", subject: "del", displayName: "Dee Legate" };
const boss = { userId: "u-boss", domain: "example.com", subject: "boss", displayName: "Pat Boss" };

describe("ActingBanner", () => {
  beforeEach(() => {
    signOut();
    releasePrincipal.mockReset();
  });

  it("is absent for a self bearer", () => {
    signIn({ token: "t", expiresAt: "2099-01-01T00:00:00Z", user: delegate, via: "self" });
    render(() => <ActingBanner />);
    expect(screen.queryByRole("region", { name: /Acting as/ })).toBeNull();
  });

  it("appears while acting, and Stop acting returns to a self bearer", async () => {
    signIn({ token: "t2", expiresAt: "2099-01-01T00:00:00Z", user: delegate, actingAs: boss, via: "delegation", scopes: ["calendars.read"] });
    releasePrincipal.mockResolvedValue({ token: "t3", expiresAt: "2099-01-01T00:00:00Z", user: delegate, via: "self" });
    render(() => <ActingBanner />);
    const banner = screen.getByRole("region", { name: /Acting as/ });
    expect(banner).toHaveTextContent("Acting as Pat Boss");
    expect(banner).toHaveTextContent("changes are made in their name, and they can see what you did");
    fireEvent.click(screen.getByRole("button", { name: "Stop acting" }));
    await waitFor(() => expect(releasePrincipal).toHaveBeenCalled());
    await waitFor(() => expect(isActing()).toBe(false));
    expect(screen.queryByRole("region", { name: /Acting as/ })).toBeNull();
  });
});
