package subpage

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// The page's languages are the web's (web/src/i18n/index.ts, LOCALES).
func TestLocales(t *testing.T) {
	if strings.Join(Locales, ",") != "ru,en" {
		t.Fatalf("locales: %v", Locales)
	}
}

// textBlock and linksBlock are own blocks of the page with the translations given.
func textBlock(text string, i18n map[string]BlockText) PageBlock {
	return PageBlock{ID: "text-a", Type: TypeText, On: true, Text: text, I18n: i18n}
}

func linksBlock(i18n map[string]LabelText) PageBlock {
	return PageBlock{ID: "links-a", Type: TypeLinks, On: true, Links: []PageLink{{Label: "Канал", URL: "https://t.me/mikan", I18n: i18n}}}
}

// A translation is held to the rules of the text it translates, in a language of the page.
func TestNormalizeTranslationsRefuses(t *testing.T) {
	en := func(s string) map[string]BlockText { return map[string]BlockText{"en": {Text: s}} }
	cases := []struct {
		name  string
		edit  func(*Page)
		field string
		code  string
	}{
		{"a language the page does not speak", func(c *Page) { c.Brand.I18n = map[string]BrandText{"de": {Subtitle: "Schnell"}} }, "brand.i18n.de", "value_invalid"},
		{"a language in capitals", func(c *Page) { c.Brand.I18n = map[string]BrandText{"EN": {Subtitle: "Fast"}} }, "brand.i18n.EN", "value_invalid"},
		{"a subtitle on two lines", func(c *Page) { c.Brand.I18n = map[string]BrandText{"en": {Subtitle: "Fast\nVPN"}} }, "brand.i18n.en.subtitle", "one_line"},
		{"a subtitle too long", func(c *Page) { c.Brand.I18n = map[string]BrandText{"en": {Subtitle: strings.Repeat("a", 81)}} }, "brand.i18n.en.subtitle", "too_long"},
		{"a heading too long", func(c *Page) { c.Blocks[1].I18n = map[string]BlockText{"en": {Title: strings.Repeat("a", 61)}} }, "blocks[1].i18n.en.title", "too_long"},
		{"a heading on two lines", func(c *Page) { c.Blocks[1].I18n = map[string]BlockText{"ru": {Title: "a\rb"}} }, "blocks[1].i18n.ru.title", "one_line"},
		{"a text of a block that has none", func(c *Page) { c.Blocks[1].I18n = en("Статус") }, "blocks[1].i18n.en.text", "value_invalid"},
		{"a text of a links block", func(c *Page) {
			b := linksBlock(nil)
			b.I18n = en("text")
			c.Blocks = append(c.Blocks, b)
		}, "blocks[14].i18n.en.text", "value_invalid"},
		{"an image over http", func(c *Page) {
			c.Blocks = append(c.Blocks, textBlock("Привет", en("![x](http://example.com/a.png)")))
		}, "blocks[14].i18n.en.text", "image_https"},
		{"a text too long", func(c *Page) { c.Blocks = append(c.Blocks, textBlock("", en(strings.Repeat("я", 8001)))) }, "blocks[14].i18n.en.text", "too_long"},
		{"a control character", func(c *Page) { c.Blocks = append(c.Blocks, textBlock("", en("a\x00b"))) }, "blocks[14].i18n.en.text", "value_invalid"},
		{"a text in a language the page does not speak", func(c *Page) {
			c.Blocks = append(c.Blocks, textBlock("", map[string]BlockText{"fr": {Text: "Bonjour"}}))
		}, "blocks[14].i18n.fr", "value_invalid"},
		{"a button's text on two lines", func(c *Page) {
			c.Blocks = append(c.Blocks, linksBlock(map[string]LabelText{"en": {Label: "Our\nchannel"}}))
		}, "blocks[14].links[0].i18n.en.label", "one_line"},
		{"a button's text too long", func(c *Page) {
			c.Blocks = append(c.Blocks, linksBlock(map[string]LabelText{"en": {Label: strings.Repeat("a", 41)}}))
		}, "blocks[14].links[0].i18n.en.label", "too_long"},
		{"a button's text in a language the page does not speak", func(c *Page) {
			c.Blocks = append(c.Blocks, linksBlock(map[string]LabelText{"uk": {Label: "Канал"}}))
		}, "blocks[14].links[0].i18n.uk", "value_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			tc.edit(&c)
			got := codes(Normalize(&c))
			if got[tc.field] != tc.code {
				t.Fatalf("problems %v, want %s at %s", got, tc.code, tc.field)
			}
		})
	}
}

