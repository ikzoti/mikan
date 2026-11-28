import { describe, expect, it } from "vitest";
import { moved } from "./reorder";

// A platform as full as it gets: six built-in apps and twenty of the admin's (subpage.MaxOwnApps).
const full = ["mikan", "clashfest", "happ", "incy", "v2raytun", "hiddify", ...Array.from({ length: 20 }, (_, i) => `own-${i}`)];

describe("moved", () => {
  it("moves one row of a full list and keeps every other in place", () => {
    const order = moved(full, 25, -1);
    expect(order).toHaveLength(26);
    expect(order.slice(23)).toEqual(["own-17", "own-19", "own-18"]);
    expect(order.slice(0, 23)).toEqual(full.slice(0, 23));
    expect(moved(full, 0, 1).slice(0, 3)).toEqual(["clashfest", "mikan", "happ"]);
  });

  it("leaves the list it was given as it was", () => {
    const list = ["a", "b"];
    expect(moved(list, 1, -1)).toEqual(["b", "a"]);
    expect(list).toEqual(["a", "b"]);
  });
});
