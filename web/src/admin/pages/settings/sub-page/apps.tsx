// The apps the page offers on each platform, and the QR code. Which apps show and in what order
// is the page's own (the draft, saved with the page); the apps themselves, the admin's own
// ones, the icons, where each is downloaded and its instruction are every page's, saved at once
// from the app's dialog.
import { Link } from "@tanstack/react-router";
import { ArrowDown, ArrowUp, Pencil, Plus } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { Schemas } from "../../../../api/client";
import { useBlockedAppIds, useSubPageApps } from "../../../../api/hooks";
import { QueryBoundary } from "../../../../components/query";
import { Select } from "../../../../components/select";
import { Switch, SwitchRow } from "../../../../components/switch";
import { Button, Field, Segmented, Skeleton } from "../../../../components/ui";
import { t } from "../../../../i18n";
import { appNote, type CatalogApp, type IconSource, type Platform } from "../../../../sub/apps";
import { orderApps, PLATFORMS } from "../../../../sub/page";
import { AppMark } from "../../../../sub/view";
import { PLATFORM_NAMES, useSubDocs } from "../sub-docs";
import { AppEditor } from "./app-editor";
import { newOwnId } from "./app-edits";
import { hiddenWith } from "./app-list";
import { OwnAppEditor } from "./own-app-editor";
import { Card, iconSourceOf, type Config, type LangProps, type Set } from "./parts";
import { focusMoved, moved } from "./reorder";
import { downloadsLack, lacksText } from "./text-edits";
import { NoTranslation, TextLangSwitch } from "./text-lang";

type AppList = Schemas["PageAppList"];
type View = Schemas["SubPageAppsView"];

/** How many apps of their own the admin may add (subpage.MaxOwnApps). */
const MAX_OWN = 20;

/** The dialog open: a built-in app's on a platform, or an own app's (a new one has an id no app has yet). */
type Editing = { kind: "preset" | "own"; id: string; platform: Platform };

type Props = {
  config: Config;
  set: Set;
  /** The fields the API refused of the page's last save. */
  errors: Record<string, string>;
  announce: (text: string) => void;
  platform: Platform;
  setPlatform: (p: Platform) => void;
  brand: string;
} & LangProps;

