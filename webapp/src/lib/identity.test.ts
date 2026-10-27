import { describe, expect, it } from "vitest";
import { formatIdentity, isEmail, parseIdentity, sameIdentity } from "./identity";

describe("parseIdentity", () => {
  it("reads handle@domain", () => {
    expect(parseIdentity("alice@example.com")).toEqual({ domain: "example.com", handle: "alice" });
    expect(parseIdentity("  @Alice@Example.COM ")).toEqual({ domain: "example.com", handle: "Alice" });
  });

  it("reads an account id", () => {
    expect(parseIdentity("01J9ZQ3K5V7W8X9Y0Z1A2B3C4D@example.com")).toEqual({ domain: "example.com", userId: "01J9ZQ3K5V7W8X9Y0Z1A2B3C4D" });
    expect(parseIdentity("3f2b1c4d-1111-2222-3333-444455556666@example.com")).toEqual({
      domain: "example.com",
      userId: "3f2b1c4d-1111-2222-3333-444455556666",
    });
    expect(parseIdentity("id:u-42@example.com")).toEqual({ domain: "example.com", userId: "u-42" });
  });

  it("uses the default domain for a bare handle", () => {
    expect(parseIdentity("bob", "example.com")).toEqual({ domain: "example.com", handle: "bob" });
    expect(parseIdentity("bob")).toBeNull();
  });

  it("accepts a dev domain with a port", () => {
    expect(parseIdentity("dev@localhost:8443")).toEqual({ domain: "localhost:8443", handle: "dev" });
  });

  it("refuses junk", () => {
    expect(parseIdentity("")).toBeNull();
    expect(parseIdentity("@")).toBeNull();
    expect(parseIdentity("alice@")).toBeNull();
    expect(parseIdentity("al ice@example.com")).toBeNull();
    expect(parseIdentity("alice@exa mple.com")).toBeNull();
    expect(parseIdentity("alice@-bad.com")).toBeNull();
  });

  it("formats and compares", () => {
    expect(formatIdentity({ domain: "example.com", handle: "alice" })).toBe("alice@example.com");
    expect(formatIdentity({ domain: "example.com", userId: "u-42" })).toBe("id:u-42@example.com");
    expect(sameIdentity({ domain: "Example.com", handle: "Alice" }, { domain: "example.com", handle: "alice" })).toBe(true);
    expect(sameIdentity({ domain: "example.com", userId: "1" }, { domain: "example.com", handle: "1" })).toBe(false);
    expect(isEmail("a@b.co")).toBe(true);
    expect(isEmail("a@localhost")).toBe(false);
  });
});
