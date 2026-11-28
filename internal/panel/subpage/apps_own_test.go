package subpage

import (
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// ownApp is an own app on the iPhone the page can show.
func ownApp(id string) OwnApp {
	return OwnApp{ID: id, Name: "Наш VPN", Family: "xray", Link: "ourvpn://add/{url_enc}", Platforms: map[string]OwnPlatform{"ios": {Note: "Для iPhone"}}}
}

func withOwn(list ...OwnApp) AppsConfig { return AppsConfig{Own: list} }

func TestNormalizeOwnAppsRefuses(t *testing.T) {
	changed := func(f func(*OwnApp)) AppsConfig {
		a := ownApp("own-a1")
		f(&a)
		return withOwn(a)
	}
	onIOS := func(p OwnPlatform) AppsConfig {
		return changed(func(a *OwnApp) { a.Platforms = map[string]OwnPlatform{"ios": p} })
	}
	store := AppDownload{Kind: KindAppStore, URL: "https://apps.example.com/ourvpn"}
	many := make([]OwnApp, MaxOwnApps+1)
	for i := range many {
		many[i] = ownApp("own-" + strconv.Itoa(i))
	}
	cases := []struct {
		name  string
		c     AppsConfig
		field string
		code  string
	}{
		{"too many apps", withOwn(many...), "own", "too_many"},
		{"no id", changed(func(a *OwnApp) { a.ID = "" }), "own[0].id", "app_id_invalid"},
		{"a built-in app's id", changed(func(a *OwnApp) { a.ID = "happ" }), "own[0].id", "app_id_invalid"},
		{"an id without its prefix", changed(func(a *OwnApp) { a.ID = "a1" }), "own[0].id", "app_id_invalid"},
		{"an id in capitals", changed(func(a *OwnApp) { a.ID = "own-A1" }), "own[0].id", "app_id_invalid"},
		{"an id too long", changed(func(a *OwnApp) { a.ID = "own-" + strings.Repeat("a", 13) }), "own[0].id", "app_id_invalid"},
		{"a hyphen after the prefix", changed(func(a *OwnApp) { a.ID = "own-a-1" }), "own[0].id", "app_id_invalid"},
		{"the same id twice", withOwn(ownApp("own-a1"), ownApp("own-a1")), "own[1].id", "app_duplicate"},
		{"no name", changed(func(a *OwnApp) { a.Name = "   " }), "own[0].name", "required"},
		{"a name on two lines", changed(func(a *OwnApp) { a.Name = "Наш\nVPN" }), "own[0].name", "one_line"},
		{"a name too long", changed(func(a *OwnApp) { a.Name = strings.Repeat("я", 41) }), "own[0].name", "too_long"},
		{"an unknown family", changed(func(a *OwnApp) { a.Family = "wireguard" }), "own[0].family", "value_invalid"},
		{"no platforms", changed(func(a *OwnApp) { a.Platforms = nil }), "own[0].platforms", "platforms_required"},
		{"an unknown platform", changed(func(a *OwnApp) { a.Platforms["beos"] = OwnPlatform{} }), "own[0].platforms.beos", "value_invalid"},
		{"no platform but an unknown one", changed(func(a *OwnApp) { a.Platforms = map[string]OwnPlatform{"beos": {}} }), "own[0].platforms", "platforms_required"},
		{"a script for a link", changed(func(a *OwnApp) { a.Link = "javascript:alert(1)//{url}" }), "own[0].link", "app_link_scheme"},
		{"http", changed(func(a *OwnApp) { a.Link = "http://example.com/add?u={url_enc}" }), "own[0].link", "app_link_scheme"},
		{"a link without the address", changed(func(a *OwnApp) { a.Link = "ourvpn://add/" }), "own[0].link", "app_link_url"},
		{"a placeholder the page does not know", changed(func(a *OwnApp) { a.Link = "ourvpn://add/{url}?t={token}" }), "own[0].link", "app_link_invalid"},
		{"a look-alike letter in the scheme", changed(func(a *OwnApp) { a.Link = "jаvascript:alert(1)//{url}" }), "own[0].link", "app_link_invalid"},
		{"a note on two lines", onIOS(OwnPlatform{Note: "Для\niPhone"}), "own[0].platforms.ios.note", "one_line"},
		{"a note too long", onIOS(OwnPlatform{Note: strings.Repeat("я", 81)}), "own[0].platforms.ios.note", "too_long"},
		{"too many downloads", onIOS(OwnPlatform{Downloads: []AppDownload{store, store, store, store, store, store}}), "own[0].platforms.ios.downloads", "too_many"},
		{"a download over http", onIOS(OwnPlatform{Downloads: []AppDownload{{Kind: KindMirror, URL: "http://files.example.com/ourvpn.ipa"}}}), "own[0].platforms.ios.downloads[0].url", "url_invalid"},
		{"a link button without its text", onIOS(OwnPlatform{Downloads: []AppDownload{store, {Kind: KindLink, URL: "https://example.com/beta"}}}), "own[0].platforms.ios.downloads[1].label", "label_required"},
		{"a negative instruction", onIOS(OwnPlatform{Doc: -1}), "own[0].platforms.ios.doc", "value_invalid"},
		{"an icon of no kind", changed(func(a *OwnApp) { a.Icon = &AppIcon{Kind: "svg"} }), "own[0].icon.kind", "value_invalid"},
		{"an emoji icon without its emoji", changed(func(a *OwnApp) { a.Icon = &AppIcon{Kind: IconEmoji, Emoji: " "} }), "own[0].icon.emoji", "required"},
		{"an emoji icon with markup", changed(func(a *OwnApp) { a.Icon = &AppIcon{Kind: IconEmoji, Emoji: "<b>"} }), "own[0].icon.emoji", "emoji_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := codes(NormalizeApps(&tc.c))
			if got[tc.field] != tc.code {
				t.Fatalf("problems %v, want %s at %s", got, tc.code, tc.field)
			}
		})
	}
}

