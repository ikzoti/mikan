import { describe, expect, it } from "vitest";
import { appIcon, appLink, appNote, appRows, downloadHref, downloadLabel, linkProblem, waysToGet, type CatalogApp, type IconSource } from "./apps";

const url = "https://vpn.example.com/sub/k3J9fQ2mXa7d";
const enc = encodeURIComponent;
const values = { url, brand: "Мой VPN & Co" };
const app = (link: string, crypt = false) => ({ link, crypt });

describe("appLink", () => {
  // The built-in apps' links come out as the page made them before the catalogue was the server's (#278).
  it.each([
    ["mikan://install-config?url={url_enc}&name={name}", `mikan://install-config?url=${enc(url)}&name=${enc(values.brand)}`],
    ["clash://install-config?url={url_enc}&name={name}", `clash://install-config?url=${enc(url)}&name=${enc(values.brand)}`],
    ["happ://add/{url}", `happ://add/${url}`],
    ["incy://add/{url}", `incy://add/${url}`],
    ["v2raytun://import/{url}", `v2raytun://import/${url}`],
    ["streisand://import/{url}#{name}", `streisand://import/${url}#${enc(values.brand)}`],
    ["hiddify://import/{url}#{name}", `hiddify://import/${url}#${enc(values.brand)}`],
    ["intent://add/{url}#Intent;scheme=myapp;end", `intent://add/${url}#Intent;scheme=myapp;end`],
    ["https://example.com/add?u={url_enc}", `https://example.com/add?u=${enc(url)}`],
  ])("fills %s", (template, want) => {
    expect(appLink(app(template), values)).toBe(want);
  });

  it("fills once: a value that spells a placeholder stays as it is", () => {
    expect(appLink(app("myapp://add/{url}#{name}"), { url, brand: "{url}" })).toBe(`myapp://add/${url}#%7Burl%7D`);
    expect(appLink(app("myapp://add/{url_enc}"), { url: "https://x.example/{name}", brand: "b" })).toBe(`myapp://add/${enc("https://x.example/{name}")}`);
  });

  it("opens Happ's crypt link when the panel gives one, and only an app of the same scheme", () => {
    const crypt = "happ://crypt5/" + "A".repeat(700);
    expect(appLink(app("happ://add/{url}", true), { ...values, happ: crypt })).toBe(crypt);
    expect(appLink(app("happ://add/{url}", true), values)).toBe(`happ://add/${url}`);
    expect(appLink(app("incy://add/{url}"), { ...values, happ: crypt })).toBe(`incy://add/${url}`);
    expect(appLink(app("happ://add/{url}", true), { ...values, happ: "javascript:alert(1)" })).toBeNull();
    expect(appLink(app("happ://add/{url}", true), { ...values, happ: "incy://add/x" })).toBeNull();
    expect(appLink(app("happ://add/{url}", true), { ...values, happ: "happ://crypt5/a b" })).toBeNull();
    expect(appLink(app("javascript:alert(1)//{url}", true), { ...values, happ: crypt })).toBeNull();
  });

  // The server's rules (subpage.AppLinkProblem): a link that would not pass there is no button here.
  it.each([
    // A scheme that opens something other than an app, under every spelling.
    "javascript:alert(1)//{url}",
    "JavaScript:alert(1)//{url}",
    "java%73cript:alert(1)//{url}",
    "%6Aavascript:alert(1)//{url}",
    "javascript&colon;alert(1)//{url}",
    "jav&#x61;script:alert(1)//{url}",
    "vbscript:msgbox(1)//{url}",
    "data:text/html,{url}",
    "file:///C:/{url}",
    "blob:https://example.com/{url}",
    "about:blank#{url}",
    "filesystem:https://example.com/{url}",
    "view-source:https://example.com/{url}",
    "jar:file:///x.jar!/{url}",
    "search-ms:query={url}",
    "search:query={url}",
    "shell:startup/{url}",
    "ms-msdt:/id/PCWDiagnostic/{url}",
    "ms-settings:{url}",
    "http://example.com/{url}",
    "HTTP://example.com/{url}",
    "Happ://add/{url}",
    "1app:{url}",
    "my_app:{url}",
    "a".repeat(33) + ":{url}",
    ":{url}",
    "//example.com/{url}",
    "{url}",
    "{url}:x",
    "",
    // What a browser drops or reads past is refused, not cleaned into a scheme.
    " javascript:alert(1)//{url}",
    "\tjavascript:alert(1)//{url}",
    "java\nscript:alert(1)//{url}",
    "java\u0000script:alert(1)//{url}",
    "myapp://add/{url} ",
    "myapp://add/{url}\u007f",
    "myapp://add/{ url }",
    // Letters that only look Latin, invisible and bidirectional marks.
    "jаvascript:alert(1)//{url}",
    "ｊavascript:alert(1)//{url}",
    "javascript\uff1aalert(1)//{url}",
    "\ufeffmyapp://add/{url}",
    "myapp://add/\u200b{url}",
    "myapp://add/{url}\u202e",
    "myapp://add/{url}\u00a0",
    "myapp://add/\ud800{url}",
    "myapp://add/{url}\uff5bname\uff5d",
    // The subscription's address must be in it, and nothing but the placeholders in braces.
    "myapp://add/",
    "myapp://add/{name}",
    "myapp://add/{URL}",
    "myapp://add/%7Burl%7D",
    "myapp://add/{url}{token}",
    "myapp://add/{url}&name={Name}",
    "myapp://add/{url}}",
    "myapp://add/{{url}}",
    "myapp://add/{url_enc",
    "myapp://add/{url}#" + "a".repeat(500 - "myapp://add/{url}#".length + 1),
  ])("refuses %j", (template) => {
    expect(appLink(app(template), values)).toBeNull();
  });

  it("takes the longest link the server does, a scheme of 32 letters and a URI of its own", () => {
    const long = "myapp://add/{url}#" + "a".repeat(500 - "myapp://add/{url}#".length);
    expect(appLink(app(long), values)).toBe(long.replace("{url}", url));
    expect(appLink(app("a".repeat(32) + ":{url}"), values)).toBe("a".repeat(32) + ":" + url);
    expect(appLink(app("com.example.vpn:{url}"), values)).toBe("com.example.vpn:" + url);
  });

  it("gives no link when what fills it breaks the link", () => {
    expect(appLink(app("myapp://add/{url}"), { url: "https://x.example/a b", brand: "b" })).toBeNull();
  });
});

