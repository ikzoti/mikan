import { describe, expect, it } from "vitest";
import { coveringDomain, sameDestination } from "./filter-destinations";

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

describe("port destinations", () => {
  it.each([
    ["1000 - 2000", "1000-2000"],
    ["443-443", "443"],
    ["0443", "443"],
  ])("matches equivalent forms %s and %s", (a, b) => {
    expect(sameDestination("port", a, b)).toBe(true);
  });

  it.each([
    ["1000-2000", "1000-2001"],
    ["abc", "abd"],
    ["1-2-3", "1-3"],
  ])("keeps %s and %s separate", (a, b) => {
    expect(sameDestination("port", a, b)).toBe(false);
  });
});

describe("domain destinations", () => {
  it("reads a domain as the panel does", () => {
    expect(sameDestination("domain", "*.Example.COM.", "example.com")).toBe(true);
  });

  it("finds the global domain that covers a site", () => {
    expect(coveringDomain("cdn.example.com", ["other.example", "example.com"])).toBe("example.com");
    expect(coveringDomain("example.com", ["example.com"])).toBe("example.com");
    expect(coveringDomain("badexample.com", ["example.com"])).toBeUndefined();
  });
});
