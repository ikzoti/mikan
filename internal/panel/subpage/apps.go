package subpage

import (
	"maps"
	"slices"

	"mikan/internal/sub/apps"
)

// CatalogApp is an app of the page's catalogue on one platform, as the page gets it: what
// the page shows of it, the link that adds the subscription to it and where it is downloaded.
type CatalogApp struct {
	ID     string      `json:"id" doc:"У встроенных это id из списка известных приложений (GET /api/v1/sub-apps): заблокированное там страница не показывает. У своих приложений id начинается с own-"`
	Name   string      `json:"name"`
	Family string      `json:"family" enum:"mihomo,xray,singbox,other" doc:"Ядро приложения"`
	Own    bool        `json:"own" doc:"Своё приложение админа (apps.own)"`
	Icon   CatalogIcon `json:"icon"`
	Link   string      `json:"link" doc:"Ссылка, которая добавляет подписку в приложение. Подстановки: {url} (адрес подписки как есть), {url_enc} (он же в percent-encoding), {name} (бренд в percent-encoding). Пусто у своего приложения без кнопки «Добавить»"`
	Crypt  bool        `json:"crypt" doc:"Открывает шифрованную ссылку Happ вместо link, когда панель её даёт"`
	Note   string      `json:"note" doc:"Подпись под названием. У встроенных это её ключ: easiest, free, stable, openSource, modern, bestWindows, tun, oneButton, allProtocols, everything; у своих приложений это текст"`
	// I18n are an own app's note in the page's languages: a built-in app's is a key the page
	// words in every language itself.
	I18n map[string]NoteText `json:"i18n,omitempty" doc:"Подпись своего приложения на языках страницы (ru, en); где перевода нет, note"`
	// Downloads is never nil: no buttons is an empty list.
	Downloads []AppDownload `json:"downloads" doc:"Кнопки «Скачать»"`
	Doc       int64         `json:"doc" doc:"Инструкция по шагам: id из GET /api/v1/sub-docs или 0, если её нет"`
}

// AppDownload is a button that leads to where the app is downloaded: a store, the releases
// on GitHub, the admin's own copy or any other page.
type AppDownload struct {
	Kind  string               `json:"kind" enum:"appstore,googleplay,github,mirror,link" doc:"appstore, googleplay и github: магазины и релизы на GitHub; mirror: своя копия приложения; link: другая страница, с надписью"`
	URL   string               `json:"url" maxLength:"500" doc:"https://"`
	Label string               `json:"label,omitempty" maxLength:"40" doc:"Надпись кнопки; без неё кнопку называет kind. У link обязательна"`
	I18n  map[string]LabelText `json:"i18n,omitempty" doc:"Надпись кнопки на языках страницы (ru, en); где перевода нет, label"`
}

// The kinds of a download button. The page words all but KindLink by itself.
const (
	KindAppStore   = "appstore"
	KindGooglePlay = "googleplay"
	KindGitHub     = "github"
	KindMirror     = "mirror"
	KindLink       = "link"
)

var downloadKinds = []string{KindAppStore, KindGooglePlay, KindGitHub, KindMirror, KindLink}

// AppCatalog is the catalogue of each platform, in the order the page lists it where the
// page orders nothing (PageAppList).
type AppCatalog struct {
	IOS     []CatalogApp `json:"ios"`
	Android []CatalogApp `json:"android"`
	Windows []CatalogApp `json:"windows"`
	MacOS   []CatalogApp `json:"macos"`
	Linux   []CatalogApp `json:"linux"`
}

// lists are the platforms' lists in the order of Platforms.
func (c *AppCatalog) lists() []*[]CatalogApp {
	return []*[]CatalogApp{&c.IOS, &c.Android, &c.Windows, &c.MacOS, &c.Linux}
}

// onlyDocs drops the apps' instructions that are not among docs.
func (c *AppCatalog) onlyDocs(docs map[int64]bool) {
	for _, list := range c.lists() {
		for i := range *list {
			if !docs[(*list)[i].Doc] {
				(*list)[i].Doc = 0
			}
		}
	}
}

