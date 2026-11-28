// One built-in app of the subscription page: its icon, on every platform, and on one platform
// its download buttons, their texts in the page's languages, and its instruction. They are
// every page's, the groups' pages too, and are saved at once (PUT /api/v1/sub-page/apps), apart
// from the page's draft; a picture picked for the icon is uploaded after them.
import { RotateCcw } from "lucide-react";
import { useState, type FormEvent } from "react";
import { errorText, type Schemas } from "../../../../api/client";
import { useSubPageAppsMutation } from "../../../../api/hooks";
import { Confirm, Drawer } from "../../../../components/overlay";
import { useToast } from "../../../../components/toast";
import { Button } from "../../../../components/ui";
import { t } from "../../../../i18n";
import { same } from "../../../../lib/draft";
import { fieldErrors } from "../../../../lib/fields";
import type { CatalogApp, Platform } from "../../../../sub/apps";
import type { TextLang } from "../../../../sub/texts";
import { PLATFORM_NAMES, useSubDocs } from "../sub-docs";
import { cleanDownloads, iconOf, scopedErrors, withAppEdit, withPresetIcon } from "./app-edits";
import { DocField, Downloads, rowErrorOf, rowsOf, shownDoc, type Row } from "./app-fields";
import { IconPicker, iconReady, useIconFile, type IconFile } from "./app-icon";
import { useDiscardGuard } from "./parts";
import { downloadsLack } from "./text-edits";
import { NoTranslation, showRefusal, TextLangBar } from "./text-lang";

type View = Schemas["SubPageAppsView"];
type AppIcon = Schemas["AppIcon"];

const LETTER: AppIcon = { kind: "letter" };

type Props = {
  view: View;
  /** The app on platform as the server has it now: after a save, the catalogue it gave back. */
  app: CatalogApp;
  platform: Platform;
  /** The language of the texts the dialog opens in: the editor's. */
  lang: TextLang;
  onClose: () => void;
};

