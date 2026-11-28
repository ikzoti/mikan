import { describe, expect, it } from "vitest";
import type { Schemas } from "../../../../api/client";
import { cleanDownloads, iconOf, newOwnId, ownAppOf, ownDraft, ownPrefix, scopedErrors, withAppEdit, withOwnApp, withoutOwnApp, withPresetIcon } from "./app-edits";

type AppsConfig = Schemas["AppsConfig"];
type OwnApp = Schemas["OwnApp"];

const store = { kind: "appstore", url: "https://apps.example.com/happ" } as const;
const mirror = { kind: "mirror", url: "https://files.example.com/happ.apk", label: "Наш сервер" } as const;
const none: AppsConfig = { presets: {}, own: [] };
const ours: OwnApp = { id: "own-a1", name: "Наш VPN", family: "xray", link: "ourvpn://add/{url_enc}", platforms: { ios: { note: "Для iPhone" } } };

describe("withAppEdit", () => {
  it("puts the platform's downloads and instruction into the document, the rest as it was", () => {
    const before: AppsConfig = { presets: { hiddify: { platforms: { linux: { doc: 3 } } } }, own: [ours] };
    const after = withAppEdit(before, "happ", "ios", { downloads: [store, mirror], doc: 7 }, []);
    expect(after).toEqual({ presets: { hiddify: { platforms: { linux: { doc: 3 } } }, happ: { platforms: { ios: { downloads: [store, mirror], doc: 7 } } } }, own: [ours] });
    expect(before).toEqual({ presets: { hiddify: { platforms: { linux: { doc: 3 } } } }, own: [ours] });
  });

  it("leaves out downloads that are the app's own, so it follows its built-in ones", () => {
    expect(withAppEdit(none, "happ", "ios", { downloads: [store], doc: 7 }, [store])).toEqual({ presets: { happ: { platforms: { ios: { doc: 7 } } } }, own: [] });
    expect(withAppEdit(none, "happ", "ios", { downloads: [], doc: 0 }, [])).toEqual(none);
  });

  it("keeps an empty list over built-in downloads: the app has no buttons then", () => {
    expect(withAppEdit(none, "happ", "ios", { downloads: [], doc: 0 }, [store])).toEqual({ presets: { happ: { platforms: { ios: { downloads: [] } } } }, own: [] });
  });

  it("drops what the platform no longer changes, and the app with nothing left", () => {
    const before: AppsConfig = { presets: { happ: { platforms: { ios: { doc: 7 }, android: { downloads: [] } } } }, own: [] };
    expect(withAppEdit(before, "happ", "ios", null, [])).toEqual({ presets: { happ: { platforms: { android: { downloads: [] } } } }, own: [] });
    expect(withAppEdit(before, "happ", "android", { downloads: [], doc: 0 }, [])).toEqual({ presets: { happ: { platforms: { ios: { doc: 7 } } } }, own: [] });
    expect(withAppEdit({ presets: { happ: { platforms: { ios: { doc: 7 } } } }, own: [] }, "happ", "ios", null, [])).toEqual(none);
  });

  it("keeps the app's icon when its platform goes back to the built-in buttons", () => {
    const before: AppsConfig = { presets: { happ: { icon: { kind: "emoji", emoji: "🚀" }, platforms: { ios: { doc: 7 } } } }, own: [] };
    expect(withAppEdit(before, "happ", "ios", null, [])).toEqual({ presets: { happ: { icon: { kind: "emoji", emoji: "🚀" } } }, own: [] });
  });

  it("sends the text trimmed and no empty label", () => {
    const typed = [{ kind: "github", url: "  https://github.com/x/y/releases ", label: "  " }, { kind: "link", url: "https://beta.example", label: " Бета " }] as const;
    expect(withAppEdit(none, "verge", "linux", { downloads: [...typed], doc: 0 }, [])).toEqual({
      presets: { verge: { platforms: { linux: { downloads: [{ kind: "github", url: "https://github.com/x/y/releases" }, { kind: "link", url: "https://beta.example", label: "Бета" }] } } } },
      own: [],
    });
  });
});

describe("withPresetIcon", () => {
  it("sets a built-in app's icon beside its platforms", () => {
    const before: AppsConfig = { presets: { happ: { platforms: { ios: { doc: 7 } } } }, own: [] };
    expect(withPresetIcon(before, "happ", { kind: "image", emoji: "x" })).toEqual({ presets: { happ: { icon: { kind: "image" }, platforms: { ios: { doc: 7 } } } }, own: [] });
    expect(withPresetIcon(none, "incy", { kind: "emoji", emoji: " ⚡ " })).toEqual({ presets: { incy: { icon: { kind: "emoji", emoji: "⚡" } } }, own: [] });
  });

  it("takes the letter for no icon of its own, and the app with nothing else changed out", () => {
    const before: AppsConfig = { presets: { happ: { icon: { kind: "image" } }, incy: { icon: { kind: "image" }, platforms: { ios: { doc: 2 } } } }, own: [] };
    expect(withPresetIcon(before, "happ", { kind: "letter" })).toEqual({ presets: { incy: { icon: { kind: "image" }, platforms: { ios: { doc: 2 } } } }, own: [] });
    expect(withPresetIcon(before, "incy", { kind: "letter" })).toEqual({ presets: { happ: { icon: { kind: "image" } }, incy: { platforms: { ios: { doc: 2 } } } }, own: [] });
  });
});