// dropDownloads leaves the apps with no download buttons: a page that turns them off sends
// none of their links (PageApps.DownloadsOff).
func (c *AppCatalog) dropDownloads() {
	for _, list := range c.lists() {
		for i := range *list {
			(*list)[i].Downloads = []AppDownload{}
		}
	}
}

// preset is a built-in app: the admin may change where it is downloaded and its instructions,
// not what it is.
type preset struct {
	id, name, link string
	// crypt: the app opens the panel's encrypted link of Happ when the panel gives one, so the
	// subscription's address stays hidden.
	crypt bool
}

// presets are the built-in apps, by the ids of apps.Catalog. The names are the page's own,
// which may differ from the panel's list (Clash Verge Rev there is Clash Verge). Mikan and
// Kabosu each have a scheme of their own: the profile lands in them, not in whichever app
// holds clash://.
var presets = []preset{
	{id: "mikan", name: "Mikan", link: "mikan://install-config?url={url_enc}&name={name}"},
	{id: "kabosu", name: "Kabosu", link: "kabosu://install-config?url={url_enc}&name={name}"},
	{id: "koala", name: "Koala Clash", link: "koala-clash://install-config?url={url_enc}&name={name}"},
	{id: "clashfest", name: "ClashFest", link: "clashfest://install-config?url={url_enc}&name={name}"},
	{id: "slothclash", name: "SlothClash", link: "slothclash://install-config?url={url_enc}&name={name}"},
	{id: "verge", name: "Clash Verge Rev", link: "clash://install-config?url={url_enc}&name={name}"},
	{id: "happ", name: "Happ", link: "happ://add/{url}", crypt: true},
	{id: "incy", name: "INCY", link: "incy://add/{url}"},
	{id: "v2raytun", name: "v2RayTun", link: "v2raytun://import/{url}"},
	{id: "streisand", name: "Streisand", link: "streisand://import/{url}#{name}"},
	{id: "hiddify", name: "Hiddify", link: "hiddify://import/{url}#{name}"},
}

// presetPlace is a built-in app on a platform's list, with the note it has there.
type presetPlace struct{ id, note string }

// presetShelves are each platform's built-in apps in their default order: the mihomo apps
// first, since they take everything the subscription has, Mikan and Kabosu, mikan's own,
// ahead of them. The notes are keys the page words (sub.notes in web/src/i18n).
var presetShelves = map[string][]presetPlace{
	"ios":     {{"happ", "easiest"}, {"incy", "modern"}, {"streisand", "free"}, {"v2raytun", "stable"}},
	"android": {{"mikan", "everything"}, {"clashfest", "allProtocols"}, {"happ", "easiest"}, {"incy", "modern"}, {"v2raytun", "stable"}, {"hiddify", "openSource"}},
	"windows": {{"kabosu", "tun"}, {"koala", "bestWindows"}, {"slothclash", "tun"}, {"verge", "tun"}, {"hiddify", "easiest"}},
	"macos":   {{"kabosu", "tun"}, {"verge", "tun"}, {"slothclash", "tun"}, {"happ", "oneButton"}, {"hiddify", "openSource"}},
	"linux":   {{"kabosu", "tun"}, {"slothclash", "tun"}, {"verge", "tun"}, {"hiddify", "openSource"}},
}

