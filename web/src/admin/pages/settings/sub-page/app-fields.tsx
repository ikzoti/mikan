// The fields of an app's dialogs on one platform, a built-in app's or an own one's: its
// download buttons, their texts in the page's languages, and its instruction. An own app's
// dialog holds one set a platform, so every id here takes the set's own prefix.
import { ArrowDown, ArrowUp, Download, Plus, Trash2 } from "lucide-react";
import { useRef, useState } from "react";
import { Select } from "../../../../components/select";
import { Button, Field, Skeleton } from "../../../../components/ui";
import { t } from "../../../../i18n";
import { downloadLabel, type AppDownload, type Platform } from "../../../../sub/apps";
import type { TextLang } from "../../../../sub/texts";
import type { useSubDocs } from "../sub-docs";
import { focusMoved, moved } from "./reorder";
import { textOf, textPath, withTextOf } from "./text-edits";
import { textLabel } from "./text-lang";

type Kind = AppDownload["kind"];

const KINDS: readonly Kind[] = ["appstore", "googleplay", "github", "mirror", "link"];

/** The server's limits of a platform's downloads (subpage.MaxDownloads, the link and its text). */
const MAX_DOWNLOADS = 5;
const MAX_URL = 500;
const MAX_LABEL = 40;

/** Where a new button leads first: the platform's own store, GitHub on the desktop. */
const FIRST_KIND: Record<Platform, Kind> = { ios: "appstore", macos: "appstore", android: "googleplay", windows: "github", linux: "github" };

const kindName = (k: Kind) => (k === "link" ? t("settings.page.appKindLink") : downloadLabel({ kind: k, url: "" }));

/** A button's name in the editor's words: its own text, or its kind's. */
const rowName = (d: AppDownload) => d.label?.trim() || kindName(d.kind);

/** A download button being edited, keyed by itself: a moved row takes its fields and focus along. */
export type Row = { key: number; d: AppDownload };

/** The rows of downloads, keyed from 0. */
export const rowsOf = (list: readonly AppDownload[]): Row[] => list.map((d, key) => ({ key, d }));

/** The API's error of a row's field, errors under the platform's fields: the server names a row by its place in the last save, sent (the rows' keys then), and its error stays with it when the rows move. */
export const rowErrorOf =
  (sent: readonly number[], errors: Record<string, string>) =>
  (key: number, field: string): string | undefined => {
    const i = sent.indexOf(key);
    return i < 0 ? undefined : errors[`downloads[${i}].${field}`];
  };

/** The download buttons: kind, link and text of each (in lang, its translation), in the order the page shows them. idPrefix keeps the ids of one set apart from another's. */
export function Downloads({
  idPrefix,
  rows,
  setRows,
  platform,
  lang,
  rowError,
  listError,
  emptyText = t("settings.page.appNoDownloadsText"),
}: {
  idPrefix: string;
  rows: Row[];
  setRows: (rows: Row[]) => void;
  platform: Platform;
  lang: TextLang;
  rowError: (key: number, field: string) => string | undefined;
  listError?: string;
  /** What the page shows of the app with no buttons. */
  emptyText?: string;
}) {
  const [live, setLive] = useState("");
  // A new row's key is one no row had: a removed row's error, named by its key, stays with it.
  const nextKey = useRef(Math.max(-1, ...rows.map((r) => r.key)) + 1);
  const id = (what: string, key?: number) => `${idPrefix}-${what}${key === undefined ? "" : `-${key}`}`;
  const setRow = (i: number, patch: Partial<AppDownload>) => setRows(rows.map((r, j) => (j === i ? { ...r, d: { ...r.d, ...patch } } : r)));
  const move = (i: number, step: -1 | 1) => {
    const r = rows[i]!;
    setRows(moved(rows, i, step));
    setLive(t("settings.page.moved", { name: rowName(r.d), n: i + step + 1, total: rows.length }));
    focusMoved(step, id("up", r.key), id("down", r.key));
  };
  const remove = (i: number) => {
    const next = rows.filter((_, j) => j !== i);
    setRows(next);
    setLive(t("settings.page.appDownloadRemoved", { name: rowName(rows[i]!.d) }));
    const after = next[Math.min(i, next.length - 1)];
    window.requestAnimationFrame(() => document.getElementById(after ? id("remove", after.key) : id("add"))?.focus());
  };
  const add = () => {
    const key = nextKey.current++;
    setRows([...rows, { key, d: { kind: FIRST_KIND[platform], url: "https://" } }]);
    window.requestAnimationFrame(() => document.getElementById(id("url", key))?.focus());
  };
  const full = rows.length >= MAX_DOWNLOADS;
  return (
    <div className="field" role="group" aria-labelledby={id("title")}>
      <span className="lbl" id={id("title")}>
        {t("settings.page.appDownloads")}
      </span>
      {rows.length ? (
        <ul className="flex flex-col gap-2">
          {rows.map((r, i) => (
            <DownloadRow key={r.key} id={(what) => id(what, r.key)} row={r} lang={lang} first={i === 0} last={i === rows.length - 1} at={(field) => rowError(r.key, field)} set={(patch) => setRow(i, patch)} move={(step) => move(i, step)} remove={() => remove(i)} />
          ))}
        </ul>
      ) : (
        <div className="panel-soft flex flex-col items-center gap-2 px-6 py-6 text-center">
          <Download size={20} className="text-[var(--ink-400)]" aria-hidden />
          <div className="text-[13px] font-semibold">{t("settings.page.appNoDownloads")}</div>
          <p className="max-w-[320px] text-xs text-[var(--ink-500)]">{emptyText}</p>
        </div>
      )}
      {listError ? (
        <p className="text-xs text-[var(--berry-600)]" role="alert">
          {listError}
        </p>
      ) : null}
      <div className="flex flex-wrap items-center gap-2">
        <button type="button" id={id("add")} className="chip-btn inline-flex items-center gap-1" onClick={add} disabled={full}>
          <Plus size={14} aria-hidden /> {t("settings.page.addLink")}
        </button>
        <span className="text-xs text-[var(--ink-500)]">{full ? t("settings.page.appDownloadsMax", { n: MAX_DOWNLOADS }) : t("settings.page.appDownloadsHint")}</span>
      </div>
      <p className="sr-only" role="status" aria-live="polite">
        {live}
      </p>
    </div>
  );
}

