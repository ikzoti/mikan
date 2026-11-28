import { useId, useState } from "react";
import type { Schemas } from "../../api/client";
import { useFolders, useGroups, useTariffs, useUsersPages } from "../../api/hooks";
import { Button, Field } from "../../components/ui";
import { t } from "../../i18n";
import { useDebounced } from "../../lib/debounce";
import { same } from "../../lib/draft";
import { coveringDomain, sameDestination } from "./filter-destinations";

type Rule = Schemas["AudienceRule"];
type Scope = Rule["scope"] | "all";
const audiences = ["users", "folders", "groups", "tariffs"] as const;
type Audience = (typeof audiences)[number];
type Editable = Omit<Rule, "scope"> & { scope: Scope };

export function FilterRules({ rules, globalValues, userNames, onSave, onRemove, error }: {
  rules: Rule[];
  userNames: Record<string, string>;
  globalValues: Record<Rule["kind"], string[]>;
  onSave: (index: number, rule: Editable) => void;
  onRemove: (index: number) => void;
  error?: string;
}) {
  const [editing, setEditing] = useState<{ index: number; rules: Rule[] } | null>(null);
  const [pickedNames, setPickedNames] = useState<Record<string, string>>({});
  if (editing && !same(editing.rules, rules)) setEditing(null);
  const open = (index: number) => setEditing({ index, rules });
  return (
    <div className="mt-5 border-t border-[var(--line)] pt-4">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="text-sm font-medium">{t("filters.rules.title")}</h3>
        {editing === null && <Button type="button" size="sm" variant="ghost" onClick={() => open(-1)}>{t("filters.rules.add")}</Button>}
      </div>
      {error && <p className="banner err" role="alert">{error}</p>}
      {rules.length > 0 && (
        <ul className="mb-3 space-y-2">
          {rules.map((rule, i) => {
            const shadowed = rule.kind === "domain" ? coveringDomain(rule.value, globalValues.domain) : undefined;
            return <li key={i} className="panel-soft flex flex-wrap items-center gap-2 p-2">
              <span className="min-w-0 flex-1 break-all text-sm">
                {t(`filters.rules.${rule.kind}`)} <span className="mono">{rule.value}</span>
                <span className="ml-2 text-xs text-[var(--ink-600)]">{t(`filters.rules.${rule.scope}`)}</span>
                {shadowed && <span className="block text-xs text-[var(--ink-600)]">{t("filters.rules.shadowed", { value: shadowed })}</span>}
              </span>
              <Button type="button" size="sm" variant="ghost" disabled={editing !== null} onClick={() => open(i)}>{t("filters.rules.edit")}</Button>
              <Button type="button" size="sm" variant="ghost" disabled={editing !== null} onClick={() => onRemove(i)}>{t("filters.rules.remove")}</Button>
            </li>;
          })}
        </ul>
      )}
      {editing !== null && (
        <RuleEditor
          initial={rules[editing.index]}
          globalValues={globalValues}
          userNames={{ ...userNames, ...pickedNames }}
          rememberUser={(id, name) => setPickedNames((names) => ({ ...names, [id]: name }))}
          close={() => setEditing(null)}
          save={(rule) => {
            onSave(editing.index, rule);
            setEditing(null);
          }}
        />
      )}
    </div>
  );
}

