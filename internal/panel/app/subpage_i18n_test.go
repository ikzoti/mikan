package app

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"mikan/internal/panel/subpage"
)

// The admin's texts in the page's languages over HTTP (#278): the page's document, the apps
// and the instructions take translations by language, held to the rules of the texts they
// translate and saved by the session alone; the page gets them, and of the instructions in its
// list their titles alone.
func TestSubPageTranslationsOverHTTP(t *testing.T) {
	k := newKeyHarness(t)
	sub := subscriberOf(t, k)
	page := func() subpage.Public {
		t.Helper()
		resp, body := k.do(http.MethodGet, sub, nil, browser)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("page: %d", resp.StatusCode)
		}
		return pageData(t, body)
	}
	refused := func(name string, resp *http.Response, raw []byte, location, code string) {
		t.Helper()
		var r refusal
		if resp.StatusCode != http.StatusUnprocessableEntity || json.Unmarshal(raw, &r) != nil || r.code(location) != code {
			t.Errorf("%s: %d %s, want %s at %s", name, resp.StatusCode, raw, code, location)
		}
	}

	// The page's document.
	resp, raw := k.do(http.MethodGet, k.api+"/sub-page", nil, nil)
	var view struct {
		Config subpage.Page `json:"config"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &view) != nil {
		t.Fatalf("page: %d %s", resp.StatusCode, raw)
	}
	with := func(brand map[string]subpage.BrandText, extra ...subpage.PageBlock) subpage.Page {
		c := view.Config
		c.Brand.Subtitle, c.Brand.I18n = "Быстрый VPN", brand
		c.Blocks = append(slices.Clone(view.Config.Blocks), extra...)
		return c
	}
	hello := subpage.PageBlock{ID: "text-a", Type: subpage.TypeText, On: true, Title: "Привет", Text: "Текст", I18n: map[string]subpage.BlockText{"en": {Title: "Hello", Text: "Text"}}}
	fast := map[string]subpage.BrandText{"en": {Subtitle: "Fast VPN"}}
	status := slices.IndexFunc(view.Config.Blocks, func(b subpage.PageBlock) bool { return b.Type == "status" })
	statusText := with(fast)
	statusText.Blocks[status].I18n = map[string]subpage.BlockText{"en": {Text: "All good"}}
	httpImage := hello
	httpImage.I18n = map[string]subpage.BlockText{"en": {Text: "![](http://example.com/a.png)"}}
	for name, c := range map[string]struct {
		config         subpage.Page
		location, code string
	}{
		"a language the page does not speak": {with(map[string]subpage.BrandText{"de": {Subtitle: "Schnell"}}), "body.config.brand.i18n.de", "value_invalid"},
		"a text of a block that has none":    {statusText, "body.config.blocks[" + strconv.Itoa(status) + "].i18n.en.text", "value_invalid"},
		"an image over http":                 {with(fast, httpImage), "body.config.blocks[" + strconv.Itoa(len(view.Config.Blocks)) + "].i18n.en.text", "image_https"},
	} {
		resp, raw := k.do(http.MethodPut, k.api+"/sub-page", map[string]any{"config": c.config}, k.csrf)
		refused(name, resp, raw, c.location, c.code)
	}
	good := with(fast, hello)
	// A full key may not put words in front of every subscriber, in any language.
	if resp, raw := k.asKey(k.full, http.MethodPut, "/sub-page", map[string]any{"config": good}); resp.StatusCode != http.StatusForbidden || !strings.Contains(string(raw), "session_only") {
		t.Fatalf("full key: %d %s", resp.StatusCode, raw)
	}
	if resp, raw := k.do(http.MethodPut, k.api+"/sub-page", map[string]any{"config": good}, k.csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("save: %d %s", resp.StatusCode, raw)
	}
	p := page()
	if !reflect.DeepEqual(p.Config.Brand.I18n, fast) {
		t.Fatalf("the page's brand: %+v", p.Config.Brand)
	}
	if i := slices.IndexFunc(p.Config.Blocks, func(b subpage.PageBlock) bool { return b.ID == "text-a" }); i < 0 || !reflect.DeepEqual(p.Config.Blocks[i].I18n, hello.I18n) {
		t.Fatalf("the page's text block: %+v", p.Config.Blocks)
	}

	// The apps: an own app's note and a button's text.
	label := map[string]subpage.LabelText{"en": {Label: "Mirror"}}
	own := map[string]any{"id": "own-ours1", "name": "Наш VPN", "family": "xray", "link": "ourvpn://add/{url_enc}", "platforms": map[string]any{"ios": map[string]any{
		"note": "Для iPhone", "i18n": map[string]any{"en": map[string]any{"note": "For iPhone"}},
		"downloads": []any{map[string]any{"kind": "mirror", "url": "https://files.example.com/a.ipa", "label": "Зеркало", "i18n": label}},
	}}}
	tooLong := map[string]any{"own": []any{}, "presets": map[string]any{"happ": map[string]any{"platforms": map[string]any{"ios": map[string]any{
		"downloads": []any{map[string]any{"kind": "mirror", "url": "https://files.example.com/a.ipa", "i18n": map[string]any{"en": map[string]any{"label": "Mirror\nfor iPhone"}}}},
	}}}}}
	resp, raw = k.do(http.MethodPut, k.api+"/sub-page/apps", tooLong, k.csrf)
	refused("a button's text on two lines", resp, raw, "body.presets.happ.platforms.ios.downloads[0].i18n.en.label", "one_line")
	if resp, raw := k.do(http.MethodPut, k.api+"/sub-page/apps", appsDoc(own), k.csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("apps: %d %s", resp.StatusCode, raw)
	}
	if a := appOn(page().Catalog.IOS, "own-ours1"); !reflect.DeepEqual(a.I18n, map[string]subpage.NoteText{"en": {Note: "For iPhone"}}) || len(a.Downloads) != 1 || !reflect.DeepEqual(a.Downloads[0].I18n, label) {
		t.Fatalf("the page's own app: %+v", a)
	}

	// The instructions.
	how := map[string]subpage.DocText{"en": {Title: "How to connect", Body: "Step 1"}}
	doc := func(i18n any) map[string]any {
		d := map[string]any{"title": "Как подключить", "emoji": "", "body": "Шаг 1", "platform": "", "published": true}
		if i18n != nil {
			d["i18n"] = i18n
		}
		return d
	}
	if resp, raw := k.asKey(k.full, http.MethodPost, "/sub-docs", doc(how)); resp.StatusCode != http.StatusForbidden || !strings.Contains(string(raw), "session_only") {
		t.Fatalf("full key: %d %s", resp.StatusCode, raw)
	}
	resp, raw = k.do(http.MethodPost, k.api+"/sub-docs", doc(map[string]any{"en": map[string]any{"body": "![](http://example.com/a.png)"}}), k.csrf)
	refused("an image over http", resp, raw, "body.i18n.en.body", "image_https")
	resp, raw = k.do(http.MethodPost, k.api+"/sub-docs", doc(map[string]any{"fr": map[string]any{"title": "Comment"}}), k.csrf)
	refused("a language the page does not speak", resp, raw, "body.i18n.fr", "value_invalid")
	var made struct {
		ID   int64                      `json:"id"`
		I18n map[string]subpage.DocText `json:"i18n"`
	}
	resp, raw = k.do(http.MethodPost, k.api+"/sub-docs", doc(map[string]any{"en": map[string]any{"title": " How to connect ", "body": "Step 1"}, "ru": map[string]any{"title": ""}}), k.csrf)
	if resp.StatusCode != http.StatusCreated || json.Unmarshal(raw, &made) != nil || !reflect.DeepEqual(made.I18n, how) {
		t.Fatalf("doc: %d %s", resp.StatusCode, raw)
	}
	id := strconv.FormatInt(made.ID, 10)
	listed := func() map[string]subpage.DocText {
		t.Helper()
		resp, raw := k.do(http.MethodGet, k.api+"/sub-docs", nil, nil)
		var list struct {
			Items []struct {
				ID   int64                      `json:"id"`
				I18n map[string]subpage.DocText `json:"i18n"`
			} `json:"items"`
		}
		if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &list) != nil || len(list.Items) != 1 || list.Items[0].ID != made.ID {
			t.Fatalf("docs: %d %s", resp.StatusCode, raw)
		}
		return list.Items[0].I18n
	}
	if got := listed(); !reflect.DeepEqual(got, how) {
		t.Fatalf("listed: %+v", got)
	}
	// The page's list carries the titles alone; the instruction's own screen, its text too.
	if p := page(); len(p.Docs) != 1 || !reflect.DeepEqual(p.Docs[0].I18n, map[string]subpage.DocText{"en": {Title: "How to connect"}}) {
		t.Fatalf("the page's docs: %+v", p.Docs)
	}
	resp, raw = k.do(http.MethodGet, sub+"/docs/"+id, nil, nil)
	var shown subpage.DocView
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &shown) != nil || !reflect.DeepEqual(shown.I18n, how) || shown.Body != "Шаг 1" {
		t.Fatalf("the instruction: %d %s", resp.StatusCode, raw)
	}
	// A save without i18n, a script's that knows nothing of them, keeps the translations;
	// an empty one takes them out.
	if resp, raw := k.do(http.MethodPut, k.api+"/sub-docs/"+id, doc(nil), k.csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("update: %d %s", resp.StatusCode, raw)
	}
	if got := listed(); !reflect.DeepEqual(got, how) {
		t.Fatalf("a save without i18n lost them: %+v", got)
	}
	if resp, raw := k.do(http.MethodPut, k.api+"/sub-docs/"+id, doc(map[string]any{}), k.csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("update: %d %s", resp.StatusCode, raw)
	}
	if got := listed(); got != nil {
		t.Fatalf("an empty i18n kept them: %+v", got)
	}
	if p := page(); len(p.Docs) != 1 || p.Docs[0].I18n != nil {
		t.Fatalf("the page's docs without translations: %+v", p.Docs)
	}
}

// Translations that do not read, edited by hand, leave an instruction in its own words: in
// the API's list, in the page's and on its own screen, none of which fails for it.
func TestSubDocBrokenTranslations(t *testing.T) {
	k := newKeyHarness(t)
	sub := subscriberOf(t, k)
	resp, raw := k.do(http.MethodPost, k.api+"/sub-docs", map[string]any{"title": "Как", "emoji": "", "body": "Шаг 1", "platform": "", "published": true, "i18n": map[string]any{"en": map[string]any{"title": "How"}}}, k.csrf)
	var made struct {
		ID int64 `json:"id"`
	}
	if resp.StatusCode != http.StatusCreated || json.Unmarshal(raw, &made) != nil {
		t.Fatalf("doc: %d %s", resp.StatusCode, raw)
	}
	id := strconv.FormatInt(made.ID, 10)
	if _, err := k.st.DB.ExecContext(context.Background(), `UPDATE sub_docs SET translations = '{"en": 5}', translated_titles = '{"en": 5}' WHERE id = $1`, made.ID); err != nil {
		t.Fatal(err)
	}
	resp, raw = k.do(http.MethodGet, k.api+"/sub-docs", nil, nil)
	var list struct {
		Items []subpage.DocView `json:"items"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &list) != nil || len(list.Items) != 1 || list.Items[0].I18n != nil {
		t.Fatalf("the API's list: %d %s", resp.StatusCode, raw)
	}
	resp, body := k.do(http.MethodGet, sub, nil, browser)
	if p := pageData(t, body); resp.StatusCode != http.StatusOK || len(p.Docs) != 1 || p.Docs[0].Title != "Как" || p.Docs[0].I18n != nil {
		t.Fatalf("the page's list: %d %+v", resp.StatusCode, p.Docs)
	}
	resp, raw = k.do(http.MethodGet, sub+"/docs/"+id, nil, nil)
	var shown subpage.DocView
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &shown) != nil || shown.Body != "Шаг 1" || shown.I18n != nil {
		t.Fatalf("the instruction: %d %s", resp.StatusCode, raw)
	}
}
