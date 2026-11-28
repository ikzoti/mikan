// An own app of the subscription page, new or saved: its name, icon and core, the link that
// adds the subscription to it, its platforms and on each the line under its name, its download
// buttons and its instruction, the texts in the page's languages too. Like the built-in apps'
// buttons it is every page's and is saved at once (PUT /api/v1/sub-page/apps); each page orders
// and hides it as any other app.
import { Trash2 } from "lucide-react";
import { useRef, useState, type FormEvent, type ReactNode } from "react";
import { errorText, type Schemas } from "../../../../api/client";
import { useSubPageAppsMutation } from "../../../../api/hooks";
import { Confirm, Drawer } from "../../../../components/overlay";
import { Select } from "../../../../components/select";
import { useToast } from "../../../../components/toast";
import { Button, Field } from "../../../../components/ui";
import { t } from "../../../../i18n";
import { same } from "../../../../lib/draft";
import { fieldErrors } from "../../../../lib/fields";
import { appLink, linkProblem, MAX_LINK, type Platform } from "../../../../sub/apps";
import { PLATFORMS } from "../../../../sub/page";
import type { TextLang } from "../../../../sub/texts";
import { PLATFORM_NAMES, useSubDocs } from "../sub-docs";
import { ownAppOf, ownDraft, ownPrefix, scopedErrors, withOwnApp, withoutOwnApp, type OwnDraft, type OwnPlatformDraft } from "./app-edits";
import { DocField, Downloads, rowErrorOf, rowsOf, shownDoc, type Row } from "./app-fields";
import { IconPicker, iconReady, useIconFile, type IconFile } from "./app-icon";
import { SAMPLE_SUB_URL, useDiscardGuard } from "./parts";
import { downloadsLack, lacksText, textOf, textPath, withText } from "./text-edits";
import { NoTranslation, showRefusal, TextLangBar, textHint, textLabel } from "./text-lang";

type View = Schemas["SubPageAppsView"];
type Family = Schemas["OwnApp"]["family"];

const FAMILIES: readonly Family[] = ["mihomo", "xray", "singbox", "other"];
const PLACEHOLDERS = ["{url}", "{url_enc}", "{name}"] as const;

/** The server's limits of an own app (subpage.OwnApp). */
const MAX_NAME = 40;
const MAX_NOTE = 80;

/** What the dialog edits of the app itself. */
type AppFields = Pick<OwnDraft, "name" | "family" | "icon" | "link" | "on">;

/** What it edits of the app on a platform: the rows keep their keys while they move. */
type PlatformFields = { note: string; rows: Row[]; doc: number; i18n: OwnPlatformDraft["i18n"] };

type Docs = ReturnType<typeof useSubDocs>;

/** The form of own app id: what the dialog edits, the app it makes and whether that differs from the one saved. */
function useOwnForm(view: View, id: string, platform: Platform, docs: Docs) {
  const at = view.apps.own.findIndex((a) => a.id === id);
  const stored = at < 0 ? undefined : view.apps.own[at];
  const [start] = useState(() => ownDraft(stored, id, platform));
  const [app, setApp] = useState<AppFields>({ name: start.name, family: start.family, icon: start.icon, link: start.link, on: start.on });
  const [fields, setFields] = useState(
    () => Object.fromEntries(PLATFORMS.map((p) => [p, { note: start.platforms[p].note, rows: rowsOf(start.platforms[p].downloads), doc: start.platforms[p].doc, i18n: start.platforms[p].i18n }])) as Record<Platform, PlatformFields>,
  );
  const platforms = Object.fromEntries(PLATFORMS.map((p) => [p, { note: fields[p].note, downloads: fields[p].rows.map((r) => r.d), doc: shownDoc(docs, fields[p].doc), i18n: fields[p].i18n }])) as OwnDraft["platforms"];
  const next = ownAppOf({ id, ...app, platforms });
  return {
    stored,
    app,
    setApp,
    fields,
    setPlatform: (p: Platform, patch: Partial<PlatformFields>) => setFields((f) => ({ ...f, [p]: { ...f[p], ...patch } })),
    next,
    changed: !stored || !same(next, ownAppOf(ownDraft(stored, id, platform))),
    prefix: ownPrefix(view.apps, id),
  };
}

type Props = {
  view: View;
  id: string;
  platform: Platform;
  brand: string;
  /** The language of the texts the dialog opens in: the editor's, for an app saved before. */
  lang: TextLang;
  onClose: () => void;
};