// A translation is trimmed as its text is, an empty one goes, and a document without any has no
// i18n at all: the page's data is what it always was.
func TestNormalizeTranslationsKeepsWhatIsRight(t *testing.T) {
	c := Default()
	c.Brand.Subtitle = "Быстрый VPN"
	c.Brand.I18n = map[string]BrandText{"en": {Subtitle: "  Fast VPN "}, "ru": {Subtitle: "  "}}
	c.Blocks[1].I18n = map[string]BlockText{"en": {Title: " Status "}}
	c.Blocks[2].I18n = map[string]BlockText{"en": {}, "ru": {Title: ""}}
	c.Blocks = append(c.Blocks,
		textBlock("# Привет", map[string]BlockText{"en": {Title: " Hello ", Text: " # Hi\r\n![](https://example.com/a.png) "}, "ru": {Text: "\n"}}),
		// A text in one language alone: the others see none.
		PageBlock{ID: "text-b", Type: TypeText, On: true, I18n: map[string]BlockText{"en": {Text: "English only"}}},
		PageBlock{ID: "links-a", Type: TypeLinks, On: true, Links: []PageLink{{Label: "Канал", URL: "https://t.me/mikan", I18n: map[string]LabelText{"en": {Label: " Channel "}, "ru": {}}}}},
	)
	if ps := Normalize(&c); len(ps) != 0 {
		t.Fatalf("problems: %+v", ps)
	}
	if !reflect.DeepEqual(c.Brand.I18n, map[string]BrandText{"en": {Subtitle: "Fast VPN"}}) {
		t.Errorf("brand: %+v", c.Brand.I18n)
	}
	if !reflect.DeepEqual(c.Blocks[1].I18n, map[string]BlockText{"en": {Title: "Status"}}) || c.Blocks[2].I18n != nil {
		t.Errorf("built-in blocks: %+v %+v", c.Blocks[1].I18n, c.Blocks[2].I18n)
	}
	if want := map[string]BlockText{"en": {Title: "Hello", Text: "# Hi\n![](https://example.com/a.png)"}}; !reflect.DeepEqual(c.Blocks[14].I18n, want) {
		t.Errorf("text block: %+v", c.Blocks[14].I18n)
	}
	if !reflect.DeepEqual(c.Blocks[15].I18n, map[string]BlockText{"en": {Text: "English only"}}) {
		t.Errorf("a text in one language: %+v", c.Blocks[15].I18n)
	}
	if want := map[string]LabelText{"en": {Label: "Channel"}}; !reflect.DeepEqual(c.Blocks[16].Links[0].I18n, want) {
		t.Errorf("button: %+v", c.Blocks[16].Links[0].I18n)
	}
	again := c
	if ps := Normalize(&again); len(ps) != 0 || !reflect.DeepEqual(again, c) {
		t.Fatalf("a clean document changed: %+v", ps)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Page
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, c) {
		t.Fatalf("does not read back: %v %s", err, raw)
	}
	def, err := json.Marshal(Default())
	if err != nil || strings.Contains(string(def), "i18n") {
		t.Fatalf("the default document: %v %s", err, def)
	}
}

// A wrong translation of a button's text is left out, and the button stays.
func TestBadTranslationKeepsTheButton(t *testing.T) {
	c := Default()
	c.Blocks = append(c.Blocks, linksBlock(map[string]LabelText{"en": {Label: "a\nb"}, "ru": {Label: "Канал"}}))
	if ps := Normalize(&c); len(ps) != 1 {
		t.Fatalf("problems: %+v", ps)
	}
	if l := c.Blocks[14].Links; len(l) != 1 || !reflect.DeepEqual(l[0].I18n, map[string]LabelText{"ru": {Label: "Канал"}}) {
		t.Fatalf("links: %+v", l)
	}
}