function RuleEditor({ initial, globalValues, userNames, rememberUser, close, save }: {
  initial?: Rule;
  userNames: Record<string, string>;
  globalValues: Record<Rule["kind"], string[]>;
  rememberUser: (id: number, name: string) => void;
  close: () => void;
  save: (rule: Editable) => void;
}) {
  const id = useId();
  const [rule, setRule] = useState<Editable>(initial ?? { kind: "domain", value: "", scope: "all" });
  const hasAudience = audiences.some((key) => rule[key]?.length);
  const shadowed = rule.scope !== "all" && rule.kind === "domain"
    ? coveringDomain(rule.value, globalValues.domain.filter((value) => !sameDestination("domain", value, rule.value)))
    : undefined;
  return (
    <div className="panel-soft p-3">
      <div className="grid gap-2 sm:grid-cols-2">
        <Field label={t("filters.rules.kind")} htmlFor={id + "-kind"}>
          <select id={id + "-kind"} className="input" value={rule.kind} onChange={(e) => setRule({ ...rule, kind: e.target.value as Rule["kind"] })}>
            {(["domain", "network", "port"] as const).map((kind) => <option key={kind} value={kind}>{t(`filters.rules.${kind}`)}</option>)}
          </select>
        </Field>
        <Field label={t("filters.rules.value")} htmlFor={id + "-value"}>
          <input id={id + "-value"} className="input mono" spellCheck={false} list={id + "-values"} value={rule.value} onChange={(e) => setRule({ ...rule, value: e.target.value })} />
          <datalist id={id + "-values"}>
            {[...new Set(globalValues[rule.kind])].map((value) => <option key={value} value={value} />)}
          </datalist>
        </Field>
      </div>
      <Field label={t("filters.rules.audience")} htmlFor={id + "-scope"}>
        <select id={id + "-scope"} className="input" value={rule.scope} onChange={(e) => setRule({ ...rule, scope: e.target.value as Scope })}>
          {(["all", "only", "except"] as const).map((scope) => <option key={scope} value={scope}>{t(`filters.rules.${scope}`)}</option>)}
        </select>
      </Field>
      {shadowed && <p className="mb-2 text-xs text-[var(--ink-600)]">{t("filters.rules.shadowed", { value: shadowed })}</p>}
      {rule.scope !== "all" && <AudiencePickers rule={rule} userNames={userNames} rememberUser={rememberUser} set={(k, ids) => setRule({ ...rule, [k]: ids })} />}
      <div className="mt-3 flex gap-2">
        <Button type="button" size="sm" disabled={!rule.value.trim() || (rule.scope !== "all" && !hasAudience)} onClick={() => save({ ...rule, value: rule.value.trim() })}>
          {t("filters.rules.apply")}
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={close}>{t("filters.rules.cancel")}</Button>
      </div>
    </div>
  );
}

function AudiencePickers({ rule, userNames, rememberUser, set }: {
  rule: Editable;
  userNames: Record<string, string>;
  rememberUser: (id: number, name: string) => void;
  set: (key: Audience, ids: number[]) => void;
}) {
  const [q, setQ] = useState("");
  const folders = useFolders();
  const tariffs = useTariffs();
  const groups = useGroups();
  const query = useDebounced(q.trim());
  const users = useUsersPages({ state: "all", q: query, hidden: "show" }, { limit: 50, poll: false });
  const id = useId();
  const options = {
    users: users.data?.pages.flatMap((page) => page.items) ?? [],
    folders: folders.data ?? [],
    groups: groups.data ?? [],
    tariffs: tariffs.data ?? [],
  };
  const failed = { users: users.isError, folders: folders.isError, groups: groups.isError, tariffs: tariffs.isError };
  const pending = { users: users.isPending || users.isPlaceholderData || query !== q.trim(), folders: folders.isPending, groups: groups.isPending, tariffs: tariffs.isPending };

  return (
    <div className="space-y-2">
      <Field label={t("filters.rules.search")} htmlFor={id + "-search"}>
        <input id={id + "-search"} className="input" type="search" value={q} onChange={(e) => setQ(e.target.value)} />
      </Field>
      {audiences.map((key) => {
        const ids = rule[key] ?? [];
        const choices = options[key];
        const available = choices.filter((item) => !ids.includes(item.id));
        return (
          <Field key={key} label={t(`filters.rules.${key}`)} htmlFor={id + key} error={failed[key] ? t("filters.rules.loadError") : undefined}>
            <select id={id + key} className="input" value="" disabled={pending[key] || failed[key]} onChange={(e) => {
              const item = choices.find((choice) => choice.id === Number(e.target.value));
              if (!item) return;
              if (key === "users") rememberUser(item.id, item.name);
              set(key, [...ids, item.id]);
            }}>
              <option value="" disabled>{t("filters.rules.choose")}</option>
              {available.map((item) => <option key={item.id} value={item.id}>{item.name} · #{item.id}</option>)}
            </select>
            {key === "users" && users.hasNextPage && !pending.users && (
              <Button type="button" size="sm" variant="ghost" loading={users.isFetchingNextPage} onClick={() => void users.fetchNextPage()}>
                {t("filters.rules.more")}
              </Button>
            )}
            {ids.length > 0 && (
              <div className="mt-1 flex flex-wrap gap-1">
                {ids.map((picked) => {
                  const name = choices.find((item) => item.id === picked)?.name ?? (key === "users" ? userNames[String(picked)] : undefined) ?? `#${picked}`;
                  return (
                    <Button key={picked} type="button" size="sm" variant="ghost" onClick={() => set(key, ids.filter((other) => picked !== other))} aria-label={t("filters.rules.removeName", { name })}>
                      {name} ×
                    </Button>
                  );
                })}
              </div>
            )}
          </Field>
        );
      })}
    </div>
  );
}