export function OwnAppEditor({ view, id, platform, brand, lang: editorLang, onClose }: Props) {
  const toast = useToast();
  const docs = useSubDocs();
  const save = useSubPageAppsMutation();
  const picture = useIconFile(id);
  const { stored, app, setApp, fields, setPlatform, next, changed, prefix } = useOwnForm(view, id, platform, docs);
  // A new app gets its own texts first: they are what a language without a translation shows.
  const [lang, setLang] = useState<TextLang>(stored ? editorLang : "");
  const [file, setFile] = useState<IconFile>(null);
  // Each platform's rows in the order of the last save: the server names a refused one by its place.
  const [sent, setSent] = useState<Partial<Record<Platform, number[]>>>({});
  // Once a save was tried, what the app lacks is said at its fields as it is put right.
  const [tried, setTried] = useState(false);
  const dirty = changed || file !== null;
  const { close, dialog: leaveDialog } = useDiscardGuard(dirty, onClose, t("settings.page.ownLeaveText"));
  const errors = scopedErrors(fieldErrors(save.error), prefix);
  const ready = iconReady(app.icon, view.icons[id], file);
  const missing: { name?: string; platforms?: string; icon?: string } = tried ? { name: next.name ? undefined : t("errors.api.required"), platforms: app.on.length ? undefined : t("errors.api.platforms_required"), icon: ready ? "" : t("settings.page.appIconMissing") } : {};

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setSent(Object.fromEntries(PLATFORMS.map((p) => [p, fields[p].rows.map((r) => r.key)])));
    setTried(true);
    picture.reset();
    if (!next.name || !app.on.length || !ready) return;
    if (changed) {
      // The save reads the document again before it is sent: the server names the app's fields
      // by its place in that one.
      let sentPrefix = prefix;
      try {
        await save.mutateAsync((apps) => {
          sentPrefix = ownPrefix(apps, id);
          return withOwnApp(apps, next);
        });
      } catch (err) {
        showRefusal(err, scopedErrors(fieldErrors(err), sentPrefix), toast, setLang);
        return;
      }
    }
    if (!(await picture.send(file))) return;
    setFile(null);
    toast.ok(t(stored ? "settings.page.appSaved" : "settings.page.ownAdded", { name: next.name }));
    onClose();
  };
  const remove = () =>
    save.mutate((apps) => withoutOwnApp(apps, id), {
      onSuccess: () => {
        toast.ok(t("settings.page.ownRemoved", { name: stored?.name ?? "" }));
        onClose();
      },
      onError: (e) => toast.error(errorText(e)),
    });
  return (
    <>
      <Drawer
        open
        onOpenChange={(open) => !open && close()}
        title={stored?.name ?? t("settings.page.ownNew")}
        meta={<span className="tag">{t("settings.page.ownTag")}</span>}
        footer={
          <>
            <Button variant="ghost" onClick={close}>
              {t("common.cancel")}
            </Button>
            <Button variant="primary" type="submit" form="own-app-form" loading={save.isPending || picture.pending} disabled={!dirty}>
              {stored ? t("common.save") : t("settings.page.ownCreate")}
            </Button>
          </>
        }
      >
        <form id="own-app-form" onSubmit={(e) => void submit(e)} noValidate className="pt-4">
          <p className="text-[13px] text-[var(--ink-600)]">{t("settings.page.ownShared")}</p>
          <TextLangBar value={lang} onChange={setLang} className="mt-4" />
          <section className="dr-sec mt-4">
            <h3>{t("settings.page.appEveryPlatform")}</h3>
            <AppPart app={app} setApp={setApp} brand={brand} errors={errors} missing={missing}>
              <IconPicker
                id="own-icon"
                appId={id}
                name={app.name}
                icon={app.icon}
                setIcon={(icon) => setApp((a) => ({ ...a, icon }))}
                uploaded={view.icons[id]}
                file={file}
                setFile={setFile}
                error={missing.icon || picture.error || errors["icon.emoji"] || errors["icon.kind"]}
              />
            </AppPart>
          </section>
          {PLATFORMS.filter((p) => app.on.includes(p)).map((p) => (
            <PlatformPart key={p} platform={p} lang={lang} fields={fields[p]} set={(patch) => setPlatform(p, patch)} docs={docs} sent={sent[p] ?? []} errors={scopedErrors(errors, `platforms.${p}.`)} noLink={!next.link} />
          ))}
          {stored ? <RemoveOwn name={stored.name} pending={save.isPending} onRemove={remove} /> : null}
        </form>
      </Drawer>
      {leaveDialog}
    </>
  );
}