describe("downloadHref", () => {
  it.each([
    ["https://apps.apple.com/app/id1630393283", "https://apps.apple.com/app/id1630393283"],
    ["HTTPS://Example.com/Happ.apk", "https://example.com/Happ.apk"],
    ["https://пример.рф/app", "https://xn--e1afmkfd.xn--p1ai/app"],
  ])("opens %s", (link, want) => {
    expect(downloadHref(link)).toBe(want);
  });

  it.each([
    "",
    "http://example.com/app.apk",
    "javascript:alert(1)",
    "data:text/html,<script>alert(1)</script>",
    "happ://add/https://example.com",
    "//example.com/app.apk",
    "/app.apk",
    " https://example.com/app.apk",
    "https://exa\tmple.com/app.apk",
    "https://example.com/app.apk\n",
    "https://user:secret@example.com/app.apk",
    "https://apps.apple.com@evil.example/app.apk",
  ])("refuses %j", (link) => {
    expect(downloadHref(link)).toBeNull();
  });
});

describe("downloadLabel", () => {
  it("names a store by itself and takes the admin's text over it", () => {
    expect(downloadLabel({ kind: "appstore", url: "https://a.example" })).toBe("App Store");
    expect(downloadLabel({ kind: "googleplay", url: "https://a.example" })).toBe("Google Play");
    expect(downloadLabel({ kind: "github", url: "https://a.example" })).toBe("GitHub");
    expect(downloadLabel({ kind: "mirror", url: "https://a.example" })).toBe("Зеркало");
    expect(downloadLabel({ kind: "mirror", url: "https://a.example", label: "Наш сервер" })).toBe("Наш сервер");
    expect(downloadLabel({ kind: "link", url: "https://beta.example/x", label: "Бета" })).toBe("Бета");
  });
});

const catalogApp = (over: Partial<CatalogApp>): CatalogApp => ({ id: "own-a1", name: "Наш VPN", family: "xray", own: true, icon: { kind: "letter" }, link: "", crypt: false, note: "", downloads: [], doc: 0, ...over });

describe("appIcon", () => {
  const root = "https://vpn.example.com/sub/";
  const src: IconSource = (a) => (a.icon.path ? root + a.icon.path : undefined);

  it("draws the picture where the page finds it", () => {
    expect(appIcon(catalogApp({ icon: { kind: "image", path: "brand/app-own-a1?v=ab12" } }), src)).toEqual({ kind: "image", src: root + "brand/app-own-a1?v=ab12" });
  });

  it("draws the letter for a picture the page has not", () => {
    expect(appIcon(catalogApp({ icon: { kind: "image" } }), src)).toEqual({ kind: "letter", letter: "Н" });
    expect(appIcon(catalogApp({ icon: { kind: "image", path: "brand/app-own-a1?v=ab12" } }), () => undefined)).toEqual({ kind: "letter", letter: "Н" });
  });

  it("draws the emoji, and the letter in place of an empty one", () => {
    expect(appIcon(catalogApp({ icon: { kind: "emoji", emoji: "🛡️" } }), src)).toEqual({ kind: "emoji", emoji: "🛡️" });
    expect(appIcon(catalogApp({ icon: { kind: "emoji", emoji: "" } }), src)).toEqual({ kind: "letter", letter: "Н" });
  });

  it("takes the name's first letter whole, one of two code units too, and a mark for no name", () => {
    expect(appIcon(catalogApp({ name: "𝔸pp" }), src)).toEqual({ kind: "letter", letter: "𝔸" });
    expect(appIcon(catalogApp({ name: "  happ" }), src)).toEqual({ kind: "letter", letter: "h" });
    expect(appIcon(catalogApp({ name: "" }), src)).toEqual({ kind: "letter", letter: "?" });
  });
});

