import { describe, expect, it } from "vitest";
import { hiddenWith } from "./app-list";

// A platform as full as it gets: six built-in apps and twenty of the admin's (subpage.MaxOwnApps).
const full = ["mikan", "clashfest", "happ", "incy", "v2raytun", "hiddify", ...Array.from({ length: 20 }, (_, i) => `own-${i}`)];

describe("hiddenWith", () => {
  const ids = ["happ", "incy", "own-a"];

  it("hides and shows an app", () => {
    expect(hiddenWith([], ids, "happ", false)).toEqual(["happ"]);
    expect(hiddenWith(["happ", "incy"], ids, "happ", true)).toEqual(["incy"]);
    expect(hiddenWith(["happ"], ids, "happ", false)).toEqual(["happ"]);
  });

  it("drops the ids of apps there are no more", () => {
    expect(hiddenWith(["own-gone", "incy"], ids, "own-a", false)).toEqual(["incy", "own-a"]);
    expect(hiddenWith(["own-gone", "incy"], ids, "incy", true)).toEqual([]);
  });

  it("keeps a full platform's list within the apps it has", () => {
    let hidden: string[] = ["own-gone-1", "own-gone-2"];
    for (const id of full) hidden = hiddenWith(hidden, full, id, false);
    expect(hidden).toEqual(full);
  });
});