export function AppsSection({ config, set, errors, announce, platform, setPlatform, brand, lang, setLang }: Props) {
  const view = useSubPageApps();
  const blocked = new Set(useBlockedAppIds());
  const [editing, setEditing] = useState<Editing | null>(null);
  // A built-in app's dialog shows the app as the server has it now: after a save, as it gave it back.
  const preset = editing?.kind === "preset" ? view.data?.catalog[editing.platform].find((a) => a.id === editing.id) : undefined;
  // The button that opened the dialog takes the focus back once the dialog is gone: while it
  // is there, its focus trap would pull the focus in again. An app deleted has no button: the
  // focus goes to "Add own app" then.
  const opener = useRef("");
  useEffect(() => {
    if (editing || !opener.current) return;
    (document.getElementById(opener.current) ?? document.getElementById("own-add"))?.focus();
    opener.current = "";
  }, [editing]);
  const apps = config.apps;
  const setList = (next: Partial<AppList>) => set((c) => ({ ...c, apps: { ...c.apps, [platform]: { ...c.apps[platform], ...next } } }));
  const edit = (app: CatalogApp) => {
    opener.current = `app-edit-${app.id}`;
    setEditing({ kind: app.own ? "own" : "preset", id: app.id, platform });
  };
  const add = (v: View) => {
    opener.current = "own-add";
    setEditing({ kind: "own", id: newOwnId(v.apps.own.map((a) => a.id)), platform });
  };
  // An app lacks a translation of the admin's texts on this platform: an own app's note or a
  // button's text. A built-in app's buttons are the admin's once edited; the ones it comes with
  // are named after their sites.
  const untranslated = (v: View) => (a: CatalogApp) => {
    if (!lang) return false;
    if (a.own) return lacksText(a, lang, "note") || downloadsLack(a.downloads, lang);
    return downloadsLack(v.apps.presets[a.id]?.platforms?.[platform]?.downloads ?? [], lang);
  };
  return (
    <Card title={t("settings.page.apps")} sub={t("settings.page.appsSub")} aside={<TextLangSwitch value={lang} onChange={setLang} />}>
      <div className="grid gap-x-3 sm:grid-cols-2">
        <Field label={t("settings.page.appsDefault")} htmlFor="sp-platform" hint={t("settings.page.appsDefaultHint")}>
          <Select id="sp-platform" className="input" value={apps.platform} onChange={(e) => set((c) => ({ ...c, apps: { ...c.apps, platform: e.target.value as Config["apps"]["platform"] } }))}>
            <option value="auto">{t("settings.page.appsAuto")}</option>
            {PLATFORMS.map((p) => (
              <option key={p} value={p}>
                {PLATFORM_NAMES[p]}
              </option>
            ))}
          </Select>
        </Field>
        <SwitchRow label={t("settings.page.qr")} sub={t("settings.page.qrSub")} checked={apps.qr} onChange={(qr) => set((c) => ({ ...c, apps: { ...c.apps, qr } }))} />
      </div>
      <div className="mb-3 flex gap-1 overflow-x-auto" role="group" aria-label={t("sub.platform")}>
        <Segmented label={t("sub.platform")} value={platform} onChange={setPlatform} options={PLATFORMS.map((p) => ({ value: p, label: PLATFORM_NAMES[p]! }))} />
      </div>
      <QueryBoundary
        query={view}
        pending={
          <div className="flex flex-col gap-2" aria-busy>
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} style={{ height: 64, borderRadius: 16 }} />
            ))}
          </div>
        }
      >
        {(v) => (
          <>
            {v.unreadable ? (
              <p className="banner warn mb-3" role="status">
                {t("settings.page.appsUnreadable")}
              </p>
            ) : null}
            <AppRows
              catalog={v.catalog[platform]}
              list={apps[platform]}
              setList={setList}
              announce={announce}
              onEdit={edit}
              iconSrc={iconSourceOf(v.icons)}
              untranslated={untranslated(v)}
              blocked={blocked}
              error={errors[`config.apps.${platform}.order`] ?? errors[`config.apps.${platform}.hidden`]}
            />
            <AddOwn count={v.apps.own.length} onAdd={() => add(v)} />
          </>
        )}
      </QueryBoundary>
      {editing?.kind === "preset" && view.data && preset ? <AppEditor key={`${editing.platform}-${editing.id}`} view={view.data} app={preset} platform={editing.platform} lang={lang} onClose={() => setEditing(null)} /> : null}
      {editing?.kind === "own" && view.data ? <OwnAppEditor key={editing.id} view={view.data} id={editing.id} platform={editing.platform} brand={brand} lang={lang} onClose={() => setEditing(null)} /> : null}
    </Card>
  );
}

/** "Add own app", up to MAX_OWN of them. */
function AddOwn({ count, onAdd }: { count: number; onAdd: () => void }) {
  const full = count >= MAX_OWN;
  return (
    <div className="mt-3 flex flex-wrap items-center gap-2">
      <Button id="own-add" size="sm" disabled={full} onClick={onAdd}>
        <Plus size={16} aria-hidden /> {t("settings.page.ownAdd")}
      </Button>
      <span className="text-xs text-[var(--ink-500)]">{full ? t("settings.page.ownMax", { n: MAX_OWN }) : t("settings.page.ownAddHint")}</span>
    </div>
  );
}

type AppRowsProps = {
  catalog: CatalogApp[];
  list: AppList;
  setList: (next: Partial<AppList>) => void;
  announce: (text: string) => void;
  onEdit: (app: CatalogApp) => void;
  iconSrc: IconSource;
  /** Whether an app lacks a translation into the language picked. */
  untranslated: (app: CatalogApp) => boolean;
  /** The ids of the apps the page does not offer, turned away in the subscription's settings. */
  blocked: ReadonlySet<string>;
  /** What the API refused of this platform's lists. */
  error?: string;
};