// presetDownloads are the download buttons the built-in apps come with, by app and platform:
// the store pages and releases each app's developer links from its site or README, checked
// by hand on 2026-11-28; the stores first, then GitHub, then other pages. A store does not
// offer every app in every country, so the admin may replace a platform's buttons
// (PlatformEdit): the Russian App Store has no Happ (one listing for iPhone and Mac), INCY,
// Streisand or Hiddify for Mac, which the App Stores of Kazakhstan and Belarus have. Happ,
// INCY and Streisand get their developer's site as well, which says how to get them. The App
// Store page of v2RayTun and the Google Play page of Happ no longer open: no button there.
var presetDownloads = map[string]map[string][]AppDownload{
	"happ": {
		"ios":     {{Kind: KindAppStore, URL: "https://apps.apple.com/app/id6504287215"}, {Kind: KindLink, URL: "https://happ.info/", Label: "happ.info"}},
		"android": {{Kind: KindGitHub, URL: "https://github.com/Happ-proxy/happ-android/releases/latest"}},
		"macos": {
			{Kind: KindAppStore, URL: "https://apps.apple.com/app/id6504287215"},
			{Kind: KindGitHub, URL: "https://github.com/Happ-proxy/happ-desktop/releases/latest"},
			{Kind: KindLink, URL: "https://happ.info/", Label: "happ.info"},
		},
	},
	"incy": {
		"ios":     {{Kind: KindAppStore, URL: "https://apps.apple.com/app/id6756943388"}, {Kind: KindLink, URL: "https://incy.cc/#downloads", Label: "incy.cc"}},
		"android": {{Kind: KindGooglePlay, URL: "https://play.google.com/store/apps/details?id=llc.itdev.incy"}, {Kind: KindGitHub, URL: "https://github.com/INCY-DEV/incy-platforms/releases/latest"}},
	},
	"streisand": {
		"ios": {{Kind: KindAppStore, URL: "https://apps.apple.com/app/id6450534064"}, {Kind: KindLink, URL: "https://streisand.pages.dev/", Label: "streisand.pages.dev"}},
	},
	"v2raytun": {
		"android": {{Kind: KindGooglePlay, URL: "https://play.google.com/store/apps/details?id=com.v2raytun.android"}},
	},
	"mikan": {
		"android": {{Kind: KindGitHub, URL: "https://github.com/getmikan/MikanApp/releases/latest"}},
	},
	"kabosu": {
		"windows": {{Kind: KindGitHub, URL: "https://github.com/getmikan/Kabosu/releases/latest"}},
		"macos":   {{Kind: KindGitHub, URL: "https://github.com/getmikan/Kabosu/releases/latest"}},
		"linux":   {{Kind: KindGitHub, URL: "https://github.com/getmikan/Kabosu/releases/latest"}},
	},
	"koala": {
		"windows": {{Kind: KindGitHub, URL: "https://github.com/coolcoala/koala-clash/releases/latest"}},
	},
	"clashfest": {
		"android": {
			{Kind: KindGitHub, URL: "https://github.com/Nemu-x/ClashFest/releases/latest"},
			{Kind: KindLink, URL: "https://f-droid.org/packages/com.nemu.clashfest.clash.alpha/", Label: "F-Droid"},
		},
	},
	"slothclash": {
		"windows": {{Kind: KindGitHub, URL: "https://github.com/Nemu-x/SlothClash/releases/latest"}},
		"macos":   {{Kind: KindGitHub, URL: "https://github.com/Nemu-x/SlothClash/releases/latest"}},
		"linux":   {{Kind: KindGitHub, URL: "https://github.com/Nemu-x/SlothClash/releases/latest"}},
	},
	"verge": {
		"windows": {{Kind: KindGitHub, URL: "https://github.com/clash-verge-rev/clash-verge-rev/releases/latest"}},
		"macos":   {{Kind: KindGitHub, URL: "https://github.com/clash-verge-rev/clash-verge-rev/releases/latest"}},
		"linux":   {{Kind: KindGitHub, URL: "https://github.com/clash-verge-rev/clash-verge-rev/releases/latest"}},
	},
	"hiddify": {
		"android": {{Kind: KindGooglePlay, URL: "https://play.google.com/store/apps/details?id=app.hiddify.com"}, {Kind: KindGitHub, URL: "https://github.com/hiddify/hiddify-app/releases/latest"}},
		"windows": {{Kind: KindGitHub, URL: "https://github.com/hiddify/hiddify-app/releases/latest"}, {Kind: KindLink, URL: "https://apps.microsoft.com/detail/9ndgwm32bnxk", Label: "Microsoft Store"}},
		"macos":   {{Kind: KindAppStore, URL: "https://apps.apple.com/app/id6596777532"}, {Kind: KindGitHub, URL: "https://github.com/hiddify/hiddify-app/releases/latest"}},
		"linux":   {{Kind: KindGitHub, URL: "https://github.com/hiddify/hiddify-app/releases/latest"}},
	},
}

