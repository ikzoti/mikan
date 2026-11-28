// Package subpage is the subscription page as the admin builds it: its look, brand, blocks
// and apps (one settings document, settings.KeySubPage), its instructions (sub_docs) and
// its images (sub_assets). Nothing configured is the page as it always was: Default.
package subpage

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"mikan/internal/textutil"
)

// Page is the page's settings document. The API takes and gives it whole.
type Page struct {
	Look   PageLook    `json:"look"`
	Brand  PageBrand   `json:"brand"`
	Blocks []PageBlock `json:"blocks" maxItems:"40" doc:"Блоки страницы в их порядке; встроенные — по одному, своих (text, links) — до 12"`
	Apps   PageApps    `json:"apps"`
	OG     PageOG      `json:"og"`
	CSS    string      `json:"css" maxLength:"20480" doc:"Свой CSS страницы подписки (не админки), до 20 КБ; @import и < убираются"`
}

// Palettes are the page's themes, the panel's: the first four follow Mode, the others are
// light or dark by themselves (web/src/lib/themes.ts, styles/themes.css). Midnight is Mikan
// in the dark mode.
var Palettes = []string{"mikan", "ocean", "sakura", "forest", "latte", "snow", "dawn", "dune", "graphite", "abyss", "ember", "plum", "moss", "terminal", "nord", "mocha", "tokyo", "dracula", "aurora", "cosmos"}

// PageLook is how the page looks.
type PageLook struct {
	Palette    string         `json:"palette" enum:"mikan,ocean,sakura,forest,latte,snow,dawn,dune,graphite,abyss,ember,plum,moss,terminal,nord,mocha,tokyo,dracula,aurora,cosmos" doc:"Тема, как темы панели. mikan, ocean, sakura и forest следуют mode; остальные светлые или тёмные сами"`
	Mode       string         `json:"mode" enum:"light,dark,system" doc:"light, dark (как Midnight) или system — как у посетителя"`
	Accent     string         `json:"accent" enum:"theme,brand" doc:"theme — цвет палитры, brand — цвет бренда (brand_accent)"`
	Background PageBackground `json:"background"`
	Font       string         `json:"font" enum:"default,onest,system,rounded" doc:"default — Unbounded и Onest, onest — только Onest, system — шрифт устройства, rounded — скруглённый системный"`
	Radius     string         `json:"radius" enum:"small,medium,large"`
	Cards      string         `json:"cards" enum:"glass,solid" doc:"glass — полупрозрачные карточки, solid — непрозрачные"`
}

// PageBackground is what is behind the cards.
type PageBackground struct {
	Kind  string `json:"kind" enum:"theme,solid,gradient,image" doc:"theme — фон темы, solid — цвет from, gradient — от from к to, image — загруженная картинка"`
	From  string `json:"from" maxLength:"7" doc:"#RRGGBB"`
	To    string `json:"to" maxLength:"7" doc:"#RRGGBB"`
	Angle int    `json:"angle" minimum:"0" maximum:"359" doc:"Направление градиента в градусах"`
	Dim   int    `json:"dim" minimum:"0" maximum:"80" doc:"Затемнение картинки, %"`
}

// PageBrand is the page's head: the logo and a line under the name (the name is the brand).
type PageBrand struct {
	Logo     string               `json:"logo" enum:"letter,image,emoji,none" doc:"letter — первая буква бренда, image — загруженный логотип, emoji, none — без значка"`
	Emoji    string               `json:"emoji" maxLength:"32"`
	Subtitle string               `json:"subtitle" maxLength:"80" doc:"Строка под названием; пусто — нет"`
	I18n     map[string]BrandText `json:"i18n,omitempty" doc:"Строка под названием на языках страницы (ru, en): посетитель видит свой язык, а где перевода нет, subtitle"`
}