export function AppEditor({ view, app, platform, lang: editorLang, onClose }: Props) {
  const toast = useToast();
  const docs = useSubDocs();
  const save = useSubPageAppsMutation();
  const [lang, setLang] = useState(editorLang);
  const savedIcon = view.apps.presets[app.id]?.icon ?? LETTER;
  const [icon, setIcon] = useState<AppIcon>(savedIcon);
  const [file, setFile] = useState<IconFile>(null);
  const picture = useIconFile(app.id);
  const [rows, setRows] = useState<Row[]>(() => rowsOf(app.downloads));
  const [doc, setDoc] = useState(app.doc);
  // The rows in the order of the last save: the server names a refused one by its place then,
  // and its error stays with it when the rows move.
  const [sent, setSent] = useState<number[]>([]);
  // Once a save was tried, an image icon with no picture is said at the picker as it is put right.
  const [tried, setTried] = useState(false);
  const preset = view.presets[platform].find((a) => a.id === app.id)?.downloads ?? [];
  const edited = !!view.apps.presets[app.id]?.platforms?.[platform];
  const downloads = rows.map((r) => r.d);
  // The built-in buttons' texts are names (happ.info, F-Droid): only the admin's own call for a translation.
  const untranslated = !!lang && !same(cleanDownloads(downloads), cleanDownloads(preset)) && downloadsLack(downloads, lang);
  const docShown = shownDoc(docs, doc);
  const changed = !same(iconOf(icon), iconOf(savedIcon)) || docShown !== app.doc || !same(cleanDownloads(downloads), cleanDownloads(app.downloads));
  const dirty = changed || file !== null;
  const { close, dialog: leaveDialog } = useDiscardGuard(dirty, onClose, t("settings.page.appLeaveText"));
  const prefix = `presets.${app.id}.`;
  const errors = scopedErrors(fieldErrors(save.error), prefix);
  const here = scopedErrors(errors, `platforms.${platform}.`);
  const ready = iconReady(icon, view.icons[app.id], file);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setSent(rows.map((r) => r.key));
    setTried(true);
    picture.reset();
    if (!ready) return;
    if (changed) {
      try {
        await save.mutateAsync((apps) => withPresetIcon(withAppEdit(apps, app.id, platform, { downloads, doc: docShown }, preset), app.id, icon));
      } catch (err) {
        showRefusal(err, scopedErrors(fieldErrors(err), prefix), toast, setLang);
        return;
      }
    }
    if (!(await picture.send(file))) return;
    setFile(null);
    toast.ok(t("settings.page.appSaved", { name: app.name }));
    onClose();
  };
  const reset = () =>
    save.mutate((apps) => withAppEdit(apps, app.id, platform, null, preset), {
      onSuccess: () => {
        toast.ok(t("settings.page.appResetDone", { name: app.name }));
        onClose();
      },
      onError: (e) => toast.error(errorText(e)),
    });
  const busy = save.isPending || picture.pending;
  return (
    <>
      <Drawer
        open
        onOpenChange={(open) => !open && close()}
        title={app.name}
        meta={
          <>
            <span>{PLATFORM_NAMES[platform]}</span>
            {edited ? <span className="tag">{t("settings.page.appEdited")}</span> : null}
          </>
        }
        footer={
          <>
            <Button variant="ghost" onClick={close}>
              {t("common.cancel")}
            </Button>
            <Button variant="primary" type="submit" form="app-edit-form" loading={busy} disabled={!dirty}>
              {t("common.save")}
            </Button>
          </>
        }
      >
        <form id="app-edit-form" onSubmit={(e) => void submit(e)} noValidate className="pt-4">
          <p className="text-[13px] text-[var(--ink-600)]">{t("settings.page.appShared")}</p>
          <TextLangBar value={lang} onChange={setLang} className="mt-4" />
          <section className="dr-sec mt-4">
            <h3>{t("settings.page.appEveryPlatform")}</h3>
            <IconPicker
              id="app-icon"
              appId={app.id}
              name={app.name}
              icon={icon}
              setIcon={setIcon}
              uploaded={view.icons[app.id]}
              file={file}
              setFile={setFile}
              error={(tried && !ready ? t("settings.page.appIconMissing") : "") || picture.error || errors["icon.emoji"] || errors["icon.kind"]}
            />
          </section>
          <section className="dr-sec">
            <h3>
              {PLATFORM_NAMES[platform]}
              {untranslated ? <NoTranslation /> : null}
            </h3>
            <Downloads idPrefix="dl" rows={rows} setRows={setRows} platform={platform} lang={lang} rowError={rowErrorOf(sent, here)} listError={here.downloads} />
            <DocField id="app-doc" docs={docs} value={docShown} onChange={setDoc} error={here.doc} />
          </section>
          {edited ? <ResetApp app={app} platform={platform} pending={save.isPending} onReset={reset} /> : null}
        </form>
      </Drawer>
      {leaveDialog}
    </>
  );
}

/** "Back to built-in": the app on this platform as it comes, on every page, once asked. */
function ResetApp({ app, platform, pending, onReset }: { app: CatalogApp; platform: Platform; pending: boolean; onReset: () => void }) {
  const [asking, setAsking] = useState(false);
  return (
    <>
      <div className="flex flex-wrap items-center gap-2 border-t border-[var(--hairline)] pt-4">
        <Button variant="ghost" size="sm" disabled={pending} onClick={() => setAsking(true)}>
          <RotateCcw size={16} aria-hidden /> {t("settings.page.appReset")}
        </Button>
        <span className="text-xs text-[var(--ink-500)]">{t("settings.page.appResetHint")}</span>
      </div>
      <Confirm
        open={asking}
        onOpenChange={setAsking}
        title={t("settings.page.appResetTitle", { name: app.name, platform: PLATFORM_NAMES[platform] ?? platform })}
        text={t("settings.page.appResetText")}
        confirm={t("settings.page.resetConfirm")}
        danger
        loading={pending}
        onConfirm={onReset}
      />
    </>
  );
}
