// An app's icon in its dialog: the first letter of its name, an emoji or a picture. A picture
// picked here goes with the dialog's save, after the app itself: an own app takes one only
// once it is saved.
import { ImageUp } from "lucide-react";
import { useRef, useState } from "react";
import { errorText, type Schemas } from "../../../../api/client";
import { useAppIconMutation } from "../../../../api/hooks";
import { Button, Field, Segmented } from "../../../../components/ui";
import { t } from "../../../../i18n";
import { fieldErrors } from "../../../../lib/fields";
import { AppMark } from "../../../../sub/view";
import { appIconURL, imageProblem, LIMITS, readBase64, type Asset } from "./parts";

type AppIcon = Schemas["AppIcon"];

/** A picture picked in the dialog, to upload on save, or "remove" for the uploaded one to go; null leaves it as it is. */
export type IconFile = { data: string; url: string } | "remove" | null;

/** The picture the dialog shows: the one picked, or the uploaded one unless it goes. */
function pictureOf(appId: string, uploaded: Asset | undefined, file: IconFile): string | undefined {
  if (file === "remove") return undefined;
  return file?.url ?? appIconURL(appId, uploaded);
}

/** A dialog's picture of app id, sent once the app is saved, with what went wrong for the picker to say. */
export function useIconFile(appId: string) {
  const mutation = useAppIconMutation();
  const [error, setError] = useState("");
  return {
    error,
    pending: mutation.isPending,
    reset: () => setError(""),
    /** Uploads the picture picked, or takes the uploaded one away; false, the reason kept, when the server refused. */
    send: async (file: IconFile): Promise<boolean> => {
      if (file === null) return true;
      try {
        await mutation.mutateAsync({ id: appId, data: file === "remove" ? undefined : file.data });
        return true;
      } catch (err) {
        setError(fieldErrors(err).data ?? errorText(err));
        return false;
      }
    },
  };
}

/** Whether the page can draw the icon once saved: an image icon has its picture. */
export function iconReady(icon: AppIcon, uploaded: Asset | undefined, file: IconFile): boolean {
  if (icon.kind !== "image") return true;
  return file !== "remove" && (file !== null || uploaded !== undefined);
}

type IconPickerProps = {
  /** The id the picker's fields are named by. */
  id: string;
  appId: string;
  name: string;
  icon: AppIcon;
  setIcon: (icon: AppIcon) => void;
  /** The picture the server has for the app. */
  uploaded?: Asset;
  file: IconFile;
  setFile: (file: IconFile) => void;
  error?: string;
};

export function IconPicker({ id, appId, name, icon, setIcon, uploaded, file, setFile, error }: IconPickerProps) {
  const input = useRef<HTMLInputElement>(null);
  const [local, setLocal] = useState("");
  const picture = pictureOf(appId, uploaded, file);
  const pick = async (chosen?: File) => {
    setLocal("");
    if (!chosen) return;
    const problem = imageProblem(chosen, LIMITS.icon);
    if (problem) {
      setLocal(problem);
      return;
    }
    try {
      const data = await readBase64(chosen);
      setFile({ data, url: `data:${chosen.type};base64,${data}` });
    } catch {
      // The browser could not read the file it was given: it is said, nothing is kept.
      setLocal(t("errors.generic"));
    }
  };
  const hint = { letter: t("settings.page.appIconLetterHint"), emoji: t("settings.page.emojiHint"), image: t("settings.page.appIconImageHint", { kb: LIMITS.icon }) }[icon.kind];
  return (
    <Field label={t("settings.page.appIcon")} hint={hint} error={local || error}>
      <div className="flex flex-wrap items-center gap-3">
        <AppMark app={{ id: appId, name, icon: { kind: icon.kind, emoji: icon.emoji } }} src={() => picture} />
        <Segmented
          label={t("settings.page.appIcon")}
          value={icon.kind}
          onChange={(kind) => {
            setLocal("");
            setIcon({ ...icon, kind });
          }}
          options={[
            { value: "letter", label: t("settings.page.logoLetter") },
            { value: "emoji", label: t("settings.page.logoEmoji") },
            { value: "image", label: t("settings.page.logoImage") },
          ]}
        />
      </div>
      {icon.kind === "emoji" ? (
        <input
          id={`${id}-emoji`}
          className="input max-w-[120px] text-center text-lg"
          value={icon.emoji ?? ""}
          maxLength={16}
          autoComplete="off"
          aria-label={t("settings.page.emoji")}
          aria-invalid={!!error}
          onChange={(e) => setIcon({ ...icon, emoji: e.target.value.trim() })}
        />
      ) : null}
      {icon.kind === "image" ? (
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" onClick={() => input.current?.click()}>
            <ImageUp size={16} aria-hidden /> {picture ? t("settings.page.imageReplace") : t("settings.page.imageUpload")}
          </Button>
          {picture ? (
            <Button size="sm" variant="ghost" onClick={() => setFile(uploaded ? "remove" : null)}>
              {t("settings.page.imageRemove")}
            </Button>
          ) : null}
          <input
            ref={input}
            type="file"
            accept="image/png,image/jpeg,image/webp"
            className="sr-only"
            tabIndex={-1}
            aria-label={t("settings.page.appIconFile")}
            onChange={(e) => {
              void pick(e.target.files?.[0]);
              e.target.value = "";
            }}
          />
        </div>
      ) : null}
    </Field>
  );
}