describe("appNote", () => {
  it("words a built-in app's note in the visitor's language", () => {
    expect(appNote({ note: "free", own: false })).toBe("Бесплатно, без рекламы");
    expect(appNote({ note: "Своя подпись", own: false })).toBe("Своя подпись");
  });

  it("keeps an own app's note as the admin wrote it, one spelled as a key too", () => {
    expect(appNote({ note: "free", own: true })).toBe("free");
    expect(appNote({ note: "", own: true })).toBe("");
  });
});

// The server's codes (subpage.AppLinkProblem), in its order of checks: the editor says why a link is refused before it is sent.
describe("linkProblem", () => {
  it.each([
    ["myapp://add/{url}", null],
    ["myapp://install-config?url={url_enc}&name={name}", null],
    ["https://example.com/add?u={url_enc}", null],
    ["", "app_link_scheme"],
    ["javascript:alert(1)//{url}", "app_link_scheme"],
    ["http://example.com/{url}", "app_link_scheme"],
    ["ms-settings:{url}", "app_link_scheme"],
    ["Happ://add/{url}", "app_link_scheme"],
    ["myapp://add/", "app_link_url"],
    ["myapp://add/{name}", "app_link_url"],
    ["myapp://add/{url_enc", "app_link_url"],
    ["myapp://add/{url}{token}", "app_link_invalid"],
    ["myapp://add/{url}}", "app_link_invalid"],
    [" myapp://add/{url}", "app_link_invalid"],
    ["jаvascript:alert(1)//{url}", "app_link_invalid"],
    ["myapp://add/{url}#" + "a".repeat(500), "app_link_invalid"],
  ])("%j: %s", (link, want) => {
    expect(linkProblem(link)).toBe(want);
  });
});

describe("appRows", () => {
  const happ = catalogApp({ id: "happ", name: "Happ", own: false, link: "happ://add/{url}", crypt: true });
  const incy = catalogApp({ id: "incy", name: "INCY", own: false, link: "incy://add/{url}" });
  const mirror = catalogApp({ id: "own-mirror", link: "" });
  const ids = (rows: ReturnType<typeof appRows>) => rows.map((r) => r.app.id);

  it("offers the apps in their order, each with the link that adds the subscription", () => {
    expect(appRows([incy, happ], undefined, values)).toEqual([
      { app: incy, link: `incy://add/${url}` },
      { app: happ, link: `happ://add/${url}` },
    ]);
  });

  it("leaves out the apps the admin turned away", () => {
    expect(ids(appRows([happ, incy, mirror], ["happ", "own-mirror"], values))).toEqual(["incy"]);
    expect(ids(appRows([happ, incy], [], values))).toEqual(["happ", "incy"]);
  });

  it("keeps an own app with no link, with no Add", () => {
    expect(appRows([mirror], undefined, values)).toEqual([{ app: mirror, link: null }]);
  });

  it("leaves out an app whose link would open no app", () => {
    const script = catalogApp({ id: "own-bad", link: "javascript:alert(1)//{url}" });
    expect(ids(appRows([script, incy], undefined, values))).toEqual(["incy"]);
    // The crypt link of another app's scheme opens neither.
    expect(ids(appRows([happ, incy], undefined, { ...values, happ: "incy://crypt/abc" }))).toEqual(["incy"]);
    expect(appRows([happ], undefined, { ...values, happ: "happ://crypt5/abc" })).toEqual([{ app: happ, link: "happ://crypt5/abc" }]);
  });
});

describe("waysToGet", () => {
  const store = { kind: "appstore", url: "https://apps.example.com/a" } as const;
  const mirror = { kind: "mirror", url: "https://files.example.com/a.ipa", label: "Наш сервер" } as const;
  const published = (id: number) => id === 7;

  it("leads straight to the only download of an app with no instruction", () => {
    expect(waysToGet({ downloads: [store], doc: 0 }, published)).toEqual({ sources: [{ href: store.url, label: "App Store" }], doc: 0, toggles: false });
  });

  it("opens the downloads under the row when there are more ways than one", () => {
    expect(waysToGet({ downloads: [store, mirror], doc: 0 }, published).toggles).toBe(true);
    expect(waysToGet({ downloads: [store], doc: 7 }, published)).toEqual({ sources: [{ href: store.url, label: "App Store" }], doc: 7, toggles: true });
  });

  it("gives the instruction alone without downloads", () => {
    expect(waysToGet({ downloads: [], doc: 7 }, published)).toEqual({ sources: [], doc: 7, toggles: false });
  });

  it("counts neither a draft instruction nor a download the page cannot open", () => {
    expect(waysToGet({ downloads: [store], doc: 9 }, published)).toEqual({ sources: [{ href: store.url, label: "App Store" }], doc: 0, toggles: false });
    expect(waysToGet({ downloads: [{ kind: "mirror", url: "http://files.example.com/a.ipa" }, mirror], doc: 0 }, published)).toEqual({ sources: [{ href: mirror.url, label: "Наш сервер" }], doc: 0, toggles: false });
    expect(waysToGet({ downloads: [], doc: 0 }, published)).toEqual({ sources: [], doc: 0, toggles: false });
  });
});