func TestNormalizeAppTranslations(t *testing.T) {
	store := AppDownload{Kind: KindAppStore, URL: "https://apps.example.com/ourvpn"}
	onIOS := func(p OwnPlatform) AppsConfig {
		a := ownApp("own-a1")
		a.Platforms = map[string]OwnPlatform{"ios": p}
		return withOwn(a)
	}
	labelled := func(i18n map[string]LabelText) AppDownload {
		d := store
		d.I18n = i18n
		return d
	}
	cases := []struct {
		name  string
		c     AppsConfig
		field string
		code  string
	}{
		{"a note on two lines", onIOS(OwnPlatform{Note: "Для iPhone", I18n: map[string]NoteText{"en": {Note: "For\niPhone"}}}), "own[0].platforms.ios.i18n.en.note", "one_line"},
		{"a note too long", onIOS(OwnPlatform{I18n: map[string]NoteText{"en": {Note: strings.Repeat("a", 81)}}}), "own[0].platforms.ios.i18n.en.note", "too_long"},
		{"a note in a language the page does not speak", onIOS(OwnPlatform{I18n: map[string]NoteText{"fr": {Note: "Pour iPhone"}}}), "own[0].platforms.ios.i18n.fr", "value_invalid"},
		{"an own button's text too long", onIOS(OwnPlatform{Downloads: []AppDownload{labelled(map[string]LabelText{"en": {Label: strings.Repeat("a", 41)}})}}), "own[0].platforms.ios.downloads[0].i18n.en.label", "too_long"},
		{"a built-in app's button text on two lines", edit("happ", "ios", PlatformEdit{Downloads: downloads(labelled(map[string]LabelText{"ru": {Label: "App\nStore"}}))}), "presets.happ.platforms.ios.downloads[0].i18n.ru.label", "one_line"},
		{"a button text in a language the page does not speak", edit("happ", "ios", PlatformEdit{Downloads: downloads(labelled(map[string]LabelText{"de": {Label: "Laden"}}))}), "presets.happ.platforms.ios.downloads[0].i18n.de", "value_invalid"},
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

// The catalogue gives the page an own app's note and every button's text in the page's
// languages, as copies: what the page does with them reaches no document.
func TestCatalogTranslations(t *testing.T) {
	mirror := AppDownload{Kind: KindMirror, URL: "https://files.example.com/a.ipa", Label: "Зеркало", I18n: map[string]LabelText{"en": {Label: " Mirror "}, "ru": {Label: ""}}}
	a := ownApp("own-a1")
	a.Platforms = map[string]OwnPlatform{"ios": {Note: "Для iPhone", I18n: map[string]NoteText{"en": {Note: " For iPhone "}}, Downloads: []AppDownload{mirror}}}
	c := withOwn(a)
	c.Presets = map[string]PresetEdit{"hiddify": {Platforms: map[string]PlatformEdit{"linux": {Downloads: downloads(mirror)}}}}
	if ps := NormalizeApps(&c); len(ps) != 0 {
		t.Fatalf("problems: %+v", ps)
	}
	got := c.Catalog(nil)
	own := find(t, got.IOS, "own-a1")
	if !reflect.DeepEqual(own.I18n, map[string]NoteText{"en": {Note: "For iPhone"}}) || !reflect.DeepEqual(own.Downloads[0].I18n, map[string]LabelText{"en": {Label: "Mirror"}}) {
		t.Fatalf("own app: %+v", own)
	}
	h := find(t, got.Linux, "hiddify")
	if h.I18n != nil || !reflect.DeepEqual(h.Downloads[0].I18n, map[string]LabelText{"en": {Label: "Mirror"}}) {
		t.Fatalf("a built-in app's buttons: %+v", h)
	}
	own.I18n["en"] = NoteText{Note: "changed"}
	own.Downloads[0].I18n["en"] = LabelText{Label: "changed"}
	h.Downloads[0].I18n["en"] = LabelText{Label: "changed"}
	if c.Own[0].Platforms["ios"].I18n["en"].Note != "For iPhone" || c.Own[0].Platforms["ios"].Downloads[0].I18n["en"].Label != "Mirror" || (*c.Presets["hiddify"].Platforms["linux"].Downloads)[0].I18n["en"].Label != "Mirror" {
		t.Fatal("the catalogue shares the document's translations")
	}
}

func TestNormalizeDocTranslations(t *testing.T) {
	d := Doc{Title: "Как подключить iPhone", Body: "Шаг 1", I18n: map[string]DocText{
		"en": {Title: " How to connect an iPhone ", Body: "Step 1\r\n![](https://example.com/1.png)"},
		"ru": {Title: " ", Body: "\n"},
	}}
	if ps := NormalizeDoc(&d); len(ps) != 0 {
		t.Fatalf("a good page: %+v", ps)
	}
	if want := map[string]DocText{"en": {Title: "How to connect an iPhone", Body: "Step 1\n![](https://example.com/1.png)"}}; !reflect.DeepEqual(d.I18n, want) {
		t.Fatalf("cleaned to %+v", d.I18n)
	}
	for name, c := range map[string]struct {
		i18n  map[string]DocText
		field string
		code  string
	}{
		"a title on two lines":    {map[string]DocText{"en": {Title: "How\nto"}}, "i18n.en.title", "one_line"},
		"a title too long":        {map[string]DocText{"en": {Title: strings.Repeat("a", 81)}}, "i18n.en.title", "too_long"},
		"an image over http":      {map[string]DocText{"en": {Body: "![a](http://example.com/x.png)"}}, "i18n.en.body", "image_https"},
		"a text too long":         {map[string]DocText{"en": {Body: strings.Repeat("a", 20001)}}, "i18n.en.body", "too_long"},
		"a control character":     {map[string]DocText{"ru": {Body: "a\x07b"}}, "i18n.ru.body", "value_invalid"},
		"a language not the page": {map[string]DocText{"kk": {Title: "Қалай"}}, "i18n.kk", "value_invalid"},
	} {
		d := Doc{Title: "x", I18n: c.i18n}
		if got := codes(NormalizeDoc(&d)); got[c.field] != c.code {
			t.Errorf("%s: %v, want %s at %s", name, got, c.code, c.field)
		}
	}
}

// The database keeps an instruction's translations as JSON by language, {} for none, and their
// titles apart; what it gives back is read as NormalizeDoc cleans it, and what does not read is
// none, with ErrUnreadable.
func TestDocI18nStorage(t *testing.T) {
	for _, none := range []map[string]DocText{nil, {}} {
		if raw, err := MarshalDocI18n(none); err != nil || raw != "{}" {
			t.Fatalf("none stored as %q (%v)", raw, err)
		}
		if raw, err := MarshalDocTitles(none); err != nil || raw != "{}" {
			t.Fatalf("no titles stored as %q (%v)", raw, err)
		}
	}
	m := map[string]DocText{"en": {Title: "<Guide>", Body: "Step 1"}, "ru": {Body: "Шаг 1"}}
	raw, err := MarshalDocI18n(m)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := UnmarshalDocI18n(raw); err != nil || !reflect.DeepEqual(got, m) {
		t.Fatalf("round trip: %q %+v %v", raw, got, err)
	}
	titles, err := MarshalDocTitles(m)
	if err != nil || strings.Contains(titles, "body") || strings.Contains(titles, "ru") {
		t.Fatalf("titles: %q %v", titles, err)
	}
	if got, err := UnmarshalDocI18n(titles); err != nil || !reflect.DeepEqual(got, map[string]DocText{"en": {Title: "<Guide>"}}) {
		t.Fatalf("titles read back: %+v %v", got, err)
	}
	for raw, want := range map[string]map[string]DocText{
		"{}": nil,
		// A translation with no title is none in the page's list.
		`{"en": {"title": null}, "ru": {"title": "Как"}}`:        {"ru": {Title: "Как"}},
		`{"en": {"title": " Guide "}, "de": {"title": "Hilfe"}}`: {"en": {Title: "Guide"}},
	} {
		if got, err := UnmarshalDocI18n(raw); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %+v (%v), want %+v", raw, got, err, want)
		}
	}
	for _, raw := range []string{"not json", `{"en": "x"}`, ""} {
		if got, err := UnmarshalDocI18n(raw); got != nil || !errors.Is(err, ErrUnreadable) {
			t.Errorf("%q: %+v (%v), want none and ErrUnreadable", raw, got, err)
		}
	}
}

// A text block whose text is in another language alone still shows https images there.
func TestRemoteImagesOfTranslations(t *testing.T) {
	c := Default()
	c.Blocks = append(c.Blocks, linksBlock(map[string]LabelText{"en": {Label: "Channel"}}))
	p := Public{Config: c, Docs: []DocItem{}}
	if p.RemoteImages() {
		t.Fatal("buttons hold no images")
	}
	p.Config.Blocks = append(p.Config.Blocks, PageBlock{ID: "text-a", Type: TypeText, On: true, I18n: map[string]BlockText{"en": {Text: "![](https://example.com/a.png)"}}})
	if !p.RemoteImages() {
		t.Fatal("a translated text may show https images")
	}
}
