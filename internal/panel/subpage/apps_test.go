package subpage

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"mikan/internal/sub/apps"
)

// ids are the ids of a platform's apps in their order.
func ids(list []CatalogApp) string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.ID)
	}
	return strings.Join(out, ",")
}

// find is the app id of a platform's list; it fails the test when there is none.
func find(t *testing.T, list []CatalogApp, id string) CatalogApp {
	t.Helper()
	i := slices.IndexFunc(list, func(a CatalogApp) bool { return a.ID == id })
	if i < 0 {
		t.Fatalf("no %s in %s", id, ids(list))
	}
	return list[i]
}

// The page's own apps are apps the panel knows by their User-Agent: the admin's block of an
// app (info.blocked_apps) leaves it out of the page by the same id, and the family is the
// one the panel tells it by.
func TestPresetsAreKnownApps(t *testing.T) {
	families := []string{string(apps.FamilyMihomo), string(apps.FamilyXray), string(apps.FamilySingBox), string(apps.FamilyOther)}
	c := Presets()
	seen := map[string]bool{}
	for i, list := range c.lists() {
		for _, a := range *list {
			k, ok := apps.Lookup(a.ID)
			if !ok {
				t.Errorf("%s (%s): not in apps.Catalog", a.ID, Platforms[i])
				continue
			}
			if a.Family != string(k.Family) || !slices.Contains(families, a.Family) {
				t.Errorf("%s: family %q, apps.Catalog has %q", a.ID, a.Family, k.Family)
			}
			seen[a.ID] = true
		}
	}
	for _, p := range presets {
		if !seen[p.id] {
			t.Errorf("%s stands on no platform", p.id)
		}
	}
}

// The built-in apps add the subscription with the links they always had (web/src/sub/apps.ts
// before #278), the Happ button with the panel's encrypted link when there is one.
func TestPresetLinks(t *testing.T) {
	want := map[string]string{
		"mikan":      "mikan://install-config?url={url_enc}&name={name}",
		"kabosu":     "kabosu://install-config?url={url_enc}&name={name}",
		"koala":      "koala-clash://install-config?url={url_enc}&name={name}",
		"clashfest":  "clashfest://install-config?url={url_enc}&name={name}",
		"slothclash": "slothclash://install-config?url={url_enc}&name={name}",
		"verge":      "clash://install-config?url={url_enc}&name={name}",
		"happ":       "happ://add/{url}",
		"incy":       "incy://add/{url}",
		"v2raytun":   "v2raytun://import/{url}",
		"streisand":  "streisand://import/{url}#{name}",
		"hiddify":    "hiddify://import/{url}#{name}",
	}
	if len(presets) != len(want) {
		t.Fatalf("%d built-in apps, want %d", len(presets), len(want))
	}
	c := Presets()
	for i, list := range c.lists() {
		for _, a := range *list {
			if a.Link != want[a.ID] {
				t.Errorf("%s on %s: %q, want %q", a.ID, Platforms[i], a.Link, want[a.ID])
			}
			if code := AppLinkProblem(a.Link); code != "" {
				t.Errorf("%s: its own link breaks the rules: %s", a.ID, code)
			}
			if a.Crypt != (a.ID == "happ") {
				t.Errorf("%s: crypt %v", a.ID, a.Crypt)
			}
		}
	}
}

