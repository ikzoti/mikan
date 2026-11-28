// The language the editor's fields edit the admin's texts in (#278): the texts themselves, or
// their translations into one of the page's languages. The page shows its visitor's language,
// and the text itself where a translation is missing.
import clsx from "clsx";
import { errorText } from "../../../../api/client";
import { Segmented } from "../../../../components/ui";
import { LOCALES, t } from "../../../../i18n";
import type { TextLang } from "../../../../sub/texts";
import { langOfField } from "./text-edits";

/**
 * What a dialog does with a refused save. own are the refusal's errors of the dialog's own
 * fields (fieldErrors, scoped to them): they show at the fields, in the first one's language. A
 * refusal of nothing of the dialog's is said in a toast.
 */
export function showRefusal(err: unknown, own: Record<string, string>, toast: { error: (text: string) => void }, setLang: (lang: TextLang) => void): void {
  const first = Object.keys(own)[0];
  if (first) setLang(langOfField(first));
  else toast.error(errorText(err));
}

/** The switch of the language the fields edit: the texts themselves or a language of the page. */
export function TextLangSwitch({ value, onChange }: { value: TextLang; onChange: (lang: TextLang) => void }) {
  return (
    <Segmented
      label={t("settings.page.textLang")}
      value={value}
      onChange={onChange}
      options={[{ value: "", label: t("settings.page.textBase") }, ...LOCALES.map((l) => ({ value: l.id, label: l.label, lang: l.id }))]}
    />
  );
}

/** What a picked language does to the fields under it; nothing with the texts themselves. */
export function TextLangNote({ lang, className }: { lang: TextLang; className?: string }) {
  if (!lang) return null;
  return <p className={clsx("text-xs text-[var(--ink-500)]", className)}>{t("settings.page.textLangHint")}</p>;
}

/** The switch with what a picked language does said under it: the top of a dialog. */
export function TextLangBar({ value, onChange, className }: { value: TextLang; onChange: (lang: TextLang) => void; className?: string }) {
  return (
    <div className={clsx("flex flex-col items-start gap-2", className)}>
      <TextLangSwitch value={value} onChange={onChange} />
      <TextLangNote lang={value} />
    </div>
  );
}

/** The mark of an item without a translation into the language picked. */
export function NoTranslation() {
  return <span className="tag shrink-0">{t("settings.page.textMissing")}</span>;
}

/** A field's label with the language of the translation it edits, for those who do not see the switch. */
export function textLabel(label: string, lang: TextLang): string {
  const name = LOCALES.find((l) => l.id === lang)?.label;
  return name ? `${label} · ${name}` : label;
}

/** The hint of a field that edits a translation: what the page shows while it is empty. */
export function textHint(own: string | undefined, translation: string): string {
  return own?.trim() && !translation.trim() ? t("settings.page.textMissingHint") : t("settings.page.textEmptyHint");
}