// An own app is trimmed into the one form the document keeps; one with no link has no "Add"
// and stays, and the letter, which every app has anyway, is no icon of its own.
func TestNormalizeOwnAppsKeepsWhatIsRight(t *testing.T) {
	c := AppsConfig{
		Presets: map[string]PresetEdit{
			// An icon alone is an edit of a built-in app.
			"happ": {Icon: &AppIcon{Kind: " emoji ", Emoji: " 🚀 "}},
			"incy": {Icon: &AppIcon{Kind: IconLetter, Emoji: "x"}},
		},
		Own: []OwnApp{
			{ID: " own-b2 ", Name: "  Наш VPN  ", Family: " singbox ", Icon: &AppIcon{Kind: IconImage, Emoji: "x"}, Link: " ourvpn://add/{url_enc}#{name} ", Platforms: map[string]OwnPlatform{
				"android": {Note: "  Для Android  ", Downloads: []AppDownload{{Kind: " googleplay ", URL: " https://play.example.com/ourvpn "}}, Doc: 4},
				"windows": {},
			}},
			{ID: "own-a1", Name: "Зеркало", Family: "other", Icon: &AppIcon{Kind: IconLetter}, Platforms: map[string]OwnPlatform{"linux": {Downloads: []AppDownload{}}}},
		},
	}
	if ps := NormalizeApps(&c); len(ps) != 0 {
		t.Fatalf("problems: %+v", ps)
	}
	want := AppsConfig{
		Presets: map[string]PresetEdit{"happ": {Icon: &AppIcon{Kind: IconEmoji, Emoji: "🚀"}}},
		Own: []OwnApp{
			{ID: "own-b2", Name: "Наш VPN", Family: "singbox", Icon: &AppIcon{Kind: IconImage}, Link: "ourvpn://add/{url_enc}#{name}", Platforms: map[string]OwnPlatform{
				"android": {Note: "Для Android", Downloads: []AppDownload{{Kind: KindGooglePlay, URL: "https://play.example.com/ourvpn"}}, Doc: 4},
				"windows": {},
			}},
			{ID: "own-a1", Name: "Зеркало", Family: "other", Platforms: map[string]OwnPlatform{"linux": {}}},
		},
	}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("cleaned to\n%+v\nwant\n%+v", c, want)
	}
	again := c
	if ps := NormalizeApps(&again); len(ps) != 0 || !reflect.DeepEqual(again, want) {
		t.Fatalf("a clean document changed: %+v %+v", ps, again)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back AppsConfig
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, want) {
		t.Fatalf("does not read back: %v %s", err, raw)
	}
	if strings.Contains(string(raw), "null") || !strings.Contains(string(raw), `"happ":{"icon":{"kind":"emoji","emoji":"🚀"}}`) {
		t.Fatalf("stored as %s", raw)
	}
}