// PageBlock is a section of the page. The built-in ones appear once each and their id is
// their type; the admin's own (text, links) have ids of their own: text-<x>, links-<x>.
type PageBlock struct {
	ID    string     `json:"id" maxLength:"24"`
	Type  string     `json:"type" enum:"announce,status,promo,shop,traffic,devices,apps,guide,instructions,link,reissue,locations,telegram,support,text,links"`
	On    bool       `json:"on"`
	Title string     `json:"title" maxLength:"60" doc:"Свой заголовок (у кнопок — текст кнопки); пусто — обычный"`
	Text  string     `json:"text,omitempty" maxLength:"8000" doc:"Блок text: Markdown"`
	Links []PageLink `json:"links,omitempty" maxItems:"8" doc:"Блок links: кнопки-ссылки"`
	// I18n are the title and the text in the page's languages; the links have their own.
	I18n map[string]BlockText `json:"i18n,omitempty" doc:"Заголовок и текст блока text на языках страницы (ru, en): посетитель видит свой язык, а где перевода нет, title и text"`
}

// PageLink is a button of a links block.
type PageLink struct {
	Label string               `json:"label" maxLength:"40"`
	URL   string               `json:"url" maxLength:"500" doc:"https://, http:// или tg://"`
	Emoji string               `json:"emoji,omitempty" maxLength:"32"`
	I18n  map[string]LabelText `json:"i18n,omitempty" doc:"Текст кнопки на языках страницы (ru, en); где перевода нет, label"`
}

// PageApps is the "Connect a device" block: which apps of the catalogue (AppCatalog, every
// page's) show for each platform and in what order.
type PageApps struct {
	Platform string      `json:"platform" enum:"auto,ios,android,windows,macos,linux" doc:"Вкладка, открытая сначала; auto — по устройству посетителя"`
	IOS      PageAppList `json:"ios"`
	Android  PageAppList `json:"android"`
	Windows  PageAppList `json:"windows"`
	MacOS    PageAppList `json:"macos"`
	Linux    PageAppList `json:"linux"`
	QR       bool        `json:"qr" doc:"Кнопка QR-кода в блоке ссылки"`
}

// PageAppList orders a platform's apps by their ids: those in Order first, in that order, the
// rest of the catalogue after them; Hidden ones are left out. An app added to the catalogue
// later shows up without the admin doing anything.
type PageAppList struct {
	Order  []string `json:"order" maxItems:"32" doc:"id приложений каталога (happ, koala…), которые идут первыми, в этом порядке; остальные идут за ними"`
	Hidden []string `json:"hidden" maxItems:"32" doc:"id скрытых приложений"`
}

// MaxAppList is how many ids a list of PageAppList holds: every app a platform may have, its
// built-in ones and MaxOwnApps of the admin's, with room for ids of apps gone since. The tags of
// PageAppList say the same (TestAppListLimit).
const MaxAppList = 32

// PageOG is the link preview messengers build from the page.
type PageOG struct {
	Title       string `json:"title" maxLength:"80" doc:"Заголовок превью; пусто — бренд"`
	Description string `json:"description" maxLength:"200"`
	Image       bool   `json:"image" doc:"Логотип картинкой превью"`
}

// builtinBlock is a block of the page that is there once, its id its type. after names the block
// it stands behind when a saved page lacks it (a block a newer version brings shows up next to
// what it belongs to); without one it goes to the end.
type builtinBlock struct {
	typ   string
	on    bool
	after string
}

// PageBlock types. The built-in ones in the order the page always had, with whether they
// show: the announcement and the server list are new and start hidden.
var builtins = []builtinBlock{
	{"announce", false, ""}, {"status", true, ""}, {"promo", true, ""}, {"shop", true, ""}, {"traffic", true, ""},
	{"devices", true, ""}, {"apps", true, ""}, {"guide", true, ""}, {"instructions", true, ""}, {"link", true, ""},
	{"reissue", true, "link"}, {"locations", false, ""}, {"telegram", true, ""}, {"support", true, ""},
}