type AppPartProps = {
  app: AppFields;
  setApp: (f: (a: AppFields) => AppFields) => void;
  brand: string;
  errors: Record<string, string>;
  missing: { name?: string; platforms?: string };
  /** The icon's picker, which keeps a file of its own. */
  children: ReactNode;
};

/** The app's own fields, the same on every platform. */
function AppPart({ app, setApp, brand, errors, missing, children }: AppPartProps) {
  const set = (patch: Partial<AppFields>) => setApp((a) => ({ ...a, ...patch }));
  const nameError = missing.name ?? errors.name;
  return (
    <>
      <div className="grid gap-x-3 sm:grid-cols-2">
        <Field label={t("settings.page.ownName")} htmlFor="own-name" error={nameError}>
          <input id="own-name" className="input" value={app.name} maxLength={MAX_NAME} autoComplete="off" aria-invalid={!!nameError} onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <Field label={t("settings.page.ownFamily")} htmlFor="own-family" hint={t("settings.page.ownFamilyHint")} error={errors.family}>
          <Select id="own-family" className="input" value={app.family} aria-invalid={!!errors.family} onChange={(e) => set({ family: e.target.value as Family })}>
            {FAMILIES.map((f) => (
              <option key={f} value={f}>
                {t(`settings.subApps.family.${f}`)}
              </option>
            ))}
          </Select>
        </Field>
      </div>
      {children}
      <LinkField value={app.link} onChange={(link) => set({ link })} brand={brand} error={errors.link} />
      <PlatformPicks on={app.on} onChange={(on) => set({ on })} error={missing.platforms ?? errors.platforms} />
    </>
  );
}

/** The link that adds the subscription: the placeholders go in at the caret, and the page's link for a sample subscription shows under it, or why the link is refused. */
function LinkField({ value, onChange, brand, error }: { value: string; onChange: (v: string) => void; brand: string; error?: string }) {
  const input = useRef<HTMLInputElement>(null);
  const insert = (placeholder: string) => {
    const el = input.current;
    const from = el?.selectionStart ?? value.length;
    const to = el?.selectionEnd ?? value.length;
    onChange((value.slice(0, from) + placeholder + value.slice(to)).slice(0, MAX_LINK));
    window.requestAnimationFrame(() => {
      el?.focus();
      el?.setSelectionRange(from + placeholder.length, from + placeholder.length);
    });
  };
  const link = value.trim();
  const problem = link ? linkProblem(link) : null;
  const example = link && !problem ? appLink({ link, crypt: false }, { url: SAMPLE_SUB_URL, brand: brand.trim() || "VPN" }) : null;
  const refused = problem ?? (link && !example ? "app_link_invalid" : null);
  return (
    <div className="field">
      <label htmlFor="own-link">{t("settings.page.ownLink")}</label>
      <input
        ref={input}
        id="own-link"
        className="input mono"
        value={value}
        maxLength={MAX_LINK}
        placeholder="myapp://add/{url_enc}"
        spellCheck={false}
        autoComplete="off"
        inputMode="url"
        aria-invalid={!!refused || !!error}
        aria-describedby="own-link-check own-link-hint"
        onChange={(e) => onChange(e.target.value)}
      />
      <div className="flex flex-wrap gap-2" role="group" aria-label={t("settings.page.ownLinkInsert")}>
        {PLACEHOLDERS.map((p) => (
          <button key={p} type="button" className="chip-btn mono" onClick={() => insert(p)} aria-label={t("settings.page.ownLinkInsertOne", { value: p })}>
            {p}
          </button>
        ))}
      </div>
      <div id="own-link-check" className="panel-soft p-3 text-xs">
        {!link ? (
          <span className="text-[var(--ink-500)]">{t("settings.page.ownLinkNone")}</span>
        ) : refused ? (
          <span className="text-[var(--berry-600)]">{t(`errors.api.${refused}`)}</span>
        ) : (
          <>
            <span className="block text-[var(--ink-500)]">{t("settings.page.ownLinkExample")}</span>
            <code className="mono mt-1 block break-all text-[var(--ink-800)]">{example}</code>
          </>
        )}
      </div>
      {error && !refused ? (
        <span className="err" role="alert">
          {error}
        </span>
      ) : (
        <span className="hint" id="own-link-hint">
          {t("settings.page.ownLinkHint", { url: "{url}", url_enc: "{url_enc}", name: "{name}" })}
        </span>
      )}
    </div>
  );
}