// Each platform lists the mihomo apps first, mikan's own ahead of them, with the names and
// notes the page always showed.
func TestPresetOrder(t *testing.T) {
	c := Presets()
	for platform, want := range map[string]string{
		"ios":     "happ:easiest,incy:modern,streisand:free,v2raytun:stable",
		"android": "mikan:everything,clashfest:allProtocols,happ:easiest,incy:modern,v2raytun:stable,hiddify:openSource",
		"windows": "kabosu:tun,koala:bestWindows,slothclash:tun,verge:tun,hiddify:easiest",
		"macos":   "kabosu:tun,verge:tun,slothclash:tun,happ:oneButton,hiddify:openSource",
		"linux":   "kabosu:tun,slothclash:tun,verge:tun,hiddify:openSource",
	} {
		list := *c.lists()[slices.Index(Platforms, platform)]
		got := make([]string, 0, len(list))
		for _, a := range list {
			got = append(got, a.ID+":"+a.Note)
			if a.Downloads == nil || len(a.Downloads) != len(presetDownloads[a.ID][platform]) || a.Doc != 0 {
				t.Errorf("%s on %s: downloads %v, doc %d", a.ID, platform, a.Downloads, a.Doc)
			}
		}
		if strings.Join(got, ",") != want {
			t.Errorf("%s: %s, want %s", platform, strings.Join(got, ","), want)
		}
	}
	names := map[string]string{}
	for _, list := range c.lists() {
		for _, a := range *list {
			names[a.ID] = a.Name
		}
	}
	if names["verge"] != "Clash Verge Rev" || names["koala"] != "Koala Clash" || names["v2raytun"] != "v2RayTun" || names["incy"] != "INCY" {
		t.Fatalf("the page's names: %v", names)
	}
}

// The links the built-in apps come with are held to the admin's rules.
func TestPresetDownloadsKeepTheRules(t *testing.T) {
	for id, byPlatform := range presetDownloads {
		for platform, list := range byPlatform {
			if _, ok := shelfOf[id][platform]; !ok {
				t.Errorf("%s is not on %s", id, platform)
			}
			c := AppsConfig{Presets: map[string]PresetEdit{id: {Platforms: map[string]PlatformEdit{platform: {Downloads: &list}}}}}
			if ps := NormalizeApps(&c); len(ps) != 0 {
				t.Errorf("%s on %s: %+v", id, platform, ps)
			}
		}
	}
}