// A stored document is shown as it was cleaned: an app the page cannot name or place goes,
// and of the rest what is wrong is left out.
func TestNormalizeOwnAppsDropsWhatIsWrong(t *testing.T) {
	github := AppDownload{Kind: KindGitHub, URL: "https://github.com/example/ourvpn/releases"}
	c := withOwn(
		OwnApp{ID: "happ", Name: "Чужой id", Family: "xray", Platforms: map[string]OwnPlatform{"ios": {}}},
		OwnApp{ID: "own-a", Name: "Первое", Family: "wireguard", Icon: &AppIcon{Kind: "svg"}, Link: "javascript:alert(1)//{url}", Platforms: map[string]OwnPlatform{
			"ios":  {Note: "две\nстроки", Doc: -2, Downloads: []AppDownload{{Kind: KindMirror, URL: "http://example.com/a.ipa"}, github}},
			"beos": {},
		}},
		OwnApp{ID: "own-a", Name: "Тот же id", Family: "xray", Platforms: map[string]OwnPlatform{"ios": {}}},
		OwnApp{ID: "own-b", Name: "Без платформ", Family: "xray", Platforms: map[string]OwnPlatform{"beos": {}}},
		OwnApp{ID: "own-c", Name: "\x00", Family: "xray", Platforms: map[string]OwnPlatform{"ios": {}}},
	)
	if ps := NormalizeApps(&c); len(ps) == 0 {
		t.Fatal("no problems")
	}
	want := withOwn(OwnApp{ID: "own-a", Name: "Первое", Family: "other", Platforms: map[string]OwnPlatform{"ios": {Downloads: []AppDownload{github}}}})
	want.Presets = map[string]PresetEdit{}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("cleaned to %+v", c)
	}
}

// Each platform lists the built-in apps in their order and the admin's own on it in the
// document's order: an own app on mihomo after the built-in ones on mihomo, ahead of the others
// (none on the iPhone), and the rest of the own apps last.
func TestCatalogOwnApps(t *testing.T) {
	store := AppDownload{Kind: KindAppStore, URL: "https://apps.example.com/ourvpn"}
	c := withOwn(
		OwnApp{ID: "own-b", Name: "Второе", Family: "mihomo", Link: "second://add/{url}", Platforms: map[string]OwnPlatform{"ios": {Note: "Подпись"}, "windows": {Doc: 3}}},
		OwnApp{ID: "own-a", Name: "Первое", Family: "xray", Platforms: map[string]OwnPlatform{"ios": {Downloads: []AppDownload{store}}, "android": {}}},
		OwnApp{ID: "own-c", Name: "Третье", Family: "mihomo", Platforms: map[string]OwnPlatform{"android": {}, "windows": {}}},
		OwnApp{ID: "own-d", Name: "Четвёртое", Family: "singbox", Platforms: map[string]OwnPlatform{"windows": {}}},
	)
	if ps := NormalizeApps(&c); len(ps) != 0 {
		t.Fatalf("problems: %+v", ps)
	}
	got, presets := c.Catalog(nil), Presets()
	for platform, want := range map[string]string{
		"ios":     "own-b," + ids(presets.IOS) + ",own-a",
		"android": "mikan,clashfest,own-c,happ,incy,v2raytun,hiddify,own-a",
		"windows": "kabosu,koala,slothclash,verge,own-b,own-c,hiddify,own-d",
		"macos":   ids(presets.MacOS),
		"linux":   ids(presets.Linux),
	} {
		if list := *got.lists()[slices.Index(Platforms, platform)]; ids(list) != want {
			t.Errorf("%s: %s, want %s", platform, ids(list), want)
		}
	}
	b := find(t, got.IOS, "own-b")
	if want := (CatalogApp{ID: "own-b", Name: "Второе", Family: "mihomo", Link: "second://add/{url}", Note: "Подпись", Own: true, Downloads: []AppDownload{}, Icon: CatalogIcon{Kind: IconLetter}}); !reflect.DeepEqual(b, want) {
		t.Errorf("own-b on ios: %+v", b)
	}
	if w := find(t, got.Windows, "own-b"); w.Doc != 3 || w.Note != "" || !w.Own {
		t.Errorf("own-b on windows: %+v", w)
	}
	a := find(t, got.IOS, "own-a")
	if a.Link != "" || !reflect.DeepEqual(a.Downloads, []AppDownload{store}) {
		t.Errorf("own-a, the app with no link: %+v", a)
	}
	if find(t, got.IOS, "happ").Own {
		t.Error("a built-in app is taken for an own one")
	}
	a.Downloads[0].URL = "https://changed.example.com"
	if !reflect.DeepEqual(c.Own[1].Platforms["ios"].Downloads[0], store) {
		t.Fatal("the catalogue shares the document's list")
	}
}

