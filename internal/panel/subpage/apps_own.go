package subpage

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"mikan/internal/sub/apps"
)

// OwnApp is an app the admin adds to the page: what the page shows of it, the link that adds
// the subscription to it and where it is downloaded. On each of its platforms it comes after
// the built-in apps, one on mihomo after the built-in ones on mihomo, in the order of the list
// (AppsConfig.Catalog).
type OwnApp struct {
	ID        string                 `json:"id" maxLength:"16" doc:"own- и от 1 до 12 букв a-z и цифр; его выбирает редактор, потом он не меняется"`
	Name      string                 `json:"name" maxLength:"40"`
	Family    string                 `json:"family" enum:"mihomo,xray,singbox,other" doc:"Ядро приложения. Приложение на mihomo идёт за встроенными на mihomo, раньше остальных; другие идут после всех встроенных"`
	Icon      *AppIcon               `json:"icon,omitempty" doc:"Значок; без поля первая буква названия"`
	Link      string                 `json:"link" maxLength:"500" doc:"Ссылка, которая добавляет подписку, с теми же подстановками, что у встроенных: {url}, {url_enc}, {name}. Пустая ссылка: без кнопки «Добавить»"`
	Platforms map[string]OwnPlatform `json:"platforms" doc:"Платформы приложения, хотя бы одна: ios, android, windows, macos, linux"`
}

// OwnPlatform is an own app on one platform.
type OwnPlatform struct {
	Note      string              `json:"note,omitempty" maxLength:"80" doc:"Подпись под названием"`
	Downloads []AppDownload       `json:"downloads,omitempty" maxItems:"5" doc:"Кнопки «Скачать»"`
	Doc       int64               `json:"doc,omitempty" minimum:"0" doc:"Инструкция по шагам: id из GET /api/v1/sub-docs; 0 или нет поля: без инструкции"`
	I18n      map[string]NoteText `json:"i18n,omitempty" doc:"Подпись на языках страницы (ru, en); где перевода нет, note"`
}

// MaxOwnApps is how many apps of their own the admin may add.
const MaxOwnApps = 20

const (
	// maxAppName is how long an own app's name may be.
	maxAppName = 40
	// maxNote is how long the line under an own app's name may be.
	maxNote = 80
)

// ownIDRe is an own app's id: own- and a few letters the editor picks, as it does for the
// own blocks. The prefix keeps it apart from the built-in apps' ids.
var ownIDRe = regexp.MustCompile(`^own-[a-z0-9]{1,12}$`)

// families are the cores an app may run, as apps.Catalog tells them.
var families = []string{string(apps.FamilyMihomo), string(apps.FamilyXray), string(apps.FamilySingBox), string(apps.FamilyOther)}

// normalizeOwnApps cleans the admin's own apps: an app the page cannot name or place (a
// wrong id, no name, no platform) goes, and of the rest what is wrong is left out.
func normalizeOwnApps(list []OwnApp) ([]OwnApp, []Problem) {
	var out []Problem
	if len(list) > MaxOwnApps {
		out = append(out, Problem{Field: "own", Code: "too_many", Value: MaxOwnApps})
		list = list[:MaxOwnApps]
	}
	clean := make([]OwnApp, 0, len(list))
	seen := map[string]bool{}
	for i, a := range list {
		field := "own[" + strconv.Itoa(i) + "]"
		ok, ps := normalizeOwnApp(field, &a)
		out = append(out, ps...)
		switch {
		case !ok:
		case seen[a.ID]:
			out = append(out, Problem{Field: field + ".id", Code: "app_duplicate", Value: a.ID})
		default:
			seen[a.ID] = true
			clean = append(clean, a)
		}
	}
	return clean, out
}

// normalizeOwnApp cleans a, the app at field, and says whether the page can show it.
func normalizeOwnApp(field string, a *OwnApp) (bool, []Problem) {
	var out []Problem
	bad := func(name, code string, value any) {
		out = append(out, Problem{Field: field + "." + name, Code: code, Value: value})
	}
	a.ID, a.Name, a.Family, a.Link = strings.TrimSpace(a.ID), strings.TrimSpace(a.Name), strings.TrimSpace(a.Family), strings.TrimSpace(a.Link)
	if !ownIDRe.MatchString(a.ID) {
		bad("id", "app_id_invalid", a.ID)
		return false, out
	}
	if code, value := nameProblem(a.Name); code != "" {
		bad("name", code, value)
		return false, out
	}
	if !slices.Contains(families, a.Family) {
		bad("family", "value_invalid", a.Family)
		a.Family = string(apps.FamilyOther)
	}
	var ps []Problem
	a.Icon, ps = normalizeIcon(field+".icon", a.Icon)
	out = append(out, ps...)
	// No link is an app with no "Add": the page shows how to get it, nothing to open.
	if a.Link != "" {
		if code := AppLinkProblem(a.Link); code != "" {
			bad("link", code, nil)
			a.Link = ""
		}
	}
	a.Platforms, ps = normalizeOwnPlatforms(field+".platforms", a.Platforms)
	out = append(out, ps...)
	if len(a.Platforms) == 0 {
		bad("platforms", "platforms_required", nil)
		return false, out
	}
	return true, out
}

// nameProblem is what is wrong with an own app's name, its code and value; "" when nothing.
func nameProblem(name string) (string, any) {
	if name == "" {
		return "required", nil
	}
	return lineProblem(name, maxAppName)
}

// normalizeOwnPlatforms cleans the platforms of an own app, at field: a platform the page has
// no tab for is dropped.
func normalizeOwnPlatforms(field string, platforms map[string]OwnPlatform) (map[string]OwnPlatform, []Problem) {
	var out []Problem
	clean := map[string]OwnPlatform{}
	for _, platform := range slices.Sorted(maps.Keys(platforms)) {
		at := field + "." + platform
		if !slices.Contains(Platforms, platform) {
			out = append(out, Problem{Field: at, Code: "value_invalid", Value: platform})
			continue
		}
		p := platforms[platform]
		out = append(out, normalizeOwnPlatform(at, &p)...)
		clean[platform] = p
	}
	return clean, out
}

// normalizeOwnPlatform cleans p, at field: a note that is not one line of maxNote letters is
// left out, so is a translation of it that is not, and so is a download that cannot be shown.
func normalizeOwnPlatform(field string, p *OwnPlatform) []Problem {
	out := cleanLine(field+".note", &p.Note, maxNote)
	var texts []Problem
	p.I18n, texts = normalizeI18n(field+".i18n", p.I18n, cleanNoteText)
	out = append(out, texts...)
	if p.Doc < 0 {
		out = append(out, Problem{Field: field + ".doc", Code: "value_invalid", Value: p.Doc})
		p.Doc = 0
	}
	downloads, ps := normalizeDownloads(field+".downloads", p.Downloads)
	// No buttons is no list: the document keeps one form of it.
	p.Downloads = nil
	if len(downloads) > 0 {
		p.Downloads = downloads
	}
	return append(out, ps...)
}

// ownOn are the admin's own apps on platform, in the document's order.
func (c AppsConfig) ownOn(platform string, icons map[string]PageAsset) []CatalogApp {
	var out []CatalogApp
	for _, a := range c.Own {
		p, ok := a.Platforms[platform]
		if !ok {
			continue
		}
		out = append(out, CatalogApp{
			ID: a.ID, Name: a.Name, Family: a.Family, Link: a.Link, Note: p.Note, I18n: maps.Clone(p.I18n), Own: true,
			Downloads: cloneDownloads(p.Downloads), Doc: p.Doc, Icon: catalogIcon(a.ID, a.Icon, icons),
		})
	}
	return out
}
