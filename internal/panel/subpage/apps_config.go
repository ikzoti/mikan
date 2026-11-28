package subpage

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"mikan/internal/textutil"
)

// AppsConfig is what the admin made of the page's apps (settings.KeySubApps), one document
// every page shares, as the instructions and images are: a page keeps only its own order and
// hidden lists (PageApps). Nothing set is the apps as they come.
type AppsConfig struct {
	Presets map[string]PresetEdit `json:"presets" doc:"Правки встроенных приложений по их id (happ, koala…); приложения без правок здесь нет"`
	Own     []OwnApp              `json:"own" maxItems:"20" doc:"Свои приложения админа, до 20, в этом порядке: на каждой платформе приложение на mihomo идёт за встроенными на mihomo, остальные после всех встроенных"`
}

// PresetEdit is what the admin changed of a built-in app: its icon, and by platform where it
// is downloaded.
type PresetEdit struct {
	Icon      *AppIcon                `json:"icon,omitempty" doc:"Свой значок; без поля первая буква названия"`
	Platforms map[string]PlatformEdit `json:"platforms,omitempty" doc:"По платформам приложения: ios, android, windows, macos, linux"`
}

// PlatformEdit replaces the app's downloads and instruction on one platform. The downloads
// are a pointer so that an empty list (no buttons) differs from none given (the app's own),
// in the API and in the stored document alike.
type PlatformEdit struct {
	Downloads *[]AppDownload `json:"downloads,omitempty" maxItems:"5" doc:"Свои кнопки «Скачать» вместо встроенных: пустой список убирает кнопки, без поля остаются встроенные"`
	Doc       int64          `json:"doc,omitempty" minimum:"0" doc:"Инструкция по шагам: id из GET /api/v1/sub-docs; 0 или нет поля: без инструкции"`
}

// MaxDownloads is how many download buttons an app has on a platform.
const MaxDownloads = 5

// maxLabel is how long the text of a button may be: a download's, a link's of a links block.
const maxLabel = 40

// downloadLinks are the links a download button may open: https alone.
var downloadLinks = textutil.LinkRules{MaxLen: 500}

// NormalizeApps makes c a document the catalogue is made of: text is trimmed, what cannot be
// shown is dropped, and so is an edit that changes nothing, so the document holds only what
// the admin changed. What it had to drop is returned, as Normalize does: the API refuses the
// save then; a stored document is shown as cleaned.
func NormalizeApps(c *AppsConfig) []Problem {
	var out []Problem
	edits := map[string]PresetEdit{}
	for _, id := range slices.Sorted(maps.Keys(c.Presets)) {
		field := "presets." + id
		on, ok := shelfOf[id]
		if !ok {
			out = append(out, Problem{Field: field, Code: "app_unknown", Value: id})
			continue
		}
		e, ps := normalizePresetEdit(field, c.Presets[id], on)
		out = append(out, ps...)
		if e.Icon != nil || len(e.Platforms) > 0 {
			edits[id] = e
		}
	}
	c.Presets = edits
	own, ps := normalizeOwnApps(c.Own)
	c.Own = own
	return append(out, ps...)
}

// normalizePresetEdit cleans e, the edit at field of an app that is on the platforms of on.
func normalizePresetEdit(field string, e PresetEdit, on map[string]presetPlace) (PresetEdit, []Problem) {
	icon, out := normalizeIcon(field+".icon", e.Icon)
	var platforms map[string]PlatformEdit
	for _, platform := range slices.Sorted(maps.Keys(e.Platforms)) {
		at := field + ".platforms." + platform
		if _, ok := on[platform]; !ok {
			out = append(out, Problem{Field: at, Code: "value_invalid", Value: platform})
			continue
		}
		p := e.Platforms[platform]
		out = append(out, normalizePlatformEdit(at, &p)...)
		if p.Downloads == nil && p.Doc == 0 {
			continue
		}
		if platforms == nil {
			platforms = map[string]PlatformEdit{}
		}
		platforms[platform] = p
	}
	return PresetEdit{Icon: icon, Platforms: platforms}, out
}

// normalizePlatformEdit cleans e, the edit at field.
func normalizePlatformEdit(field string, e *PlatformEdit) []Problem {
	var out []Problem
	if e.Doc < 0 {
		out = append(out, Problem{Field: field + ".doc", Code: "value_invalid", Value: e.Doc})
		e.Doc = 0
	}
	if e.Downloads == nil {
		return out
	}
	clean, ps := normalizeDownloads(field+".downloads", *e.Downloads)
	e.Downloads = &clean
	return append(out, ps...)
}