// presetByID and shelfOf (the platforms each built-in app is on, with its place there) are
// the tables above by id.
var (
	presetByID = func() map[string]preset {
		m := make(map[string]preset, len(presets))
		for _, p := range presets {
			m[p.id] = p
		}
		return m
	}()
	shelfOf = func() map[string]map[string]presetPlace {
		m := map[string]map[string]presetPlace{}
		for platform, shelf := range presetShelves {
			for _, place := range shelf {
				if m[place.id] == nil {
					m[place.id] = map[string]presetPlace{}
				}
				m[place.id][platform] = place
			}
		}
		return m
	}()
)

// presetApp is the built-in app at place on platform as it comes.
func presetApp(platform string, place presetPlace) CatalogApp {
	p := presetByID[place.id]
	// Every built-in app is a known one (TestPresetsAreKnownApps).
	known, _ := apps.Lookup(p.id)
	return CatalogApp{
		ID: p.id, Name: p.name, Family: string(known.Family), Icon: CatalogIcon{Kind: IconLetter}, Link: p.link, Crypt: p.crypt, Note: place.note,
		Downloads: cloneDownloads(presetDownloads[p.id][platform]),
	}
}

// cloneDownloads is list as a copy that shares nothing with it, the translations of its
// buttons included; never nil, as no buttons is an empty list.
func cloneDownloads(list []AppDownload) []AppDownload {
	out := make([]AppDownload, len(list))
	for i, d := range list {
		d.I18n = maps.Clone(d.I18n)
		out[i] = d
	}
	return out
}

// Catalog is the catalogue every page shows, a platform at a time: the built-in apps in their
// default order with what the admin changed of them, and the admin's own in the document's
// order. An own app on mihomo comes after the built-in apps on mihomo, which lead the list
// (presetShelves), and ahead of the others; the rest of the own apps go last. icons are the
// apps' uploaded icons by app id (AppIcons). It shares nothing with c.
func (c AppsConfig) Catalog(icons map[string]PageAsset) AppCatalog {
	var out AppCatalog
	for i, list := range out.lists() {
		platform := Platforms[i]
		built, own := c.presetsOn(platform, icons), c.ownOn(platform, icons)
		lead := slices.IndexFunc(built, func(a CatalogApp) bool { return !onMihomo(a) })
		if lead < 0 {
			lead = len(built)
		}
		ownMihomo := slices.DeleteFunc(slices.Clone(own), func(a CatalogApp) bool { return !onMihomo(a) })
		*list = slices.Concat(built[:lead], ownMihomo, built[lead:], slices.DeleteFunc(own, onMihomo))
	}
	return out
}

func onMihomo(a CatalogApp) bool { return a.Family == string(apps.FamilyMihomo) }

// keepShown leaves in c the apps page shows, each platform by its own list (Service.catalogOf).
func (c *AppCatalog) keepShown(page PageApps) {
	hides := page.lists()
	for i, list := range c.lists() {
		*list = slices.DeleteFunc(*list, func(a CatalogApp) bool { return slices.Contains(hides[i].Hidden, a.ID) })
	}
}

// presetsOn are the built-in apps on platform in their default order, as the admin changed them.
func (c AppsConfig) presetsOn(platform string, icons map[string]PageAsset) []CatalogApp {
	shelf := presetShelves[platform]
	out := make([]CatalogApp, 0, len(shelf))
	for _, place := range shelf {
		a := presetApp(platform, place)
		e := c.Presets[place.id]
		a.Icon = catalogIcon(a.ID, e.Icon, icons)
		if p, ok := e.Platforms[platform]; ok {
			a.Doc = p.Doc
			if p.Downloads != nil {
				a.Downloads = cloneDownloads(*p.Downloads)
			}
		}
		out = append(out, a)
	}
	return out
}

// Presets is the catalogue as it comes, with nothing of the admin's.
func Presets() AppCatalog { return AppsConfig{}.Catalog(nil) }
