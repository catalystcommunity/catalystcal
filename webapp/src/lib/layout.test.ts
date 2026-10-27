import { describe, expect, it } from "vitest";
import { assignLanes, layoutTimed } from "./layout";

describe("layoutTimed", () => {
  it("gives a lone event the full width", () => {
    const m = layoutTimed([{ id: "a", start: 60, end: 120 }]);
    expect(m.get("a")).toEqual({ col: 0, cols: 1, span: 1 });
  });

  it("puts two overlapping events side by side", () => {
    const m = layoutTimed([
      { id: "a", start: 60, end: 180 },
      { id: "b", start: 120, end: 240 },
    ]);
    expect(m.get("a")).toEqual({ col: 0, cols: 2, span: 1 });
    expect(m.get("b")).toEqual({ col: 1, cols: 2, span: 1 });
  });

  it("does not treat touching events as overlapping", () => {
    const m = layoutTimed([
      { id: "a", start: 60, end: 120 },
      { id: "b", start: 120, end: 180 },
    ]);
    expect(m.get("a")?.cols).toBe(1);
    expect(m.get("b")?.cols).toBe(1);
  });

  it("reuses a free column and widens events over free columns", () => {
    // a: 9-12, b: 9-10, c: 10-11, d: 11-12 -> two columns; b,c,d share col 1.
    const m = layoutTimed([
      { id: "a", start: 540, end: 720 },
      { id: "b", start: 540, end: 600 },
      { id: "c", start: 600, end: 660 },
      { id: "d", start: 660, end: 720 },
    ]);
    expect(m.get("a")).toEqual({ col: 0, cols: 2, span: 1 });
    expect(m.get("b")?.col).toBe(1);
    expect(m.get("c")?.col).toBe(1);
    expect(m.get("d")?.col).toBe(1);
  });

  it("chains overlaps into one cluster", () => {
    // a overlaps b, b overlaps c, a does not overlap c; e is short at the top.
    const m = layoutTimed([
      { id: "a", start: 0, end: 60 },
      { id: "b", start: 30, end: 120 },
      { id: "c", start: 90, end: 150 },
      { id: "e", start: 0, end: 20 },
    ]);
    expect(m.get("a")).toEqual({ col: 0, cols: 2, span: 1 });
    expect(m.get("e")).toEqual({ col: 1, cols: 2, span: 1 });
    // b reuses the column e left; c reuses the column a left.
    expect(m.get("b")).toEqual({ col: 1, cols: 2, span: 1 });
    expect(m.get("c")).toEqual({ col: 0, cols: 2, span: 1 });
  });

  it("separates clusters", () => {
    const m = layoutTimed([
      { id: "a", start: 0, end: 60 },
      { id: "b", start: 0, end: 60 },
      { id: "c", start: 120, end: 180 },
    ]);
    expect(m.get("a")?.cols).toBe(2);
    expect(m.get("c")).toEqual({ col: 0, cols: 1, span: 1 });
  });

  it("gives very short events a minimum length", () => {
    const m = layoutTimed(
      [
        { id: "a", start: 0, end: 5 },
        { id: "b", start: 10, end: 60 },
      ],
      30,
    );
    expect(m.get("a")?.cols).toBe(2);
  });

  it("widens an event across free columns to its right", () => {
    // x 0-100 col0, y 0-30 col1, z 0-30 col2; w 50-100 takes col1 and widens into col2.
    const m = layoutTimed([
      { id: "x", start: 0, end: 100 },
      { id: "y", start: 0, end: 30 },
      { id: "z", start: 0, end: 30 },
      { id: "w", start: 50, end: 100 },
    ]);
    expect(m.get("w")).toEqual({ col: 1, cols: 3, span: 2 });
  });
});

describe("assignLanes", () => {
  it("stacks overlapping bars and reuses lanes", () => {
    const m = assignLanes([
      { id: "a", startIdx: 0, endIdx: 2 },
      { id: "b", startIdx: 1, endIdx: 3 },
      { id: "c", startIdx: 3, endIdx: 4 },
      { id: "d", startIdx: 5, endIdx: 6 },
    ]);
    expect(m.get("a")).toBe(0);
    expect(m.get("b")).toBe(1);
    expect(m.get("c")).toBe(0);
    expect(m.get("d")).toBe(0);
  });
});
