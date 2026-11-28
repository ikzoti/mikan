// The brand on the page: its name, its mark and the line under the name, in the page's
// languages. On a group's own page the name and the logo may be the group's own.
import { Link } from "@tanstack/react-router";
import { SwitchRow } from "../../../../components/switch";
import { Field, Segmented } from "../../../../components/ui";
import { t } from "../../../../i18n";
import { ImageField } from "./look";
import { Card, imageURL, type Asset, type Config, type LangProps, type SectionProps } from "./parts";
import { textOf, textPath, withTextOf } from "./text-edits";
import { TextLangNote, TextLangSwitch, textHint, textLabel } from "./text-lang";

type Props = SectionProps &
  LangProps & {
    /** The logo the picker uploads: the panel's, or on a group's page the group's own. */
    logo?: Asset;
    /** The logo the page shows: on a group's page its own, else the panel's. */
    shownLogo?: Asset;
    /** The panel's brand name. */
    brandName: string;
    /** The group whose own page this is. */
    group?: number;
  };

export function BrandSection({ draft, setDraft, set, errors, logo, shownLogo, brandName, group, lang, setLang }: Props) {
  const b = draft.config.brand;
  const og = draft.config.og;
  const setBrand = (patch: Partial<Config["brand"]>) => set((c) => ({ ...c, brand: { ...c.brand, ...patch } }));
  const setOG = (patch: Partial<Config["og"]>) => set((c) => ({ ...c, og: { ...c.og, ...patch } }));
  const panelName = brandName.trim() || "VPN";
  const name = (group ? draft.brand.trim() : "") || panelName;
  const subtitle = textOf(b, lang, "subtitle");
  const subtitleError = errors[`config.brand.${textPath(lang, "subtitle")}`];
  return (
    <>
      <Card title={t("settings.page.brand")} sub={t("settings.page.brandSub")} aside={<TextLangSwitch value={lang} onChange={setLang} />}>
        <TextLangNote lang={lang} className="mb-3" />
        <div className="grid gap-x-3 sm:grid-cols-2">
          {group ? (
            <Field label={t("settings.page.groupBrand")} htmlFor="sp-brand" hint={t("settings.page.groupBrandHint", { name: panelName })} error={errors.brand}>
              <input
                id="sp-brand"
                className="input"
                value={draft.brand}
                maxLength={40}
                placeholder={panelName}
                autoComplete="off"
                onChange={(e) => setDraft((d) => ({ ...d, brand: e.target.value }))}
                aria-invalid={!!errors.brand}
              />
            </Field>
          ) : (
            <Field label={t("settings.brand")} hint={t("settings.page.brandNameHint")}>
              <div className="flex min-h-11 items-center justify-between gap-2 rounded-[var(--r-md)] border border-[var(--hairline)] bg-[var(--surface-soft)] py-1 pr-1 pl-3">
                <span className="truncate font-medium">{panelName}</span>
                <Link to="/settings" search={{ tab: "general" }} className="btn btn-ghost btn-sm">
                  {t("settings.page.brandNameEdit")}
                </Link>
              </div>
            </Field>
          )}
          <Field label={textLabel(t("settings.page.subtitle"), lang)} htmlFor="sp-subtitle" hint={lang ? textHint(b.subtitle, subtitle) : t("settings.page.subtitleHint")} error={subtitleError}>
            <input
              id="sp-subtitle"
              className="input"
              value={subtitle}
              maxLength={80}
              placeholder={lang ? b.subtitle : undefined}
              onChange={(e) => set((c) => ({ ...c, brand: withTextOf(c.brand, lang, "subtitle", e.target.value) }))}
              aria-invalid={!!subtitleError}
            />
          </Field>
        </div>
        <Field label={t("settings.page.logo")} hint={t("settings.page.logoHint")} error={errors["config.brand.logo"]}>
          <Segmented
            label={t("settings.page.logo")}
            value={b.logo}
            onChange={(logo) => setBrand({ logo })}
            options={[
              { value: "letter", label: t("settings.page.logoLetter") },
              { value: "image", label: t("settings.page.logoImage") },
              { value: "emoji", label: t("settings.page.logoEmoji") },
              { value: "none", label: t("settings.page.logoNone") },
            ]}
          />
        </Field>
        {b.logo === "image" ? <ImageField name="logo" asset={logo} group={group} label={t("settings.page.logoFile")} hint={group ? t("settings.page.groupLogoHint") : t("settings.page.logoFileHint")} /> : null}
        {b.logo === "emoji" ? (
          <Field label={t("settings.page.emoji")} htmlFor="sp-emoji" hint={t("settings.page.emojiHint")} error={errors["config.brand.emoji"]}>
            <input id="sp-emoji" className="input max-w-[120px] text-center text-lg" value={b.emoji} maxLength={16} autoComplete="off" onChange={(e) => setBrand({ emoji: e.target.value.trim() })} aria-invalid={!!errors["config.brand.emoji"]} />
          </Field>
        ) : null}
      </Card>

      <Card title={t("settings.page.og")} sub={t("settings.page.ogSub")}>
        <Field label={t("settings.page.ogTitle")} htmlFor="sp-og-title" error={errors["config.og.title"]}>
          <input id="sp-og-title" className="input" value={og.title} maxLength={80} placeholder={name} onChange={(e) => setOG({ title: e.target.value })} aria-invalid={!!errors["config.og.title"]} />
        </Field>
        <Field label={t("settings.page.ogDescription")} htmlFor="sp-og-desc" error={errors["config.og.description"]}>
          <textarea id="sp-og-desc" className="input" rows={2} value={og.description} maxLength={200} onChange={(e) => setOG({ description: e.target.value.replace(/\n/g, " ") })} aria-invalid={!!errors["config.og.description"]} />
        </Field>
        <SwitchRow label={t("settings.page.ogImage")} sub={shownLogo ? t("settings.page.ogImageSub") : t("settings.page.ogImageNoLogo")} checked={og.image} disabled={!shownLogo && !og.image} onChange={(image) => setOG({ image })} />
        {errors["config.og.image"] ? (
          <p className="mb-3 text-xs text-[var(--berry-600)]" role="alert">
            {errors["config.og.image"]}
          </p>
        ) : null}
        <div className="text-xs text-[var(--ink-500)]">{t("settings.page.ogLook")}</div>
        {og.title || og.description || (og.image && shownLogo) ? (
          <div className="panel-soft mt-2 flex items-start gap-3 p-3" aria-hidden>
            <span className="w-0.5 self-stretch rounded-full bg-[var(--mikan-500)]" />
            <div className="min-w-0 flex-1">
              <div className="text-xs font-semibold text-[var(--mikan-700)]">{name}</div>
              <div className="truncate text-[13px] font-semibold">{og.title || name}</div>
              {og.description ? <div className="line-clamp-2 text-xs text-[var(--ink-600)]">{og.description}</div> : null}
            </div>
            {og.image && shownLogo ? <img src={imageURL(shownLogo)} alt="" className="h-12 w-12 shrink-0 rounded-[10px] object-cover" /> : null}
          </div>
        ) : (
          <p className="panel-soft mt-2 p-3 text-xs text-[var(--ink-500)]">{t("settings.page.ogNone")}</p>
        )}
      </Card>
    </>
  );
}