// The built-in apps come with their developers' own pages, the stores first: a store's
// button leads to that store, GitHub's to the app's latest release, and another page (F-Droid,
// the Microsoft Store, the developer's site) is named by its label.
func TestPresetDownloads(t *testing.T) {
	prefixes := map[string]string{KindAppStore: "https://apps.apple.com/app/id", KindGooglePlay: "https://play.google.com/store/apps/details?id=", KindGitHub: "https://github.com/"}
	want := map[string]string{
		"happ/ios": "appstore,link", "happ/android": "github", "happ/macos": "appstore,github,link",
		"incy/ios": "appstore,link", "incy/android": "googleplay,github",
		"streisand/ios":    "appstore,link",
		"v2raytun/android": "googleplay",
		"mikan/android":    "github",
		"kabosu/windows":   "github", "kabosu/macos": "github", "kabosu/linux": "github",
		"koala/windows":      "github",
		"clashfest/android":  "github,link",
		"slothclash/windows": "github", "slothclash/macos": "github", "slothclash/linux": "github",
		"verge/windows": "github", "verge/macos": "github", "verge/linux": "github",
		"hiddify/android": "googleplay,github", "hiddify/windows": "github,link", "hiddify/macos": "appstore,github", "hiddify/linux": "github",
	}
	got := map[string]string{}
	presets := Presets()
	for i, list := range presets.lists() {
		for _, a := range *list {
			var kinds []string
			for _, d := range a.Downloads {
				kinds = append(kinds, d.Kind)
				if prefix, ok := prefixes[d.Kind]; ok && !strings.HasPrefix(d.URL, prefix) {
					t.Errorf("%s on %s: %s leads to %s", a.ID, Platforms[i], d.Kind, d.URL)
				}
				if d.Kind == KindGitHub && !strings.HasSuffix(d.URL, "/releases/latest") {
					t.Errorf("%s on %s: GitHub's button leads to %s, not the latest release", a.ID, Platforms[i], d.URL)
				}
				if d.Kind != KindLink && d.Label != "" {
					t.Errorf("%s on %s: a %s button with a label of its own", a.ID, Platforms[i], d.Kind)
				}
			}
			if kinds != nil {
				got[a.ID+"/"+Platforms[i]] = strings.Join(kinds, ",")
			}
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the built-in downloads:\n%v\nwant\n%v", got, want)
	}
}

func downloads(d ...AppDownload) *[]AppDownload { return &d }

func edit(id, platform string, e PlatformEdit) AppsConfig {
	return AppsConfig{Presets: map[string]PresetEdit{id: {Platforms: map[string]PlatformEdit{platform: e}}}}
}

func TestNormalizeAppsRefuses(t *testing.T) {
	store := AppDownload{Kind: KindAppStore, URL: "https://apps.example.com/happ"}
	six := downloads(store, store, store, store, store, store)
	cases := []struct {
		name  string
		c     AppsConfig
		field string
		code  string
	}{
		{"unknown app", edit("nonesuch", "ios", PlatformEdit{Doc: 1}), "presets.nonesuch", "app_unknown"},
		{"a known app that is not built in", edit("flclash", "android", PlatformEdit{Doc: 1}), "presets.flclash", "app_unknown"},
		{"a platform the app is not on", edit("streisand", "android", PlatformEdit{Doc: 1}), "presets.streisand.platforms.android", "value_invalid"},
		{"no such platform", edit("happ", "beos", PlatformEdit{Doc: 1}), "presets.happ.platforms.beos", "value_invalid"},
		{"too many downloads", edit("happ", "ios", PlatformEdit{Downloads: six}), "presets.happ.platforms.ios.downloads", "too_many"},
		{"unknown kind", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{Kind: "torrent", URL: "https://example.com/a"})}), "presets.happ.platforms.ios.downloads[0].kind", "value_invalid"},
		{"no kind", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{URL: "https://example.com/a"})}), "presets.happ.platforms.ios.downloads[0].kind", "value_invalid"},
		{"http", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{Kind: KindMirror, URL: "http://example.com/happ.ipa"})}), "presets.happ.platforms.ios.downloads[0].url", "url_invalid"},
		{"javascript", edit("happ", "ios", PlatformEdit{Downloads: downloads(store, AppDownload{Kind: KindMirror, URL: "javascript:alert(1)"})}), "presets.happ.platforms.ios.downloads[1].url", "url_invalid"},
		{"an app's scheme", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{Kind: KindLink, URL: "happ://add/x", Label: "Get"})}), "presets.happ.platforms.ios.downloads[0].url", "url_invalid"},
		{"credentials", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{Kind: KindMirror, URL: "https://user:pw@example.com/a"})}), "presets.happ.platforms.ios.downloads[0].url", "url_invalid"},
		{"no address", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{Kind: KindMirror})}), "presets.happ.platforms.ios.downloads[0].url", "url_invalid"},
		{"too long a link", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{Kind: KindMirror, URL: "https://example.com/" + strings.Repeat("a", 500)})}), "presets.happ.platforms.ios.downloads[0].url", "url_invalid"},
		{"label on two lines", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{Kind: KindMirror, URL: "https://example.com/a", Label: "a\nb"})}), "presets.happ.platforms.ios.downloads[0].label", "one_line"},
		{"label too long", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{Kind: KindMirror, URL: "https://example.com/a", Label: strings.Repeat("я", 41)})}), "presets.happ.platforms.ios.downloads[0].label", "too_long"},
		{"link without a label", edit("happ", "ios", PlatformEdit{Downloads: downloads(AppDownload{Kind: KindLink, URL: "https://example.com/a", Label: "  "})}), "presets.happ.platforms.ios.downloads[0].label", "label_required"},
		{"negative instruction", edit("happ", "ios", PlatformEdit{Doc: -1}), "presets.happ.platforms.ios.doc", "value_invalid"},
		{"an icon of no kind", AppsConfig{Presets: map[string]PresetEdit{"happ": {Icon: &AppIcon{Kind: "svg"}}}}, "presets.happ.icon.kind", "value_invalid"},
		{"an emoji icon without its emoji", AppsConfig{Presets: map[string]PresetEdit{"happ": {Icon: &AppIcon{Kind: IconEmoji}}}}, "presets.happ.icon.emoji", "required"},
		{"the icon of an unknown app", AppsConfig{Presets: map[string]PresetEdit{"nonesuch": {Icon: &AppIcon{Kind: IconEmoji, Emoji: "🚀"}}}}, "presets.nonesuch", "app_unknown"},
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

