// The admin's texts in the visitor's language (#278). Each text of the admin's may have a
// translation into each language of the page, in the i18n of its object by language, with the
// object's own field names; the page shows the one of its visitor's language and the text
// itself where there is none. The page gives the visitor's language (getLocale()), the admin's
// preview the one picked for the admin's texts. lib/localized.ts picks an addon's texts, which
// fall back to English: here the fallback is the admin's own text.
import type { Locale } from "../i18n";
import type { CatalogApp } from "./apps";
import type { DocItem, PageBlock } from "./page";

/** A language of the admin's texts: one of the page's, or "" for the texts themselves, as the editor has them. */
export type TextLang = Locale | "";

/** An object's texts by language, as the API gives them: the object's own field names. */
export type Texts<F extends string> = { [lang: string]: Partial<Record<F, string>> };

/** The text of an object's field in lang: its translation when it has one, else the object's own. */
export function textIn<F extends string>(lang: TextLang, own: string, i18n: Texts<F> | undefined, field: F): string {
  const translated = lang ? i18n?.[lang]?.[field]?.trim() : "";
  return translated || own;
}

/** b with its texts in lang: its heading, its text and its buttons' texts. */
export function localBlock(b: PageBlock, lang: TextLang): PageBlock {
  if (!lang) return b;
  return {
    ...b,
    title: textIn(lang, b.title, b.i18n, "title"),
    text: b.type === "text" ? textIn(lang, b.text ?? "", b.i18n, "text") : b.text,
    links: b.links?.map((l) => ({ ...l, label: textIn(lang, l.label, l.i18n, "label") })),
  };
}

/** app with the admin's texts in lang: an own app's note (a built-in app's is a key the page words itself) and its buttons' texts. */
export function localApp(app: CatalogApp, lang: TextLang): CatalogApp {
  if (!lang) return app;
  return { ...app, note: textIn(lang, app.note, app.i18n, "note"), downloads: app.downloads.map((d) => ({ ...d, label: textIn(lang, d.label ?? "", d.i18n, "label") })) };
}

/** doc with its title in lang, and on its own screen its text. */
export function localDoc<D extends DocItem & { body?: string }>(doc: D, lang: TextLang): D {
  if (!lang) return doc;
  const out: D = { ...doc, title: textIn(lang, doc.title, doc.i18n, "title") };
  return doc.body === undefined ? out : { ...out, body: textIn(lang, doc.body, doc.i18n, "body") };
}