// An app is drawn with its picture when one is uploaded for it, its emoji, or else the letter:
// a picture chosen but not there is the letter, and one there but not chosen is not shown.
func TestCatalogIcons(t *testing.T) {
	c := AppsConfig{
		Presets: map[string]PresetEdit{
			"happ":    {Icon: &AppIcon{Kind: IconImage}},
			"incy":    {Icon: &AppIcon{Kind: IconImage}},
			"hiddify": {Icon: &AppIcon{Kind: IconEmoji, Emoji: "🛡️"}},
		},
		Own: []OwnApp{{ID: "own-a", Name: "Наш", Family: "other", Icon: &AppIcon{Kind: IconImage}, Platforms: map[string]OwnPlatform{"ios": {}}}},
	}
	if ps := NormalizeApps(&c); len(ps) != 0 {
		t.Fatalf("problems: %+v", ps)
	}
	got := c.Catalog(map[string]PageAsset{"happ": {Hash: "aa11"}, "own-a": {Hash: "bb22"}, "streisand": {Hash: "cc33"}})
	happ := CatalogIcon{Kind: IconImage, Path: "brand/app-happ?v=aa11"}
	letter := CatalogIcon{Kind: IconLetter}
	for _, list := range [][]CatalogApp{got.IOS, got.Android, got.MacOS} {
		if a := find(t, list, "happ"); a.Icon != happ {
			t.Errorf("happ: %+v", a.Icon)
		}
	}
	for _, list := range [][]CatalogApp{got.Android, got.Windows, got.MacOS, got.Linux} {
		if a := find(t, list, "hiddify"); a.Icon != (CatalogIcon{Kind: IconEmoji, Emoji: "🛡️"}) {
			t.Errorf("hiddify: %+v", a.Icon)
		}
	}
	for id, want := range map[string]CatalogIcon{
		"incy":      letter,
		"streisand": letter,
		"v2raytun":  letter,
		"own-a":     {Kind: IconImage, Path: "brand/app-own-a?v=bb22"},
	} {
		if a := find(t, got.IOS, id); a.Icon != want {
			t.Errorf("%s: %+v, want %+v", id, a.Icon, want)
		}
	}
	presets := Presets()
	for _, list := range presets.lists() {
		for _, a := range *list {
			if a.Icon != letter {
				t.Errorf("%s as it comes: %+v", a.ID, a.Icon)
			}
		}
	}
}

// The apps' icons are the page's images named app-<id>.
func TestAppIcons(t *testing.T) {
	assets := map[string]PageAsset{
		AssetLogo:     {Name: AssetLogo, Hash: "1"},
		"app-happ":    {Name: "app-happ", Hash: "2"},
		"app-own-a1":  {Name: "app-own-a1", Hash: "3"},
		"application": {Name: "application", Hash: "4"},
	}
	want := map[string]PageAsset{"happ": assets["app-happ"], "own-a1": assets["app-own-a1"]}
	if got := AppIcons(assets); !reflect.DeepEqual(got, want) {
		t.Fatalf("icons %+v", got)
	}
	if AppIconAsset("own-a1") != "app-own-a1" {
		t.Fatal(AppIconAsset("own-a1"))
	}
}

func TestHas(t *testing.T) {
	c := withOwn(ownApp("own-a1"))
	for id, want := range map[string]bool{"happ": true, "hiddify": true, "own-a1": true, "own-b2": false, "flclash": false, "": false} {
		if c.Has(id) != want {
			t.Errorf("Has(%q) = %v", id, !want)
		}
	}
}

// An own app taken out of the document takes its icon along; the built-in apps' icons and
// the page's other images stay.
func TestOrphanIcons(t *testing.T) {
	c := withOwn(ownApp("own-a1"))
	names := []string{AssetBackground, AssetLogo, "app-happ", "app-own-a1", "app-own-gone", "app-flclash"}
	if got := c.OrphanIcons(names); !reflect.DeepEqual(got, []string{"app-own-gone", "app-flclash"}) {
		t.Fatalf("orphans %v", got)
	}
	if got := c.OrphanIcons(nil); len(got) != 0 {
		t.Fatalf("orphans of nothing: %v", got)
	}
}
