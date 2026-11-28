// What the editor makes of the admin's texts in the page's languages (#278): a field edits the
// text itself or, with a language picked, its translation (the i18n of its object, by language,
// with the object's own field names). The draft keeps no empty translation, so a text typed and
// wiped leaves the page as it was.
import type { Schemas } from "../../../../api/client";
import { LOCALES, type Locale } from "../../../../i18n";
import type { TextLang, Texts } from "../../../../sub/texts";

/** An object's translations by language, as the API has them: entries of the object's own field names. */
type I18n<T> = { [lang: string]: T };

/** An object with a text of its own in field, translated in its i18n. */
type Translated<F extends string> = Partial<Record<F, string>> & { i18n?: Texts<F> };

/** A field of an object that its translations have too. */
type TextField<O extends { i18n?: I18n<object> }> = keyof O & keyof NonNullable<O["i18n"]>[string] & string;

/**
 * i18n with the translation of field into lang set to value: an empty value takes it out, a
 * language left with no text goes, and so does a map left with no language. A translation of
 * one text alone is gone with it, so no entry is left without the field it needs.
 */
export function withText<T extends object>(i18n: I18n<T> | undefined, lang: Locale, field: string, value: string): I18n<T> | undefined {
  const entry: Record<string, unknown> = { ...i18n?.[lang] };
  if (value) entry[field] = value;
  else delete entry[field];
  const next: I18n<T> = { ...i18n, [lang]: entry as T };
  if (Object.keys(entry).length === 0) delete next[lang];
  return Object.keys(next).length ? next : undefined;
}

/** o with field set to value in lang: its own text, or with a language its translation. */
export function withTextOf<O extends { i18n?: I18n<object> }>(o: O, lang: TextLang, field: TextField<O>, value: string): O {
  return lang ? { ...o, i18n: withText(o.i18n, lang, field, value) } : { ...o, [field]: value };
}

/** What the field of o shows in lang: its own text, or with a language its translation ("" when there is none). */
export function textOf<F extends string>(o: Translated<F>, lang: TextLang, field: F): string {
  return (lang ? o.i18n?.[lang]?.[field] : o[field]) ?? "";
}

/** Whether the field of o has a text of its own and no translation of it into lang: the page shows that language the text itself. */
export function lacksText<F extends string>(o: Translated<F>, lang: Locale, field: F): boolean {
  return !!o[field]?.trim() && !o.i18n?.[lang]?.[field]?.trim();
}

/** i18n as the server keeps it: the languages in order, the texts trimmed, the empty ones left out; nothing left is none. */
export function cleanTexts<T extends object>(i18n: I18n<T> | undefined): I18n<T> | undefined {
  const out: I18n<T> = {};
  for (const lang of Object.keys(i18n ?? {}).sort()) {
    const entry = Object.entries(i18n?.[lang] ?? {}).flatMap(([field, text]) => (typeof text === "string" && text.trim() ? [[field, text.trim()]] : []));
    if (entry.length) out[lang] = Object.fromEntries(entry) as T;
  }
  return Object.keys(out).length ? out : undefined;
}

/** Where the API names a refused field of an object in lang: the field itself, or its translation (i18n.en.title). */
export const textPath = (lang: TextLang, field: string): string => (lang ? `i18n.${lang}.${field}` : field);

/** The language of a refused field inside a translation (config.blocks[3].i18n.en.title); "" for a text itself. */
export function langOfField(field: string): TextLang {
  const lang = /(?:^|\.)i18n\.([a-z]+)(?:\.|$)/.exec(field)?.[1];
  return LOCALES.find((l) => l.id === lang)?.id ?? "";
}

/** Whether a block lacks a translation into lang: of its heading, its text or a button's text. */
export function blockLacks(b: Schemas["PageBlock"], lang: Locale): boolean {
  return lacksText(b, lang, "title") || (b.type === "text" && lacksText(b, lang, "text")) || !!b.links?.some((l) => lacksText(l, lang, "label"));
}

/** Whether download buttons lack a translation of a text of theirs into lang. */
export function downloadsLack(list: readonly Schemas["AppDownload"][], lang: Locale): boolean {
  return list.some((d) => lacksText(d, lang, "label"));
}

/** Whether an instruction lacks a translation into lang of its title or its text. */
export function docLacks(doc: Schemas["SubDocView"], lang: Locale): boolean {
  return lacksText(doc, lang, "title") || lacksText(doc, lang, "body");
}