// What cannot be shown is dropped, so a stored document is shown as it was cleaned.
func TestNormalizeAppsDropsWhatIsWrong(t *testing.T) {
	c := AppsConfig{Presets: map[string]PresetEdit{
		"nonesuch":  {Platforms: map[string]PlatformEdit{"ios": {Doc: 1}}},
		"streisand": {Platforms: map[string]PlatformEdit{"android": {Doc: 1}}},
		"happ": {Platforms: map[string]PlatformEdit{"ios": {Doc: -4, Downloads: downloads(
			AppDownload{Kind: KindMirror, URL: "http://example.com/a"},
			AppDownload{Kind: KindGitHub, URL: "https://github.com/example/happ/releases"},
		)}}},
	}}
	if ps := NormalizeApps(&c); len(ps) != 4 {
		t.Fatalf("problems: %+v", ps)
	}
	want := AppsConfig{Presets: map[string]PresetEdit{"happ": {Platforms: map[string]PlatformEdit{"ios": {Downloads: downloads(AppDownload{Kind: KindGitHub, URL: "https://github.com/example/happ/releases"})}}}}, Own: []OwnApp{}}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("cleaned to %+v", c)
	}
}

func TestNormalizeAppsKeepsWhatIsRight(t *testing.T) {
	var zero AppsConfig
	if ps := NormalizeApps(&zero); len(ps) != 0 || zero.Presets == nil || zero.Own == nil {
		t.Fatalf("the zero document: %+v %+v", ps, zero)
	}
	c := AppsConfig{Presets: map[string]PresetEdit{
		"happ": {Platforms: map[string]PlatformEdit{
			"ios": {Doc: 7, Downloads: downloads(
				AppDownload{Kind: " appstore ", URL: " https://apps.example.com/happ "},
				AppDownload{Kind: KindMirror, URL: "https://mirror.example.com/happ.ipa"},
				AppDownload{Kind: KindLink, URL: "https://example.com/beta", Label: "  Бета  "},
			)},
			// An empty list: no buttons at all.
			"android": {Downloads: downloads()},
			// Nothing changed: left out.
			"macos": {},
		}},
		"hiddify": {Platforms: map[string]PlatformEdit{"linux": {Doc: 3}}},
		// An app nothing is changed of is left out too.
		"incy": {Platforms: map[string]PlatformEdit{}},
	}}
	if ps := NormalizeApps(&c); len(ps) != 0 {
		t.Fatalf("problems: %+v", ps)
	}
	ios := c.Presets["happ"].Platforms["ios"]
	if ios.Doc != 7 || len(*ios.Downloads) != 3 || !reflect.DeepEqual((*ios.Downloads)[0], AppDownload{Kind: KindAppStore, URL: "https://apps.example.com/happ"}) || (*ios.Downloads)[2].Label != "Бета" {
		t.Fatalf("cleaned wrong: %+v", ios)
	}
	if android := c.Presets["happ"].Platforms["android"]; android.Downloads == nil || len(*android.Downloads) != 0 {
		t.Fatalf("an empty list became %+v", android)
	}
	if _, ok := c.Presets["happ"].Platforms["macos"]; ok || len(c.Presets) != 2 {
		t.Fatalf("edits that change nothing are kept: %+v", c.Presets)
	}
	again := c
	if ps := NormalizeApps(&again); len(ps) != 0 || !reflect.DeepEqual(again, c) {
		t.Fatalf("a clean document changed: %+v", again)
	}
	// Stored and read back, an empty list stays a list: no buttons, not the app's own.
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back AppsConfig
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, c) {
		t.Fatalf("does not read back: %v %s", err, raw)
	}
	if !strings.Contains(string(raw), `"android":{"downloads":[]}`) || strings.Contains(string(raw), "null") {
		t.Fatalf("stored as %s", raw)
	}
}