// Custom block types: the admin's own, any number up to MaxCustom.
const (
	TypeText  = "text"
	TypeLinks = "links"
	// MaxCustom is how many own blocks a page may have.
	MaxCustom = 12
)

// Platforms of the apps block and of the instructions.
var Platforms = []string{"ios", "android", "windows", "macos", "linux"}

// Default is the page as it was before it could be configured.
func Default() Page {
	c := Page{
		Look: PageLook{Palette: "mikan", Mode: "light", Accent: "theme", Font: "default", Radius: "medium", Cards: "glass",
			Background: PageBackground{Kind: "theme", Angle: 160, Dim: 30}},
		Brand: PageBrand{Logo: "letter"},
		Apps:  PageApps{Platform: "auto", QR: true},
	}
	for _, b := range builtins {
		c.Blocks = append(c.Blocks, PageBlock{ID: b.typ, Type: b.typ, On: b.on})
	}
	for _, l := range c.Apps.lists() {
		l.Order, l.Hidden = []string{}, []string{}
	}
	return c
}

// lists are the platforms' lists in the order of Platforms.
func (a *PageApps) lists() []*PageAppList {
	return []*PageAppList{&a.IOS, &a.Android, &a.Windows, &a.MacOS, &a.Linux}
}

// Problem is what is wrong with a field of a Page: Field is its path in the document
// (look.background.from, blocks[2].links[0].url), Code the error code the web words.
type Problem struct {
	Field string
	Code  string
	Value any
}

var (
	colorRe  = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	customRe = regexp.MustCompile(`^(text|links)-[a-z0-9]{1,12}$`)
	// The images of Markdown: ![alt](url "title").
	imageRe = regexp.MustCompile(`!\[[^\]]*\]\(\s*([^)\s]*)`)
)

// ValidColor says whether s is #RRGGBB.
func ValidColor(s string) bool { return colorRe.MatchString(s) }

func oneOf(v string, set ...string) bool { return slices.Contains(set, v) }

