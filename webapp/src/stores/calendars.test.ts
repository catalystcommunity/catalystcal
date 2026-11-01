import { describe, expect, it } from "vitest";
import { accessAtLeast, canAddEvents, canEditEvent } from "~/stores/calendars";

describe("access helpers", () => {
  it("orders contribute between read and write", () => {
    expect(accessAtLeast("contribute", "read")).toBe(true);
    expect(accessAtLeast("contribute", "write")).toBe(false);
    expect(accessAtLeast("write", "contribute")).toBe(true);
  });

  it("lets contribute add events", () => {
    expect(canAddEvents("contribute")).toBe(true);
    expect(canAddEvents("read")).toBe(false);
    expect(canAddEvents(undefined)).toBe(false);
  });

  it("lets contribute change only the caller's events", () => {
    expect(canEditEvent("contribute", "u1", "u1")).toBe(true);
    expect(canEditEvent("contribute", "u2", "u1")).toBe(false);
    expect(canEditEvent("contribute", undefined, "u1")).toBe(false);
    expect(canEditEvent("write", "u2", "u1")).toBe(true);
    expect(canEditEvent("read", "u1", "u1")).toBe(false);
  });
});
