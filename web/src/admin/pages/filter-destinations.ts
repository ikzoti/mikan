function normalizedDomain(value: string): string {
  const name = value.trim().toLowerCase().replace(/^\*?\./, "").replace(/\.$/, "");
  if (!name || /[/:?#@]/.test(name)) return name;
  try {
    return new URL("http://" + name).hostname;
  } catch {
    return name;
  }
}

export function coveringDomain(value: string, domains: string[]): string | undefined {
  const name = normalizedDomain(value);
  return domains.find((raw) => {
    const domain = normalizedDomain(raw);
    return domain && (name === domain || name.endsWith("." + domain));
  });
}

// Match the panel's CIDR normalization, including IPv4-mapped IPv6 addresses.
function normalizedNetwork(value: string): string | undefined {
  const [host = "", prefix, ...rest] = value.trim().split("/");
  if (rest.length || (prefix !== undefined && !/^(0|[1-9][0-9]{0,2})$/.test(prefix))) return;
  let width = 32;
  let address: bigint;
  if (host.includes(":")) {
    if (!/^[0-9a-f:.]+$/i.test(host)) return;
    try {
      const canon = new URL(`http://[${host}]`).hostname.slice(1, -1);
      const [head = "", tail] = canon.split("::");
      const first = head ? head.split(":") : [];
      const last = tail ? tail.split(":") : [];
      const parts = tail === undefined ? first : [...first, ...Array<string>(8 - first.length - last.length).fill("0"), ...last];
      address = BigInt("0x" + parts.map((part) => part.padStart(4, "0")).join(""));
      width = 128;
    } catch {
      return;
    }
  } else {
    const parts = host.split(".");
    if (parts.length !== 4 || parts.some((part) => !/^(0|[1-9][0-9]{0,2})$/.test(part) || Number(part) > 255)) return;
    address = parts.reduce((ip, part) => (ip << 8n) | BigInt(part), 0n);
  }
  let bits = prefix === undefined ? width : Number(prefix);
  if (bits > width) return;
  if (width === 128 && bits >= 96 && address >> 32n === 0xffffn) {
    width = 32;
    bits -= 96;
    address &= 0xffffffffn;
  }
  return `${width}:${address >> BigInt(width - bits)}/${bits}`;
}

// Match the panel's port normalization (CanonPorts): spaces around the dash, leading
// zeros and a range of one port are the same entry.
function normalizedPorts(value: string): string | undefined {
  const parts = value.split("-").map((part) => part.trim());
  if (parts.length > 2 || !parts.every((part) => /^\d{1,5}$/.test(part))) return;
  const [lo, hi = lo] = parts.map(Number);
  return lo === hi ? String(lo) : `${lo}-${hi}`;
}

export function sameDestination(kind: "domain" | "network" | "port", a: string, b: string): boolean {
  if (a.trim() === b.trim()) return true;
  if (kind === "domain") return normalizedDomain(a) === normalizedDomain(b);
  const normalized = kind === "network" ? normalizedNetwork : normalizedPorts;
  const value = normalized(a);
  return value !== undefined && value === normalized(b);
}
