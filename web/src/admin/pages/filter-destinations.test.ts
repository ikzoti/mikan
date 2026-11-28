import { describe, expect, it } from "vitest";
import { sameDestination } from "./filter-destinations";

describe("network destinations", () => {
  it.each([
    ["198.51.100.7", "198.51.100.7/32"],
    ["198.51.100.7/24", "198.51.100.0/24"],
    ["2001:0DB8:0:0:0:0:0:1", "2001:db8::1/128"],
    ["2001:db8:ffff::1/32", "2001:db8::/32"],
    ["::ffff:198.51.100.7/120", "198.51.100.0/24"],
    ["::ffff:c633:6407", "198.51.100.7/32"],
  ])("matches equivalent forms %s and %s", (a, b) => {
    expect(sameDestination("network", a, b)).toBe(true);
  });

  it.each([
    ["198.51.100.7/24", "198.51.100.7/32"],
    ["0.0.0.0/0", "::/0"],
    ["198.51.100.007", "198.51.100.7/32"],
    ["198.51.100.7/33", "198.51.100.7/32"],
    ["fe80::1%eth0", "fe80::1/128"],
    ["::ffff:198.51.100.7/95", "0.0.0.0/0"],
  ])("keeps %s and %s separate", (a, b) => {
    expect(sameDestination("network", a, b)).toBe(false);
  });
});