// Normalize makes c a document the page can show: unset values take their defaults, text is
// trimmed, built-in blocks missing from the list are added at its end, or behind the block they belong
// to (a block a newer version brings shows up where nothing was decided about it), what cannot be shown is
// dropped. What it had to drop or could not take is returned: the API refuses the save
// then; a stored document is shown as cleaned.
func Normalize(c *Page) []Problem {
	var out []Problem
	bad := func(field, code string, value any) {
		out = append(out, Problem{Field: field, Code: code, Value: value})
	}
	d := Default()

	l := &c.Look
	enum := func(field string, v *string, def string, set ...string) {
		*v = strings.TrimSpace(*v)
		switch {
		case *v == "":
			*v = def
		case !oneOf(*v, set...):
			bad(field, "value_invalid", *v)
			*v = def
		}
	}
	enum("look.palette", &l.Palette, d.Look.Palette, Palettes...)
	enum("look.mode", &l.Mode, d.Look.Mode, "light", "dark", "system")
	enum("look.accent", &l.Accent, d.Look.Accent, "theme", "brand")
	enum("look.font", &l.Font, d.Look.Font, "default", "onest", "system", "rounded")
	enum("look.radius", &l.Radius, d.Look.Radius, "small", "medium", "large")
	enum("look.cards", &l.Cards, d.Look.Cards, "glass", "solid")
	bg := &l.Background
	enum("look.background.kind", &bg.Kind, d.Look.Background.Kind, "theme", "solid", "gradient", "image")
	for field, v := range map[string]*string{"look.background.from": &bg.From, "look.background.to": &bg.To} {
		*v = strings.ToUpper(strings.TrimSpace(*v))
		if *v != "" && !ValidColor(*v) {
			bad(field, "color_invalid", *v)
			*v = ""
		}
	}
	switch {
	case bg.Kind == "solid" && bg.From == "":
		bad("look.background.from", "color_required", nil)
		bg.Kind = "theme"
	case bg.Kind == "gradient" && (bg.From == "" || bg.To == ""):
		field := "look.background.from"
		if bg.From != "" {
			field = "look.background.to"
		}
		bad(field, "color_required", nil)
		bg.Kind = "theme"
	}
	if bg.Angle < 0 || bg.Angle > 359 {
		bad("look.background.angle", "value_invalid", bg.Angle)
		bg.Angle = d.Look.Background.Angle
	}
	if bg.Dim < 0 || bg.Dim > 80 {
		bad("look.background.dim", "value_invalid", bg.Dim)
		bg.Dim = d.Look.Background.Dim
	}

	b := &c.Brand
	enum("brand.logo", &b.Logo, d.Brand.Logo, "letter", "image", "emoji", "none")
	b.Emoji = strings.TrimSpace(b.Emoji)
	if b.Emoji != "" && !shortMark(b.Emoji) {
		bad("brand.emoji", "emoji_invalid", nil)
		b.Emoji = ""
	}
	if b.Logo == "emoji" && b.Emoji == "" {
		bad("brand.emoji", "emoji_required", nil)
		b.Logo = d.Brand.Logo
	}
	b.Subtitle = strings.TrimSpace(b.Subtitle)
	if !oneLine(b.Subtitle, maxSubtitle) {
		bad("brand.subtitle", "one_line", nil)
		b.Subtitle = ""
	}
	var texts []Problem
	b.I18n, texts = normalizeI18n("brand.i18n", b.I18n, cleanBrandText)
	out = append(out, texts...)

	out = append(out, normalizeBlocks(c)...)

	a := &c.Apps
	enum("apps.platform", &a.Platform, d.Apps.Platform, append([]string{"auto"}, Platforms...)...)
	for i, l := range a.lists() {
		for _, part := range []struct {
			name string
			list *[]string
		}{{"order", &l.Order}, {"hidden", &l.Hidden}} {
			field := "apps." + Platforms[i] + "." + part.name
			clean := []string{}
			for _, n := range *part.list {
				n = strings.TrimSpace(n)
				id, ok := appID(n)
				if !ok {
					bad(field, "app_invalid", n)
					continue
				}
				if !slices.Contains(clean, id) {
					clean = append(clean, id)
				}
			}
			if len(clean) > MaxAppList {
				bad(field, "too_many", MaxAppList)
				clean = clean[:MaxAppList]
			}
			*part.list = clean
		}
	}

	o := &c.OG
	o.Title, o.Description = strings.TrimSpace(o.Title), strings.TrimSpace(o.Description)
	if !oneLine(o.Title, 80) {
		bad("og.title", "one_line", nil)
		o.Title = ""
	}
	if !oneLine(o.Description, 200) {
		bad("og.description", "one_line", nil)
		o.Description = ""
	}

	if utf8.RuneCountInString(c.CSS) > 20480 {
		bad("css", "too_long", 20480)
		c.CSS = ""
	}
	c.CSS = CleanCSS(c.CSS)
	return out
}

