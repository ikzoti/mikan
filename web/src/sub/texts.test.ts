import { describe, expect, it } from "vitest";
import type { CatalogApp } from "./apps";
import type { DocView, PageBlock } from "./page";
import { localApp, localBlock, localDoc, textIn } from "./texts";

describe("textIn", () => {
  const i18n = { en: { title: "Connect" }, ru: { title: "  " } };
  it("gives the translation into the language", () => {
    expect(textIn("en", "Подключение", i18n, "title")).toBe("Connect");
  });
  it("falls back to the text itself, never to another language", () => {
    expect(textIn("ru", "Подключение", { en: { title: "Connect" } }, "title")).toBe("Подключение");
    expect(textIn("en", "Подключение", undefined, "title")).toBe("Подключение");
    expect(textIn("en", "Подключение", { en: {} }, "title")).toBe("Подключение");
  });
  it("takes a blank translation for none", () => {
    expect(textIn("ru", "Подключение", i18n, "title")).toBe("Подключение");
  });
  it("gives the text itself with no language picked", () => {
    expect(textIn("", "Подключение", i18n, "title")).toBe("Подключение");
  });
  it("gives a translation of a text that has none of its own", () => {
    expect(textIn("en", "", i18n, "title")).toBe("Connect");
    expect(textIn("ru", "", i18n, "title")).toBe("");
  });
});

describe("localBlock", () => {
  const text: PageBlock = { id: "text-a", type: "text", on: true, title: "Привет", text: "Текст", i18n: { en: { title: "Hello", text: "Text" } } };
  const links: PageBlock = { id: "links-a", type: "links", on: true, title: "", links: [{ label: "Канал", url: "https://t.me/x", i18n: { en: { label: "Channel" } } }, { label: "Бот", url: "tg://resolve?domain=x" }] };

  it("puts the heading, the text and the buttons' texts in the language", () => {
    expect(localBlock(text, "en")).toMatchObject({ title: "Hello", text: "Text" });
    expect(localBlock(links, "en").links?.map((l) => l.label)).toEqual(["Channel", "Бот"]);
  });
  it("keeps the block's own texts where the language has none, and the block as it is", () => {
    expect(localBlock(text, "ru")).toMatchObject({ title: "Привет", text: "Текст" });
    expect(localBlock(links, "ru").links?.map((l) => l.label)).toEqual(["Канал", "Бот"]);
    expect(text.title).toBe("Привет");
  });
  it("gives no text to a block that has none", () => {
    const status: PageBlock = { id: "status", type: "status", on: true, title: "", i18n: { en: { title: "Status" } } };
    expect(localBlock(status, "en")).toMatchObject({ title: "Status", text: undefined });
  });
  it("is the block itself with no language picked", () => {
    expect(localBlock(text, "")).toBe(text);
  });
});

describe("localApp", () => {
  const own: CatalogApp = {
    id: "own-a1",
    name: "Наш VPN",
    family: "xray",
    own: true,
    icon: { kind: "letter" },
    link: "ourvpn://add/{url_enc}",
    crypt: false,
    note: "Для iPhone",
    i18n: { en: { note: "For iPhone" } },
    downloads: [
      { kind: "mirror", url: "https://files.example.com/a.ipa", label: "Зеркало", i18n: { en: { label: "Mirror" } } },
      { kind: "appstore", url: "https://apps.example.com/a" },
    ],
    doc: 0,
  };
  it("puts an own app's note and its buttons' texts in the language", () => {
    const app = localApp(own, "en");
    expect(app.note).toBe("For iPhone");
    expect(app.downloads.map((d) => d.label ?? "")).toEqual(["Mirror", ""]);
  });
  it("keeps a built-in app's note, a key the page words itself", () => {
    expect(localApp({ ...own, own: false, note: "easiest", i18n: undefined }, "en").note).toBe("easiest");
  });
  it("is the app itself with no language picked", () => {
    expect(localApp(own, "")).toBe(own);
  });
});

describe("localDoc", () => {
  const doc: DocView = { id: 1, title: "Как подключить", body: "Шаг 1", i18n: { en: { title: "How to connect", body: "Step 1" } } };
  it("puts the title and the text in the language", () => {
    expect(localDoc(doc, "en")).toMatchObject({ title: "How to connect", body: "Step 1" });
  });
  it("leaves the text of a doc in a list, which has none", () => {
    const item = localDoc({ id: 1, title: "Как подключить", i18n: { en: { title: "How to connect" } } }, "en");
    expect(item).toEqual({ id: 1, title: "How to connect", i18n: { en: { title: "How to connect" } } });
  });
  it("keeps its own words where the language has none", () => {
    expect(localDoc(doc, "ru")).toMatchObject({ title: "Как подключить", body: "Шаг 1" });
  });
});
