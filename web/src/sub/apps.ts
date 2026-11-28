// The apps the page offers, as the server's catalogue (subpage.AppCatalog) has them, and the
// links of their buttons: the one that adds the subscription, the ones that download the app.
import type { components } from "../api/schema";
import { t, tMaybe } from "../i18n";

export type Platform = "ios" | "android" | "windows" | "macos" | "linux";
export type CatalogApp = components["schemas"]["CatalogApp"];
export type AppCatalog = components["schemas"]["AppCatalog"];
export type AppDownload = components["schemas"]["AppDownload"];

/** A catalogue of no apps: the page served without its data (the Vite dev server). */
export const NO_APPS: AppCatalog = { ios: [], android: [], windows: [], macos: [], linux: [] };

// The rules of the server's subpage.AppLinkProblem, held here too: a link the server would
// refuse gives no button, whatever the data says.
/** How long an app's link may be (subpage.MaxLinkLen). */
export const MAX_LINK = 500;
const SCHEME = /^[a-z][a-z0-9+.-]{0,31}$/;
// Schemes that open something other than an app (any ms- one too); http would carry the
// subscription's address in the clear.
const REFUSED = new Set(["http", "javascript", "vbscript", "data", "file", "blob", "about", "filesystem", "view-source", "jar", "search-ms", "search", "shell"]);
const PLACEHOLDER = /\{(url|url_enc|name)\}/g;
// Anything but printable ASCII: browsers drop tabs and line breaks from a link and read past
// letters that only look Latin, so such a link is refused rather than cleaned.
const NOT_PRINTABLE = /[^!-~]/;

/** The scheme link opens, when it is printable ASCII and the scheme is an app's; null otherwise. */
function appScheme(link: string): string | null {
  const colon = link.indexOf(":");
  if (colon < 0 || NOT_PRINTABLE.test(link)) return null;
  const scheme = link.slice(0, colon);
  return SCHEME.test(scheme) && !scheme.startsWith("ms-") && !REFUSED.has(scheme) ? scheme : null;
}

/** Why the server refuses an app's link template (its errors.api code), in the order it checks; null when it takes it. */
export function linkProblem(link: string): "app_link_invalid" | "app_link_scheme" | "app_link_url" | null {
  if (link.length > MAX_LINK || NOT_PRINTABLE.test(link)) return "app_link_invalid";
  if (!appScheme(link)) return "app_link_scheme";
  if (!/\{url(_enc)?\}/.test(link)) return "app_link_url";
  return /[{}]/.test(link.replace(PLACEHOLDER, "")) ? "app_link_invalid" : null;
}

/** The scheme of an app's link template the server would take: the address in it, nothing but the placeholders in braces. */
function templateScheme(link: string): string | null {
  return linkProblem(link) ? null : appScheme(link);
}

/** What an app's link is filled with: the subscription's address, the brand and Happ's crypt link when the panel gives one. */
export type LinkValues = { url: string; brand: string; happ?: string };

/**
 * The link that adds the subscription to app: its template with {url} (the address as it
 * is), {url_enc} and {name} (the address and the brand percent-encoded), filled in one pass,
 * or the panel's crypt link for an app that takes it. Null when the template breaks the
 * server's rules or the link would open something other than the app.
 */
export function appLink(app: Pick<CatalogApp, "link" | "crypt">, v: LinkValues): string | null {
  const scheme = templateScheme(app.link);
  if (!scheme) return null;
  // The crypt link is the same app's, or none is opened: the plain one would show the
  // address the admin chose to hide.
  if (app.crypt && v.happ) return appScheme(v.happ) === scheme ? v.happ : null;
  const link = app.link.replace(PLACEHOLDER, (_, name: string) => (name === "url" ? v.url : encodeURIComponent(name === "url_enc" ? v.url : v.brand)));
  return appScheme(link) === scheme ? link : null;
}

/** An app the page offers, with the link that adds the subscription to it; null for an own app the admin gave none. */
export type AppRow = { app: CatalogApp; link: string | null };