describe("own apps", () => {
  const other: OwnApp = { id: "own-b2", name: "Второе", family: "mihomo", link: "", platforms: { linux: {} } };

  it("puts an app in its place, or after the others when it is new, and takes one out", () => {
    const apps: AppsConfig = { presets: {}, own: [ours, other] };
    const renamed = { ...ours, name: "Новое имя" };
    expect(withOwnApp(apps, renamed).own).toEqual([renamed, other]);
    const third: OwnApp = { ...ours, id: "own-c3" };
    expect(withOwnApp(apps, third).own).toEqual([ours, other, third]);
    expect(withoutOwnApp(apps, "own-a1")).toEqual({ presets: {}, own: [other] });
    expect(apps.own).toEqual([ours, other]);
  });

  it("makes an id no other app has", () => {
    const values = [0.5, 0.25, 0.75];
    const random = () => values.shift() ?? 0;
    expect(newOwnId([], () => 0.5)).toBe("own-i");
    expect(newOwnId(["own-i", "own-9"], random)).toBe("own-r");
    expect(newOwnId([], () => 0)).toBe("own-a");
    expect(newOwnId([])).toMatch(/^own-[a-z0-9]{1,12}$/);
  });

  it("reads back from its draft as the server keeps it", () => {
    const saved: OwnApp = {
      id: "own-a1",
      name: "Наш VPN",
      family: "xray",
      icon: { kind: "emoji", emoji: "🛡️" },
      link: "ourvpn://add/{url_enc}",
      platforms: { android: { downloads: [store], doc: 3 }, ios: { note: "Для iPhone" } },
    };
    const d = ownDraft(saved, "own-zz", "linux");
    expect(d.on).toEqual(["ios", "android"]);
    expect(d.platforms.linux).toEqual({ note: "", downloads: [], doc: 0 });
    expect(ownAppOf(d)).toEqual(saved);
  });

  it("sends the platforms ticked, the text trimmed and nothing empty", () => {
    const d = ownDraft(undefined, "own-k7", "ios");
    expect(d).toMatchObject({ id: "own-k7", name: "", family: "other", icon: { kind: "letter" }, link: "", on: ["ios"] });
    d.name = "  Наш VPN ";
    d.link = " ourvpn://add/{url} ";
    d.on = ["windows", "ios"];
    d.platforms.ios = { note: "  ", downloads: [{ kind: "link", url: " https://example.com/a ", label: " Бета " }], doc: 0 };
    d.platforms.windows = { note: " Для Windows ", downloads: [], doc: 4 };
    d.platforms.android = { note: "не отмечена", downloads: [store], doc: 2 };
    expect(ownAppOf(d)).toEqual({
      id: "own-k7",
      name: "Наш VPN",
      family: "other",
      link: "ourvpn://add/{url}",
      platforms: { ios: { downloads: [{ kind: "link", url: "https://example.com/a", label: "Бета" }] }, windows: { note: "Для Windows", doc: 4 } },
    });
  });
});

describe("iconOf", () => {
  it("keeps what the document keeps of an icon", () => {
    expect(iconOf({ kind: "letter", emoji: "x" })).toBeUndefined();
    expect(iconOf({ kind: "image", emoji: "x" })).toEqual({ kind: "image" });
    expect(iconOf({ kind: "emoji", emoji: " 🚀 " })).toEqual({ kind: "emoji", emoji: "🚀" });
  });
});

describe("cleanDownloads", () => {
  it("compares as the server keeps them", () => {
    expect(cleanDownloads([{ kind: "appstore", url: " https://a.example ", label: "" }])).toEqual([{ kind: "appstore", url: "https://a.example" }]);
  });
});

describe("scopedErrors", () => {
  it("keeps the errors under a prefix, named from there", () => {
    const errors = {
      "presets.happ.platforms.ios.downloads[1].url": "url",
      "presets.happ.icon.emoji": "emoji",
      "presets.happy.icon.emoji": "other app",
      "own[1].name": "name",
      "own[10].name": "eleventh",
    };
    expect(scopedErrors(errors, "presets.happ.")).toEqual({ "platforms.ios.downloads[1].url": "url", "icon.emoji": "emoji" });
    expect(scopedErrors(errors, "own[1].")).toEqual({ name: "name" });
    expect(scopedErrors({}, "own[0].")).toEqual({});
  });
});

describe("ownPrefix", () => {
  it("names an own app's fields by its place in the document, a new one after the others", () => {
    const apps: AppsConfig = { presets: {}, own: [ours, { ...ours, id: "own-b2" }] };
    expect(ownPrefix(apps, "own-b2")).toBe("own[1].");
    expect(ownPrefix(apps, "own-a1")).toBe("own[0].");
    expect(ownPrefix(apps, "own-new")).toBe("own[2].");
    expect(ownPrefix(none, "own-new")).toBe("own[0].");
  });
});
