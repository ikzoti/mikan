import { describe, expect, it } from "vitest";
import { orderApps } from "./page";

const catalogue = ["kabosu", "koala", "slothclash", "verge", "hiddify"].map((id) => ({ id }));
const ids = (apps: { id: string }[]) => apps.map((a) => a.id).join(",");

describe("orderApps", () => {
  it("puts the ordered ids first and the rest after them, in the catalogue's order", () => {
    expect(ids(orderApps(catalogue, { order: ["hiddify", "koala"], hidden: [] }))).toBe("hiddify,koala,kabosu,slothclash,verge");
  });
  it("leaves the hidden ones out, ordered or not", () => {
    expect(ids(orderApps(catalogue, { order: ["verge", "kabosu"], hidden: ["kabosu", "slothclash"] }))).toBe("verge,koala,hiddify");
  });
  it("skips ids the catalogue has not, and keeps the catalogue as it is", () => {
    const before = ids(catalogue);
    expect(ids(orderApps(catalogue, { order: ["gone", "verge"], hidden: ["gone"] }))).toBe("verge,kabosu,koala,slothclash,hiddify");
    expect(ids(catalogue)).toBe(before);
  });
  it("matches ids, not names", () => {
    expect(ids(orderApps(catalogue, { order: ["Clash Verge Rev"], hidden: ["Koala Clash"] }))).toBe(ids(catalogue));
  });
  it("gives the catalogue as it is without a list", () => {
    expect(orderApps(catalogue, undefined)).toBe(catalogue);
  });
});
