// What the editor sends of the page's apps: PUT /api/v1/sub-page/apps takes the whole
// document, so a dialog of one app sends the document with that app changed.
import type { Schemas } from "../../../../api/client";
import { same } from "../../../../lib/draft";
import type { Platform } from "../../../../sub/apps";
import { PLATFORMS } from "../../../../sub/page";
import { cleanTexts } from "./text-edits";

type AppsConfig = Schemas["AppsConfig"];
type AppDownload = Schemas["AppDownload"];
type AppIcon = Schemas["AppIcon"];
type OwnApp = Schemas["OwnApp"];
type OwnPlatform = Schemas["OwnPlatform"];
type PlatformEdit = Schemas["PlatformEdit"];
type PresetEdit = Schemas["PresetEdit"];

/** What the dialog edits of an app on one platform: its download buttons and its instruction (0: none). */
export type PlatformDraft = { downloads: AppDownload[]; doc: number };

/** An own app on one platform as its dialog edits it: the line under its name too, in the page's languages. */
export type OwnPlatformDraft = PlatformDraft & { note: string; i18n?: OwnPlatform["i18n"] };

/** An own app as its dialog edits it: the fields of every platform are kept while it is open, those in `on` are sent. */
export type OwnDraft = { id: string; name: string; family: OwnApp["family"]; icon: AppIcon; link: string; on: Platform[]; platforms: Record<Platform, OwnPlatformDraft> };

/** Downloads as the server keeps them: the texts trimmed, no empty label or translation. */
export function cleanDownloads(list: readonly AppDownload[]): AppDownload[] {
  return list.map(({ kind, url, label, i18n }) => {
    const out: AppDownload = { kind, url: url.trim() };
    const text = label?.trim();
    const texts = cleanTexts(i18n);
    if (text) out.label = text;
    if (texts) out.i18n = texts;
    return out;
  });
}

/** An icon as the document keeps it: the letter, which every app has, is none. */
export function iconOf(icon: AppIcon): AppIcon | undefined {
  if (icon.kind === "emoji") return { kind: "emoji", emoji: icon.emoji?.trim() ?? "" };
  return icon.kind === "image" ? { kind: "image" } : undefined;
}

const without = <T>(map: Record<string, T>, key: string): Record<string, T> => Object.fromEntries(Object.entries(map).filter(([k]) => k !== key));

/** apps with e as built-in app id's edit; an edit that changes nothing goes. */
function withPreset(apps: AppsConfig, id: string, e: PresetEdit): AppsConfig {
  const edit: PresetEdit = {};
  if (e.icon) edit.icon = e.icon;
  if (e.platforms && Object.keys(e.platforms).length) edit.platforms = e.platforms;
  const rest = without(apps.presets, id);
  return { ...apps, presets: Object.keys(edit).length ? { ...rest, [id]: edit } : rest };
}

/**
 * apps with app id's edit on platform made of draft, or taken out with no draft (the app's
 * own downloads and no instruction again). Downloads equal to the app's own (preset) are
 * left out, so the app follows its built-in ones; an app with nothing changed goes.
 */
export function withAppEdit(apps: AppsConfig, id: string, platform: string, draft: PlatformDraft | null, preset: readonly AppDownload[]): AppsConfig {
  const edit: PlatformEdit = {};
  if (draft) {
    const downloads = cleanDownloads(draft.downloads);
    if (!same(downloads, cleanDownloads(preset))) edit.downloads = downloads;
    if (draft.doc > 0) edit.doc = draft.doc;
  }
  const others = without(apps.presets[id]?.platforms ?? {}, platform);
  return withPreset(apps, id, { icon: apps.presets[id]?.icon, platforms: Object.keys(edit).length ? { ...others, [platform]: edit } : others });
}

/** apps with built-in app id drawn by icon, on every platform. */
export function withPresetIcon(apps: AppsConfig, id: string, icon: AppIcon): AppsConfig {
  return withPreset(apps, id, { icon: iconOf(icon), platforms: apps.presets[id]?.platforms });
}

/** apps with app in place of the own app of its id, or after the others when it is new. */
export function withOwnApp(apps: AppsConfig, app: OwnApp): AppsConfig {
  const at = apps.own.findIndex((a) => a.id === app.id);
  return { ...apps, own: at < 0 ? [...apps.own, app] : apps.own.map((a, i) => (i === at ? app : a)) };
}

/** apps without own app id. */
export function withoutOwnApp(apps: AppsConfig, id: string): AppsConfig {
  return { ...apps, own: apps.own.filter((a) => a.id !== id) };
}

/** How the API names the fields of own app id in apps sent with it (withOwnApp): by its place there, "own[2].", a new app last. */
export function ownPrefix(apps: AppsConfig, id: string): string {
  const at = apps.own.findIndex((a) => a.id === id);
  return `own[${at < 0 ? apps.own.length : at}].`;
}

/** A new own app's id: own- and up to eight letters and digits, as the own blocks' ids, none of taken. */
export function newOwnId(taken: readonly string[], random: () => number = Math.random): string {
  for (;;) {
    const id = `own-${random().toString(36).slice(2, 10) || "a"}`;
    if (!taken.includes(id)) return id;
  }
}

const NO_PLATFORM: OwnPlatformDraft = { note: "", downloads: [], doc: 0 };

/** The dialog's draft of own app, or of a new one (id) on platform. */
export function ownDraft(app: OwnApp | undefined, id: string, platform: Platform): OwnDraft {
  const platforms = Object.fromEntries(
    PLATFORMS.map((p) => {
      const x = app?.platforms[p];
      return [p, x ? { note: x.note ?? "", downloads: x.downloads ?? [], doc: x.doc ?? 0, i18n: x.i18n } : NO_PLATFORM];
    }),
  ) as Record<Platform, OwnPlatformDraft>;
  if (!app) return { id, name: "", family: "other", icon: { kind: "letter" }, link: "", on: [platform], platforms };
  return { id: app.id, name: app.name, family: app.family, icon: app.icon ?? { kind: "letter" }, link: app.link, on: PLATFORMS.filter((p) => p in app.platforms), platforms };
}

/** The own app the draft makes, as the server keeps it: text trimmed, the platforms of `on` alone, nothing empty sent. */
export function ownAppOf(d: OwnDraft): OwnApp {
  const platforms: Record<string, OwnPlatform> = {};
  for (const p of PLATFORMS.filter((x) => d.on.includes(x))) {
    const x = d.platforms[p];
    const out: OwnPlatform = {};
    const note = x.note.trim();
    const downloads = cleanDownloads(x.downloads);
    const notes = cleanTexts(x.i18n);
    if (note) out.note = note;
    if (downloads.length) out.downloads = downloads;
    if (x.doc > 0) out.doc = x.doc;
    if (notes) out.i18n = notes;
    platforms[p] = out;
  }
  const icon = iconOf(d.icon);
  return { id: d.id, name: d.name.trim(), family: d.family, ...(icon ? { icon } : {}), link: d.link.trim(), platforms };
}

/** The API's field errors under prefix (an app's "presets.happ." or "own[2]."), named from there ("platforms.ios.doc"). */
export function scopedErrors(errors: Record<string, string>, prefix: string): Record<string, string> {
  return Object.fromEntries(Object.entries(errors).flatMap(([k, v]) => (k.startsWith(prefix) ? [[k.slice(prefix.length), v]] : [])));
}