// normalizeDownloads cleans list, the downloads at field: one that cannot be shown is
// dropped, the rest kept in their order. The list it gives is never nil.
func normalizeDownloads(field string, list []AppDownload) ([]AppDownload, []Problem) {
	var out []Problem
	if len(list) > MaxDownloads {
		out = append(out, Problem{Field: field, Code: "too_many", Value: MaxDownloads})
		list = list[:MaxDownloads]
	}
	clean := []AppDownload{}
	for i, d := range list {
		ok, ps := normalizeDownload(field+"["+strconv.Itoa(i)+"].", &d)
		if ok {
			clean = append(clean, d)
		}
		out = append(out, ps...)
	}
	return clean, out
}

// normalizeDownload trims d, the download at field, says what is wrong with it and whether the
// page can show it. Its text is one line of maxLabel letters at most, and a link has one: the
// page words the other kinds itself. A translation of the text that breaks those rules is left
// out, and the button stays.
func normalizeDownload(field string, d *AppDownload) (bool, []Problem) {
	var out []Problem
	bad := func(name, code string, value any) {
		out = append(out, Problem{Field: field + name, Code: code, Value: value})
	}
	d.Kind, d.URL, d.Label = strings.TrimSpace(d.Kind), strings.TrimSpace(d.URL), strings.TrimSpace(d.Label)
	if !oneOf(d.Kind, downloadKinds...) {
		bad("kind", "value_invalid", d.Kind)
	}
	if !textutil.ValidLink(d.URL, downloadLinks) {
		bad("url", "url_invalid", nil)
	}
	if code, value := lineProblem(d.Label, maxLabel); code != "" {
		bad("label", code, value)
	} else if d.Label == "" && d.Kind == KindLink {
		bad("label", "label_required", nil)
	}
	ok := len(out) == 0
	var texts []Problem
	d.I18n, texts = normalizeI18n(field+"i18n", d.I18n, cleanLabelText)
	return ok, append(out, texts...)
}

// Has says whether id is an app of the catalogue: a built-in one or one of c's own.
func (c AppsConfig) Has(id string) bool {
	_, preset := presetByID[id]
	return preset || slices.ContainsFunc(c.Own, func(a OwnApp) bool { return a.ID == id })
}

// OrphanIcons are the icons among names (the page's images) of apps the catalogue of c has
// not: an own app taken out of the document takes its icon along.
func (c AppsConfig) OrphanIcons(names []string) []string {
	var out []string
	for _, name := range names {
		if id, ok := strings.CutPrefix(name, appIconPrefix); ok && !c.Has(id) {
			out = append(out, name)
		}
	}
	return out
}

// DocRef is an instruction an app links to, at Field of the document.
type DocRef struct {
	Field string
	ID    int64
}

// DropDocs takes out of c the instructions of gone (refs of DocRefs that are there no more),
// and the edits left changing nothing without them: the document reads as one PUT takes.
func (c *AppsConfig) DropDocs(gone []DocRef) {
	isGone := func(id int64) bool { return slices.ContainsFunc(gone, func(r DocRef) bool { return r.ID == id }) }
	for id, e := range c.Presets {
		for platform, p := range e.Platforms {
			if p.Doc == 0 || !isGone(p.Doc) {
				continue
			}
			if p.Downloads == nil {
				delete(e.Platforms, platform)
				continue
			}
			p.Doc = 0
			e.Platforms[platform] = p
		}
		switch {
		case e.Icon == nil && len(e.Platforms) == 0:
			delete(c.Presets, id)
		case len(e.Platforms) == 0:
			c.Presets[id] = PresetEdit{Icon: e.Icon}
		}
	}
	for _, a := range c.Own {
		for platform, p := range a.Platforms {
			if p.Doc != 0 && isGone(p.Doc) {
				p.Doc = 0
				a.Platforms[platform] = p
			}
		}
	}
}

// DocRefs are the instructions the apps of c link to, in the order of its fields.
func (c AppsConfig) DocRefs() []DocRef {
	var out []DocRef
	for _, id := range slices.Sorted(maps.Keys(c.Presets)) {
		e := c.Presets[id]
		for _, platform := range slices.Sorted(maps.Keys(e.Platforms)) {
			if doc := e.Platforms[platform].Doc; doc > 0 {
				out = append(out, DocRef{Field: "presets." + id + ".platforms." + platform + ".doc", ID: doc})
			}
		}
	}
	for i, a := range c.Own {
		for _, platform := range slices.Sorted(maps.Keys(a.Platforms)) {
			if doc := a.Platforms[platform].Doc; doc > 0 {
				out = append(out, DocRef{Field: "own[" + strconv.Itoa(i) + "].platforms." + platform + ".doc", ID: doc})
			}
		}
	}
	return out
}