func normalizeBlocks(c *Page) []Problem {
	var out []Problem
	bad := func(i int, field, code string, value any) {
		p := "blocks[" + strconv.Itoa(i) + "]"
		if field != "" {
			p += "." + field
		}
		out = append(out, Problem{Field: p, Code: code, Value: value})
	}
	seen := map[string]bool{}
	custom := 0
	blocks := make([]PageBlock, 0, len(c.Blocks)+len(builtins))
	for i, b := range c.Blocks {
		b.ID, b.Type = strings.TrimSpace(b.ID), strings.TrimSpace(b.Type)
		builtin := slices.ContainsFunc(builtins, func(x builtinBlock) bool { return x.typ == b.Type })
		switch {
		case builtin:
			if b.ID != b.Type {
				bad(i, "id", "block_id_invalid", b.ID)
				continue
			}
		case b.Type == TypeText || b.Type == TypeLinks:
			if !customRe.MatchString(b.ID) || !strings.HasPrefix(b.ID, b.Type+"-") {
				bad(i, "id", "block_id_invalid", b.ID)
				continue
			}
			if custom++; custom > MaxCustom {
				bad(i, "", "too_many_blocks", MaxCustom)
				continue
			}
		default:
			bad(i, "type", "block_unknown", b.Type)
			continue
		}
		if seen[b.ID] {
			bad(i, "id", "block_duplicate", b.ID)
			continue
		}
		seen[b.ID] = true
		at := "blocks[" + strconv.Itoa(i) + "]."
		b.Title = strings.TrimSpace(b.Title)
		if !oneLine(b.Title, maxBlockTitle) {
			bad(i, "title", "one_line", nil)
			b.Title = ""
		}
		if b.Type != TypeText {
			b.Text = ""
		}
		if b.Type != TypeLinks {
			b.Links = nil
		}
		if b.Type == TypeText {
			out = append(out, cleanMarkdown(at+"text", &b.Text, maxBlockText)...)
		}
		var texts []Problem
		b.I18n, texts = normalizeI18n(at+"i18n", b.I18n, cleanBlockText(b.Type))
		out = append(out, texts...)
		if b.Type == TypeLinks {
			if len(b.Links) > 8 {
				bad(i, "links", "too_many", 8)
				b.Links = b.Links[:8]
			}
			b.Links, texts = normalizeLinks(at, b.Links)
			out = append(out, texts...)
		}
		blocks = append(blocks, b)
	}
	for _, x := range builtins {
		if seen[x.typ] {
			continue
		}
		missing := PageBlock{ID: x.typ, Type: x.typ, On: x.on}
		if at := slices.IndexFunc(blocks, func(b PageBlock) bool { return x.after != "" && b.ID == x.after }); at >= 0 {
			blocks = slices.Insert(blocks, at+1, missing)
		} else {
			blocks = append(blocks, missing)
		}
	}
	c.Blocks = blocks
	return out
}

// normalizeLinks cleans list, the buttons of a links block whose fields start with at
// (blocks[i].): a button the page cannot show goes, and a translation of its text that breaks
// the text's rules is left out.
func normalizeLinks(at string, list []PageLink) ([]PageLink, []Problem) {
	var out []Problem
	links := []PageLink{}
	for j, l := range list {
		field := at + "links[" + strconv.Itoa(j) + "]."
		bad := func(name, code string, value any) {
			out = append(out, Problem{Field: field + name, Code: code, Value: value})
		}
		l.Label, l.URL, l.Emoji = strings.TrimSpace(l.Label), strings.TrimSpace(l.URL), strings.TrimSpace(l.Emoji)
		ok := true
		if l.Label == "" || !oneLine(l.Label, maxLabel) {
			bad("label", "label_required", nil)
			ok = false
		}
		if !textutil.ValidLink(l.URL, pageLinks) {
			bad("url", "link_invalid", nil)
			ok = false
		}
		if l.Emoji != "" && !shortMark(l.Emoji) {
			bad("emoji", "emoji_invalid", nil)
			ok = false
		}
		var texts []Problem
		l.I18n, texts = normalizeI18n(field+"i18n", l.I18n, cleanLabelText)
		out = append(out, texts...)
		if ok {
			links = append(links, l)
		}
	}
	return links, out
}

// pageLinks are the links a button of the page may open: http(s) or a Telegram one.
var pageLinks = textutil.LinkRules{HTTP: true, TG: true, Opaque: true, MaxLen: 500}

// FirstBadImage is the first image of a Markdown text that is not an https link, "" when
// they all are: the page shows only those (its own origin and https).
func FirstBadImage(md string) string {
	for _, m := range imageRe.FindAllStringSubmatch(md, -1) {
		u, err := url.Parse(m[1])
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			if m[1] == "" {
				return "()"
			}
			return m[1]
		}
	}
	return ""
}

// oneLine: at most n letters, no line breaks or other control characters (lineProblem).
func oneLine(s string, n int) bool {
	code, _ := lineProblem(s, n)
	return code == ""
}