// An edit replaces what it names on its own platform; the rest is the app's own.
func TestCatalogMerges(t *testing.T) {
	mirror := AppDownload{Kind: KindMirror, URL: "https://mirror.example.com/hiddify.exe"}
	c := AppsConfig{Presets: map[string]PresetEdit{"hiddify": {Platforms: map[string]PlatformEdit{
		"windows": {Downloads: downloads(mirror), Doc: 5},
		"linux":   {Downloads: downloads()},
		"android": {Doc: 9},
	}}}}
	if ps := NormalizeApps(&c); len(ps) != 0 {
		t.Fatalf("problems: %+v", ps)
	}
	got, presets := c.Catalog(nil), Presets()
	for i := range Platforms {
		if ids(*got.lists()[i]) != ids(*presets.lists()[i]) {
			t.Errorf("%s: the order moved: %s", Platforms[i], ids(*got.lists()[i]))
		}
	}
	if w := find(t, got.Windows, "hiddify"); !reflect.DeepEqual(w.Downloads, []AppDownload{mirror}) || w.Doc != 5 || w.Name != "Hiddify" || w.Note != "easiest" {
		t.Errorf("windows: %+v", w)
	}
	if l := find(t, got.Linux, "hiddify"); l.Downloads == nil || len(l.Downloads) != 0 || l.Doc != 0 {
		t.Errorf("linux, an empty list: %+v", l)
	}
	if a := find(t, got.Android, "hiddify"); !reflect.DeepEqual(a.Downloads, presetDownloads["hiddify"]["android"]) || a.Doc != 9 {
		t.Errorf("android, the instruction alone: %+v", a)
	}
	if m := find(t, got.MacOS, "hiddify"); !reflect.DeepEqual(m, find(t, presets.MacOS, "hiddify")) {
		t.Errorf("macos is not edited: %+v", m)
	}
	if p := find(t, presets.Windows, "hiddify"); len(p.Downloads) == 0 || !reflect.DeepEqual(p.Downloads, presetDownloads["hiddify"]["windows"]) {
		t.Errorf("the app's own downloads: %+v", p)
	}
	// The catalogue is a copy: what the page does to it reaches neither the document nor the
	// built-in table.
	find(t, got.Windows, "hiddify").Downloads[0].URL = "https://changed.example.com"
	find(t, got.Android, "hiddify").Downloads[0].URL = "https://changed.example.com"
	if !reflect.DeepEqual((*c.Presets["hiddify"].Platforms["windows"].Downloads)[0], mirror) || presetDownloads["hiddify"]["android"][0].URL == "https://changed.example.com" {
		t.Fatal("the catalogue shares a list")
	}
}

// The page links only the instructions it can open: a draft or a deleted one is no step.
func TestCatalogOnlyDocs(t *testing.T) {
	c := AppsConfig{Presets: map[string]PresetEdit{
		"happ":    {Platforms: map[string]PlatformEdit{"ios": {Doc: 5}, "macos": {Doc: 6}}},
		"hiddify": {Platforms: map[string]PlatformEdit{"linux": {Doc: 5}}},
	}}
	got := c.Catalog(nil)
	got.onlyDocs(map[int64]bool{5: true})
	if find(t, got.IOS, "happ").Doc != 5 || find(t, got.Linux, "hiddify").Doc != 5 || find(t, got.MacOS, "happ").Doc != 0 {
		t.Fatalf("instructions: %+v %+v %+v", got.IOS, got.Linux, got.MacOS)
	}
}

func TestDocRefs(t *testing.T) {
	c := AppsConfig{Presets: map[string]PresetEdit{
		"happ":    {Platforms: map[string]PlatformEdit{"macos": {Doc: 4}, "ios": {Doc: 2}, "android": {Downloads: downloads()}}},
		"hiddify": {Platforms: map[string]PlatformEdit{"linux": {Doc: 4}}},
	}, Own: []OwnApp{
		{ID: "own-b", Platforms: map[string]OwnPlatform{"windows": {Doc: 6}, "ios": {Doc: 4}, "linux": {}}},
		{ID: "own-a", Platforms: map[string]OwnPlatform{"android": {Doc: 2}}},
	}}
	want := []DocRef{
		{Field: "presets.happ.platforms.ios.doc", ID: 2},
		{Field: "presets.happ.platforms.macos.doc", ID: 4},
		{Field: "presets.hiddify.platforms.linux.doc", ID: 4},
		{Field: "own[0].platforms.ios.doc", ID: 4},
		{Field: "own[0].platforms.windows.doc", ID: 6},
		{Field: "own[1].platforms.android.doc", ID: 2},
	}
	if got := c.DocRefs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("refs %+v", got)
	}
}