/** The platforms the app is on: one at least. */
function PlatformPicks({ on, onChange, error }: { on: Platform[]; onChange: (on: Platform[]) => void; error?: string }) {
  return (
    <div className="field" role="group" aria-labelledby="own-platforms">
      <span className="lbl" id="own-platforms">
        {t("settings.page.ownPlatforms")}
      </span>
      <div className="flex flex-wrap gap-x-4 gap-y-2">
        {PLATFORMS.map((p) => (
          <label key={p} className="flex min-h-8 items-center gap-2 text-sm">
            <input type="checkbox" className="check shrink-0" checked={on.includes(p)} aria-invalid={!!error} onChange={(e) => onChange(PLATFORMS.filter((x) => (x === p ? e.target.checked : on.includes(x))))} />
            {PLATFORM_NAMES[p]}
          </label>
        ))}
      </div>
      {error ? (
        <span className="err" role="alert">
          {error}
        </span>
      ) : (
        <span className="hint">{t("settings.page.ownPlatformsHint")}</span>
      )}
    </div>
  );
}

type PlatformPartProps = {
  platform: Platform;
  /** The language its texts are edited in. */
  lang: TextLang;
  fields: PlatformFields;
  set: (patch: Partial<PlatformFields>) => void;
  docs: Docs;
  sent: number[];
  errors: Record<string, string>;
  /** The app has no link, so no "Add" either. */
  noLink: boolean;
};

/** The app on one platform: the line under its name, its download buttons and its instruction. */
function PlatformPart({ platform, lang, fields, set, docs, sent, errors, noLink }: PlatformPartProps) {
  const note = textOf(fields, lang, "note");
  const noteError = errors[textPath(lang, "note")];
  const untranslated = !!lang && (lacksText(fields, lang, "note") || downloadsLack(fields.rows.map((r) => r.d), lang));
  return (
    <section className="dr-sec">
      <h3>
        {PLATFORM_NAMES[platform]}
        {untranslated ? <NoTranslation /> : null}
      </h3>
      <Field label={textLabel(t("settings.page.ownNote"), lang)} htmlFor={`own-note-${platform}`} hint={lang ? textHint(fields.note, note) : t("settings.page.ownNoteHint")} error={noteError}>
        <input
          id={`own-note-${platform}`}
          className="input"
          value={note}
          maxLength={MAX_NOTE}
          placeholder={lang ? fields.note : undefined}
          autoComplete="off"
          aria-invalid={!!noteError}
          onChange={(e) => set(lang ? { i18n: withText(fields.i18n, lang, "note", e.target.value) } : { note: e.target.value })}
        />
      </Field>
      <Downloads
        idPrefix={`own-dl-${platform}`}
        rows={fields.rows}
        setRows={(rows) => set({ rows })}
        platform={platform}
        lang={lang}
        rowError={rowErrorOf(sent, errors)}
        listError={errors.downloads}
        emptyText={noLink ? t("settings.page.ownNoDownloadsText") : undefined}
      />
      <DocField id={`own-doc-${platform}`} docs={docs} value={shownDoc(docs, fields.doc)} onChange={(doc) => set({ doc })} error={errors.doc} />
    </section>
  );
}

/** "Delete app": from every page, with its icon, once asked. */
function RemoveOwn({ name, pending, onRemove }: { name: string; pending: boolean; onRemove: () => void }) {
  const [asking, setAsking] = useState(false);
  return (
    <>
      <div className="flex flex-wrap items-center gap-2 border-t border-[var(--hairline)] pt-4">
        <Button variant="danger" size="sm" disabled={pending} onClick={() => setAsking(true)}>
          <Trash2 size={16} aria-hidden /> {t("settings.page.ownRemove")}
        </Button>
        <span className="text-xs text-[var(--ink-500)]">{t("settings.page.ownRemoveHint")}</span>
      </div>
      <Confirm
        open={asking}
        onOpenChange={setAsking}
        title={t("settings.page.ownRemoveTitle", { name })}
        text={t("settings.page.ownRemoveText")}
        confirm={t("settings.page.ownRemoveConfirm")}
        danger
        loading={pending}
        onConfirm={onRemove}
      />
    </>
  );
}
