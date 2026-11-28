// What the sections of the subscription page's editor share: the page's types, its limits,
// the card each section is and how its dialogs close.
import { useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
import { basePath, type Schemas } from "../../../../api/client";
import { Card as LayoutCard } from "../../../../components/layout";
import { Confirm } from "../../../../components/overlay";
import { t } from "../../../../i18n";
import type { IconSource } from "../../../../sub/apps";
import type { TextLang } from "../../../../sub/texts";

export type View = Schemas["SubPageView"];

export type Config = Schemas["Page"];

export type Block = Schemas["PageBlock"];

export type Look = Schemas["PageLook"];

export type Asset = Schemas["PageAsset"];

/** The editor's draft: the page, and its brand's colour and name. On the panel's page they are the panel's (the name is edited in the general settings); on a group's own page the group's own, "" for the panel's. */
export type Form = { config: Config; brand_accent: string; brand: string };

export type Set = (f: (c: Config) => Config) => void;

export const COLOR = /^#[0-9A-Fa-f]{6}$/;

/** How big the page's images may be, in KB (subpage.AssetLimit): an app's icon is drawn 40 pixels wide. */
export const LIMITS = { logo: 512, background: 2048, icon: 128 } as const;

export const MAX_CUSTOM = 12;

export const MAX_CSS = 20480;

/** Where the admin sees the page's image a: the panel's, or a group's own (logo-g<id>, subpage.GroupAsset). */
export const imageURL = (a?: Asset) => {
  if (!a) return undefined;
  const own = /^(logo|background)-g(\d+)$/.exec(a.name);
  const path = own ? `groups/${own[2]}/page/images/${own[1]}` : `sub-page/images/${a.name}`;
  return `${basePath}/api/v1/${path}?v=${a.hash}`;
};

/** Where the admin sees app id's uploaded icon a. */
export const appIconURL = (id: string, a?: Asset) => (a ? `${basePath}/api/v1/sub-page/apps/${encodeURIComponent(id)}/icon?v=${a.hash}` : undefined);

/** The apps' icons in the editor's rows and preview: the ones uploaded, icons by app id. */
export const iconSourceOf =
  (icons: Record<string, Asset> | undefined): IconSource =>
  (a) =>
    appIconURL(a.id, icons?.[a.id]);

/**
 * The question before a dialog with unsaved changes closes: close asks first while dirty, and
 * dialog is the question, for the dialog to show beside itself; text says what would be lost.
 */
export function useDiscardGuard(dirty: boolean, onClose: () => void, text: string) {
  const [asking, setAsking] = useState(false);
  return {
    close: () => (dirty ? setAsking(true) : onClose()),
    dialog: (
      <Confirm
        open={asking}
        onOpenChange={setAsking}
        title={t("settings.page.docLeaveTitle")}
        text={text}
        confirm={t("settings.page.discard")}
        danger
        onConfirm={() => {
          setAsking(false);
          onClose();
        }}
      />
    ),
  };
}

/** The subscription address the preview and the editor's examples show. */
export const SAMPLE_SUB_URL = "https://vpn.example.com/sub/k3J9fQ2mXa7d";

/** Why the page would not take an image file (its errors.api text), "" when it would: PNG, JPEG or WebP of limit KB at most. */
export function imageProblem(file: File, limit: number): string {
  if (!["image/png", "image/jpeg", "image/webp"].includes(file.type)) return t("errors.api.image_type");
  return file.size > limit * 1024 ? t("errors.api.image_too_big", { value: limit }) : "";
}

/** The file as base64, as the API takes images. */
export function readBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(String(r.result).replace(/^data:[^,]*,/, ""));
    r.onerror = () => reject(new Error("read"));
    r.readAsDataURL(file);
  });
}

export type SectionProps = { draft: Form; setDraft: Dispatch<SetStateAction<Form>>; set: Set; errors: Record<string, string> };

/** The language the editor edits the admin's texts in, one for all its sections and the preview. */
export type LangProps = { lang: TextLang; setLang: (lang: TextLang) => void };

// The editor's cards come and go with its sections: no entrance.
export function Card({ title, sub, aside, children }: { title: string; sub?: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <LayoutCard title={title} sub={sub} aside={aside} reveal={false}>
      {children}
    </LayoutCard>
  );
}