/**
 * The rows of the apps the page offers, of apps in their order. The apps the admin turned
 * away (blocked) are not offered: they would be given a stub, not the servers. An app with no
 * link shows how to get it, with no "Add"; one whose link would open no app is not offered.
 */
export function appRows(apps: readonly CatalogApp[], blocked: readonly string[] | undefined, values: LinkValues): AppRow[] {
  return apps.flatMap((app): AppRow[] => {
    if (blocked?.includes(app.id)) return [];
    if (!app.link) return [{ app, link: null }];
    const link = appLink(app, values);
    return link ? [{ app, link }] : [];
  });
}

/** Where an app is downloaded: the page it opens and the button's text. */
export type Source = { href: string; label: string };

/**
 * How a visitor gets app: the download buttons the page can open, its instruction when one the
 * visitor can open (published: a draft, or one deleted since, is none), and whether "Download"
 * opens them under the row. One download with no instruction is a link of its own.
 */
export function waysToGet(app: Pick<CatalogApp, "downloads" | "doc">, published: (doc: number) => boolean): { sources: Source[]; doc: number; toggles: boolean } {
  const sources = app.downloads.flatMap((d) => {
    const href = downloadHref(d.url);
    return href ? [{ href, label: downloadLabel(d) }] : [];
  });
  const doc = app.doc > 0 && published(app.doc) ? app.doc : 0;
  return { sources, doc, toggles: sources.length > 1 || (sources.length === 1 && doc > 0) };
}

/** The page a download button opens: an https address with a host and no user in it, as the server takes it (textutil.ValidLink); null for any other. */
export function downloadHref(url: string): string | null {
  if (Array.from(url).some((c) => c <= " " || c === "\u007f")) return null;
  try {
    const u = new URL(url);
    return u.protocol === "https:" && u.hostname !== "" && !u.username && !u.password ? u.href : null;
  } catch {
    return null;
  }
}

/** The line under an app's name: an own app's is the admin's text, a built-in app's a key of sub.notes, put in the visitor's words. */
export const appNote = (app: Pick<CatalogApp, "note" | "own">): string => (app.own ? app.note : (tMaybe(`sub.notes.${app.note}`) ?? app.note));

/** Where an app's picture is, as the page (or the admin's preview) reaches it; undefined when it has none. */
export type IconSource = (app: Pick<CatalogApp, "id" | "icon">) => string | undefined;

/** How an app is drawn in its row. */
export type AppIconView = { kind: "image"; src: string } | { kind: "emoji"; emoji: string } | { kind: "letter"; letter: string };

/** How app is drawn: its picture where src finds one, its emoji, or else the first letter of its name. */
export function appIcon(app: Pick<CatalogApp, "id" | "name" | "icon">, src: IconSource): AppIconView {
  const image = app.icon.kind === "image" ? src(app) : undefined;
  if (image) return { kind: "image", src: image };
  if (app.icon.kind === "emoji" && app.icon.emoji) return { kind: "emoji", emoji: app.icon.emoji };
  return { kind: "letter", letter: Array.from(app.name.trim())[0] ?? "?" };
}

const STORES: Partial<Record<AppDownload["kind"], string>> = { appstore: "App Store", googleplay: "Google Play", github: "GitHub" };

/** A download button's text: the admin's own (a link always has one), else the store's name or "Mirror". */
export function downloadLabel(d: AppDownload): string {
  return d.label || (STORES[d.kind] ?? t("sub.mirror"));
}

export function detect(): Platform {
  const ua = navigator.userAgent;
  if (/iPhone|iPad|iPod/.test(ua)) return "ios";
  if (/Android/.test(ua)) return "android";
  if (/Mac OS X/.test(ua)) return "macos";
  if (/Windows/.test(ua)) return "windows";
  if (/Linux|X11|CrOS/.test(ua)) return "linux"; // after Android: its browsers say Linux too
  return "android";
}
