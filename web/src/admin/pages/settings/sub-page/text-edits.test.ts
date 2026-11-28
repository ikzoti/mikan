import { describe, expect, it } from "vitest";
import type { Schemas } from "../../../../api/client";
import { blockLacks, cleanTexts, docLacks, downloadsLack, lacksText, langOfField, textOf, textPath, withText, withTextOf } from "./text-edits";

type Block = Schemas["PageBlock"];

const block: Block = { id: "text-a", type: "text", on: true, title: "Привет", text: "Текст", i18n: { en: { title: "Hello" } } };

describe("withText", () => {
  it("sets a translation, the other languages and fields as they were", () => {
    expect(withText({ en: { title: "Hello" } }, "en", "text", "Text")).toEqual({ en: { title: "Hello", text: "Text" } });
    expect(withText({ en: { title: "Hello" } }, "ru", "title", "Привет")).toEqual({ en: { title: "Hello" }, ru: { title: "Привет" } });
    expect(withText(undefined, "en", "title", "Hello")).toEqual({ en: { title: "Hello" } });
  });
  it("takes an emptied one out, then its language, then the map", () => {
    expect(withText({ en: { title: "Hello", text: "Text" } }, "en", "text", "")).toEqual({ en: { title: "Hello" } });
    expect(withText({ en: { title: "Hello" }, ru: { title: "Привет" } }, "en", "title", "")).toEqual({ ru: { title: "Привет" } });
    expect(withText({ en: { title: "Hello" } }, "en", "title", "")).toBeUndefined();
  });
  it("keeps what is typed as it is: spaces go on saving", () => {
    expect(withText(undefined, "en", "title", " Hel")).toEqual({ en: { title: " Hel" } });
  });
});

describe("withTextOf and textOf", () => {
  it("edit the text itself with no language picked", () => {
    const b = withTextOf(block, "", "title", "Здравствуйте");
    expect(b.title).toBe("Здравствуйте");
    expect(b.i18n).toEqual({ en: { title: "Hello" } });
    expect(textOf(b, "", "title")).toBe("Здравствуйте");
  });
  it("edit the translation with a language picked, and leave the block as it was", () => {
    const b = withTextOf(block, "en", "text", "Text");
    expect(b).toMatchObject({ title: "Привет", text: "Текст", i18n: { en: { title: "Hello", text: "Text" } } });
    expect(textOf(b, "en", "text")).toBe("Text");
    expect(textOf(b, "ru", "text")).toBe("");
    expect(block.i18n).toEqual({ en: { title: "Hello" } });
  });
  it("leave a draft without the translation typed and wiped as it started", () => {
    const b = withTextOf(withTextOf(block, "ru", "title", "П"), "ru", "title", "");
    expect(JSON.stringify(b)).toBe(JSON.stringify(block));
  });
});

describe("what lacks a translation", () => {
  it("is a text of its own the language has nothing for", () => {
    expect(lacksText(block, "en", "title")).toBe(false);
    expect(lacksText(block, "en", "text")).toBe(true);
    expect(lacksText(block, "ru", "title")).toBe(true);
    expect(lacksText({ ...block, title: " " }, "ru", "title")).toBe(false);
    expect(lacksText({ title: "Привет", i18n: { ru: { title: "  " } } }, "ru", "title")).toBe(true);
  });
  it("looks at a block's heading, a text block's text and its buttons' texts", () => {
    expect(blockLacks(block, "en")).toBe(true);
    expect(blockLacks({ ...block, i18n: { en: { title: "Hello", text: "Text" } } }, "en")).toBe(false);
    expect(blockLacks({ id: "status", type: "status", on: true, title: "" }, "en")).toBe(false);
    const links: Block = { id: "links-a", type: "links", on: true, title: "", links: [{ label: "Канал", url: "https://t.me/x", i18n: { en: { label: "Channel" } } }, { label: "Бот", url: "tg://x" }] };
    expect(blockLacks(links, "en")).toBe(true);
    expect(blockLacks({ ...links, links: links.links?.slice(0, 1) }, "en")).toBe(false);
  });
  it("is a button's text with no translation; a button the page names itself needs none", () => {
    expect(downloadsLack([{ kind: "appstore", url: "https://apps.example.com/a" }], "en")).toBe(false);
    expect(downloadsLack([{ kind: "mirror", url: "https://files.example.com/a", label: "Зеркало" }], "en")).toBe(true);
    expect(downloadsLack([{ kind: "mirror", url: "https://files.example.com/a", label: "Зеркало", i18n: { en: { label: "Mirror" } } }], "en")).toBe(false);
  });
  it("is an instruction's title or text", () => {
    const doc = { id: 1, title: "Как", emoji: "", body: "Шаг 1", platform: "", published: true, updated_at: 0 };
    expect(docLacks({ ...doc, i18n: { en: { title: "How" } } }, "en")).toBe(true);
    expect(docLacks({ ...doc, i18n: { en: { title: "How", body: "Step 1" } } }, "en")).toBe(false);
    expect(docLacks({ ...doc, body: "", i18n: { en: { title: "How" } } }, "en")).toBe(false);
  });
});

describe("cleanTexts", () => {
  it("trims the texts and leaves out the empty ones, in the server's order of languages", () => {
    expect(cleanTexts({ ru: { label: " Зеркало " }, en: { label: "Mirror" } })).toEqual({ en: { label: "Mirror" }, ru: { label: "Зеркало" } });
    expect(Object.keys(cleanTexts({ ru: { label: "a" }, en: { label: "b" } }) ?? {})).toEqual(["en", "ru"]);
    expect(cleanTexts({ en: { title: "  ", body: "Step" }, ru: { title: "" } })).toEqual({ en: { body: "Step" } });
  });
  it("gives none for nothing left", () => {
    expect(cleanTexts({ en: { label: " " } })).toBeUndefined();
    expect(cleanTexts(undefined)).toBeUndefined();
  });
});

describe("the paths of refused fields", () => {
  it("name a translation by its language", () => {
    expect(textPath("", "title")).toBe("title");
    expect(textPath("en", "title")).toBe("i18n.en.title");
  });
  it("say the language of a refused translation", () => {
    expect(langOfField("config.blocks[14].i18n.en.title")).toBe("en");
    expect(langOfField("i18n.ru.body")).toBe("ru");
    expect(langOfField("platforms.ios.downloads[0].i18n.en.label")).toBe("en");
    expect(langOfField("config.blocks[14].title")).toBe("");
    expect(langOfField("config.brand.i18n.de")).toBe("");
    expect(langOfField("config.brand.i18n.en")).toBe("en");
  });
});
