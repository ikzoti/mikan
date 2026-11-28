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

export function sameDestination(kind: "domain" | "network" | "port", a: string, b: string): boolean {
  return kind === "domain" ? normalizedDomain(a) === normalizedDomain(b) : a.trim() === b.trim();
}