function DownloadRow({
  id,
  row: { d },
  lang,
  first,
  last,
  at,
  set,
  move,
  remove,
}: {
  id: (what: string) => string;
  row: Row;
  lang: TextLang;
  first: boolean;
  last: boolean;
  at: (field: string) => string | undefined;
  set: (patch: Partial<AppDownload>) => void;
  move: (step: -1 | 1) => void;
  remove: () => void;
}) {
  const name = rowName(d);
  const label = textPath(lang, "label");
  const error = at("kind") ?? at("url") ?? at(label);
  // The row's error names the fields it is about.
  const note = (field: string) => (at(field) ? id("err") : undefined);
  return (
    <li className="panel-soft p-3">
      <div className="flex items-center gap-1">
        <Select className="input min-w-0 flex-1" aria-label={t("settings.page.appKind")} aria-invalid={!!at("kind")} aria-describedby={note("kind")} value={d.kind} onChange={(e) => set({ kind: e.target.value as Kind })}>
          {KINDS.map((k) => (
            <option key={k} value={k}>
              {kindName(k)}
            </option>
          ))}
        </Select>
        <button type="button" id={id("up")} className="icon-btn" disabled={first} onClick={() => move(-1)} aria-label={t("settings.page.moveUp", { name })}>
          <ArrowUp size={16} />
        </button>
        <button type="button" id={id("down")} className="icon-btn" disabled={last} onClick={() => move(1)} aria-label={t("settings.page.moveDown", { name })}>
          <ArrowDown size={16} />
        </button>
        <button type="button" id={id("remove")} className="icon-btn" onClick={remove} aria-label={t("settings.page.linkRemove", { name })}>
          <Trash2 size={16} />
        </button>
      </div>
      <input
        id={id("url")}
        className="input mono mt-2"
        type="url"
        inputMode="url"
        value={d.url}
        maxLength={MAX_URL}
        placeholder="https://"
        spellCheck={false}
        autoComplete="off"
        aria-label={t("settings.page.linkURL")}
        aria-invalid={!!at("url")}
        aria-describedby={note("url")}
        onChange={(e) => set({ url: e.target.value })}
      />
      <input
        className="input mt-2"
        value={textOf(d, lang, "label")}
        maxLength={MAX_LABEL}
        placeholder={lang ? d.label?.trim() || kindName(d.kind) : d.kind === "link" ? t("settings.page.appLabelRequired") : kindName(d.kind)}
        autoComplete="off"
        aria-label={textLabel(t("settings.page.linkLabel"), lang)}
        aria-invalid={!!at(label)}
        aria-describedby={note(label)}
        onChange={(e) => set(withTextOf(d, lang, "label", e.target.value))}
      />
      {error ? (
        <p id={id("err")} className="mt-2 text-xs text-[var(--berry-600)]" role="alert">
          {error}
        </p>
      ) : null}
    </li>
  );
}

/** The instruction the app's "Guide" opens: a published one, a draft (shown once published) or none. */
export function DocField({ id, docs, value, onChange, error }: { id: string; docs: ReturnType<typeof useSubDocs>; value: number; onChange: (id: number) => void; error?: string }) {
  if (!docs.data) {
    return (
      <Field label={t("settings.page.appDoc")}>
        {docs.isError ? (
          <div className="flex flex-wrap items-center gap-2 text-[13px] text-[var(--ink-600)]" role="alert">
            {t("common.loadFailed")}
            <Button size="sm" variant="ghost" loading={docs.isFetching} onClick={() => void docs.refetch()}>
              {t("common.retry")}
            </Button>
          </div>
        ) : (
          <Skeleton style={{ height: 44, borderRadius: 12 }} />
        )}
      </Field>
    );
  }
  const list = docs.data;
  return (
    <Field label={t("settings.page.appDoc")} htmlFor={id} hint={list.length ? t("settings.page.appDocHint") : t("settings.page.appDocEmpty")} error={error}>
      <Select id={id} className="input" value={String(value)} aria-invalid={!!error} onChange={(e) => onChange(Number(e.target.value))}>
        <option value="0">{t("settings.page.appDocNone")}</option>
        {list.map((d) => (
          <option key={d.id} value={String(d.id)}>
            {[d.emoji, d.title, d.published ? "" : `· ${t("settings.page.docDraft")}`].filter(Boolean).join(" ")}
          </option>
        ))}
      </Select>
    </Field>
  );
}

/** A doc an app links to, as its dialog sends it back: one deleted since is none. Until the list is here, the app's own goes back as it came. */
export function shownDoc(docs: ReturnType<typeof useSubDocs>, doc: number): number {
  return !docs.data || docs.data.some((d) => d.id === doc) ? doc : 0;
}