// shortMark is an emoji, or a few letters in its place: up to 16 code points (the longest
// emoji sequences, families and flags of regions, take 10 or so), no spaces or markup.
func shortMark(s string) bool {
	return utf8.RuneCountInString(s) <= 16 && !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r) || r == '<' || r == '>' || r == '&' || r == '"' || r == '\''
	})
}

// appIDRe is an id of the app catalogue: lowercase Latin letters, digits and hyphens.
var appIDRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// oldAppNames are the names the page's lists held before they held ids (#278), the built-in
// apps' names on the page then: a page saved before keeps its order.
var oldAppNames = map[string]string{
	"Mikan": "mikan", "Kabosu": "kabosu", "Koala Clash": "koala", "ClashFest": "clashfest", "SlothClash": "slothclash",
	"Clash Verge Rev": "verge", "Happ": "happ", "INCY": "incy", "v2RayTun": "v2raytun", "Streisand": "streisand", "Hiddify": "hiddify",
}

// appID is the id s stands for in a list of the page's apps: an id as it is, an old name as
// its app's id; false for anything else. An id is kept whether or not the catalogue has it
// now, as a name was.
func appID(s string) (string, bool) {
	if id, ok := oldAppNames[s]; ok {
		return id, true
	}
	return s, appIDRe.MatchString(s)
}

var (
	cssImport = regexp.MustCompile(`(?i)@import`)
	// A url() or image-set() that leaves the panel: every subscriber's app would fetch it,
	// telling another site their address.
	cssRemote = regexp.MustCompile(`(?i)(url|image-set|image)\(\s*['"]?\s*(https?:|//)[^)]*\)`)
)

// CleanCSS keeps the admin's CSS inside its <style> and the subscribers to the panel: no
// "<" at all (so no "</style>" either; CSS needs it nowhere but in strings), no @import,
// no escapes (they spell @import or url( so that the checks miss them) and no url() of
// another site, which would let it count the visitors. Local and data: images stay.
func CleanCSS(css string) string {
	css = strings.ReplaceAll(css, "<", "")
	css = strings.ReplaceAll(css, "\x00", "")
	css = strings.ReplaceAll(css, `\`, "")
	for {
		next := cssImport.ReplaceAllString(css, "")
		next = cssRemote.ReplaceAllString(next, "none")
		if next == css {
			break
		}
		css = next
	}
	return strings.TrimSpace(css)
}

// HasControl says whether s holds a control character other than a line break or a tab:
// PostgreSQL refuses NUL in text, and the rest only breaks the page.
func HasControl(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' })
}

// Doc is an instruction as the admin writes it: a page of Markdown with a title, an emoji
// and, if it is about one, a platform.
type Doc struct {
	Title    string
	Emoji    string
	Body     string
	Platform string
	// I18n are its title and text in the page's languages.
	I18n map[string]DocText
}

// MaxDocs is how many instructions the page may have.
const MaxDocs = 50

// NormalizeDoc trims d and says what is wrong with it.
func NormalizeDoc(d *Doc) []Problem {
	var out []Problem
	bad := func(field, code string, value any) {
		out = append(out, Problem{Field: field, Code: code, Value: value})
	}
	d.Title, d.Emoji, d.Platform = strings.TrimSpace(d.Title), strings.TrimSpace(d.Emoji), strings.TrimSpace(d.Platform)
	if d.Title == "" || !oneLine(d.Title, maxDocTitle) {
		bad("title", "title_required", nil)
	}
	if d.Emoji != "" && !shortMark(d.Emoji) {
		bad("emoji", "emoji_invalid", nil)
	}
	if d.Platform != "" && !slices.Contains(Platforms, d.Platform) {
		bad("platform", "value_invalid", d.Platform)
	}
	out = append(out, cleanMarkdown("body", &d.Body, maxDocBody)...)
	var texts []Problem
	d.I18n, texts = normalizeI18n("i18n", d.I18n, cleanDocText)
	return append(out, texts...)
}