/** A platform's apps of the catalogue, hidden ones too, in the order the page shows them. */
function AppRows({ catalog, list, setList, announce, onEdit, iconSrc, untranslated, blocked, error }: AppRowsProps) {
  const docs = useSubDocs();
  const all = orderApps(catalog, { order: list.order, hidden: [] });
  const ids = all.map((a) => a.id);
  const move = (i: number, step: -1 | 1) => {
    const id = ids[i]!;
    // The order holds the ids of the apps there are: an app deleted since leaves it now.
    setList({ order: moved(ids, i, step) });
    announce(t("settings.page.moved", { name: all[i]!.name, n: i + step + 1, total: ids.length }));
    focusMoved(step, `app-up-${id}`, `app-down-${id}`);
  };
  // What the app has on this platform beside its link: the buttons and the instruction, a
  // draft named so (the page leaves it out).
  const summary = (a: CatalogApp) => {
    const doc = docs.data?.find((d) => d.id === a.doc);
    const guide = doc ? t(doc.published ? "settings.page.appSummaryDoc" : "settings.page.appSummaryDraft", { name: doc.title }) : "";
    return [a.downloads.length ? t("settings.page.appSummaryDownloads", { n: a.downloads.length }) : "", guide].filter(Boolean).join(" · ");
  };
  const shownCount = all.filter((a) => !list.hidden.includes(a.id)).length;
  return (
    <>
      <ul className="flex flex-col gap-2">
        {all.map((a, i) => {
          const hidden = list.hidden.includes(a.id);
          const more = summary(a);
          const note = appNote(a);
          const turnedAway = blocked.has(a.id);
          return (
            <li key={a.id} className="panel-soft p-3">
              <div className="flex items-center gap-3">
                <Switch checked={!hidden} label={t("settings.page.showApp", { name: a.name })} onChange={(on) => setList({ hidden: hiddenWith(list.hidden, ids, a.id, on) })} />
                <div className={`flex min-w-0 flex-1 items-center gap-3 ${hidden ? "opacity-60" : ""}`}>
                  {/* On a phone the row keeps its width for the name. */}
                  <span className="hidden sm:block">
                    <AppMark app={a} src={iconSrc} />
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-[13px] font-semibold">{a.name}</div>
                    {note || a.own || turnedAway ? (
                      <div className="flex min-w-0 items-center gap-2 text-xs text-[var(--ink-500)]">
                        {a.own ? <span className="tag shrink-0">{t("settings.page.ownTag")}</span> : null}
                        {turnedAway ? <BlockedMark name={a.name} /> : null}
                        {note ? <span className="truncate">{note}</span> : null}
                      </div>
                    ) : null}
                    {more ? <div className="mt-1 text-xs text-[var(--ink-500)]">{more}</div> : null}
                    {a.own && !a.link ? <div className="mt-1 text-xs text-[var(--ink-500)]">{t("settings.page.ownNoLink")}</div> : null}
                  </div>
                </div>
                <button type="button" id={`app-edit-${a.id}`} className="icon-btn" onClick={() => onEdit(a)} aria-label={t("settings.page.appEdit", { name: a.name })} title={t("settings.page.appEdit", { name: a.name })}>
                  <Pencil size={16} />
                </button>
                <button type="button" id={`app-up-${a.id}`} className="icon-btn" disabled={i === 0} onClick={() => move(i, -1)} aria-label={t("settings.page.moveUp", { name: a.name })}>
                  <ArrowUp size={16} />
                </button>
                <button type="button" id={`app-down-${a.id}`} className="icon-btn" disabled={i === all.length - 1} onClick={() => move(i, 1)} aria-label={t("settings.page.moveDown", { name: a.name })}>
                  <ArrowDown size={16} />
                </button>
              </div>
              {/* A line of its own under the name, which a phone leaves no room beside. */}
              {untranslated(a) ? (
                <div className="mt-2 pl-13 sm:pl-26">
                  <NoTranslation />
                </div>
              ) : null}
            </li>
          );
        })}
      </ul>
      {error ? (
        <p className="mt-3 text-xs text-[var(--berry-600)]" role="alert">
          {error}
        </p>
      ) : (
        <p className="mt-3 text-xs text-[var(--ink-500)]">{shownCount === 0 ? t("settings.page.appsNone") : t("settings.page.appsNote")}</p>
      )}
    </>
  );
}

/** The mark of an app turned away in the subscription's settings: it leads there, where the app is allowed again. */
function BlockedMark({ name }: { name: string }) {
  const hint = t("settings.page.appBlockedHint", { name });
  return (
    <Link to="/settings" search={{ tab: "subscription", part: "clients" }} className="pill warn shrink-0 hover:underline hover:underline-offset-2" title={hint} aria-label={hint}>
      {t("settings.page.appBlocked")}
    </Link>
  );
}