// An instruction deleted since an app was given it leaves the document, and an edit that
// changes nothing without it goes too; the rest stays as it was.
func TestDropDocs(t *testing.T) {
	mirror := AppDownload{Kind: KindMirror, URL: "https://files.example.com/happ.apk"}
	rocket := &AppIcon{Kind: IconEmoji, Emoji: "🚀"}
	c := AppsConfig{Presets: map[string]PresetEdit{
		"happ":      {Platforms: map[string]PlatformEdit{"ios": {Doc: 2}, "android": {Downloads: downloads(mirror), Doc: 2}, "macos": {Doc: 3}}},
		"hiddify":   {Platforms: map[string]PlatformEdit{"linux": {Doc: 2}}},
		"streisand": {Icon: rocket, Platforms: map[string]PlatformEdit{"ios": {Doc: 2}}},
	}, Own: []OwnApp{{ID: "own-a", Name: "Наш", Family: "xray", Platforms: map[string]OwnPlatform{"ios": {Note: "Подпись", Doc: 2}, "android": {Doc: 3}}}}}
	c.DropDocs([]DocRef{{Field: "presets.happ.platforms.ios.doc", ID: 2}})
	want := AppsConfig{Presets: map[string]PresetEdit{
		"happ": {Platforms: map[string]PlatformEdit{"android": {Downloads: downloads(mirror)}, "macos": {Doc: 3}}},
		// The icon is still an edit of its own.
		"streisand": {Icon: rocket},
	}, Own: []OwnApp{{ID: "own-a", Name: "Наш", Family: "xray", Platforms: map[string]OwnPlatform{"ios": {Note: "Подпись"}, "android": {Doc: 3}}}}}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("got %+v", c)
	}
	if problems := NormalizeApps(&c); len(problems) != 0 {
		t.Fatalf("the document left is not one PUT takes: %+v", problems)
	}
	c.DropDocs(nil)
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("nothing gone changed it: %+v", c)
	}
}

// A page's data holds the apps the page shows: one hidden there is not in it. A group's own page
// keeps those the panel's page shows too, so only an app both hide is left out.
func TestKeepShown(t *testing.T) {
	c := withOwn(OwnApp{ID: "own-vip", Name: "Для своих", Family: "xray", Platforms: map[string]OwnPlatform{"ios": {}, "android": {}}})
	if ps := NormalizeApps(&c); len(ps) != 0 {
		t.Fatalf("problems: %+v", ps)
	}
	panel, group := Default().Apps, Default().Apps
	panel.IOS.Hidden = []string{"own-vip", "happ", "streisand"}
	panel.Android.Hidden = []string{"own-vip"}
	group.IOS = PageAppList{Order: []string{"own-vip"}, Hidden: []string{"incy", "streisand"}}
	presets := Presets()

	onPanel := c.Catalog(nil)
	onPanel.keepShown(panel)
	for platform, want := range map[string]string{
		"ios":     "incy,v2raytun",
		"android": ids(presets.Android),
		"windows": ids(presets.Windows),
	} {
		if list := *onPanel.lists()[slices.Index(Platforms, platform)]; ids(list) != want {
			t.Errorf("the panel's page, %s: %s, want %s", platform, ids(list), want)
		}
	}
	onGroup := c.Catalog(nil)
	onGroup.keepShown(group, panel)
	for platform, want := range map[string]string{
		"ios":     "happ,incy,v2raytun,own-vip",
		"android": ids(presets.Android) + ",own-vip",
	} {
		if list := *onGroup.lists()[slices.Index(Platforms, platform)]; ids(list) != want {
			t.Errorf("the group's page, %s: %s, want %s", platform, ids(list), want)
		}
	}
	all := c.Catalog(nil)
	all.keepShown()
	if !reflect.DeepEqual(all, c.Catalog(nil)) {
		t.Fatal("no page hides nothing")
	}
}
