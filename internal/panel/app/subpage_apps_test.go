package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"mikan/internal/panel/domain"
	"mikan/internal/panel/settings"
	"mikan/internal/panel/store/db"
	"mikan/internal/panel/subpage"
)

// subscriberOf gives k's panel a subscriber: the path of their subscription.
func subscriberOf(t *testing.T, k *keyHarness) string {
	t.Helper()
	ctx := context.Background()
	tariffs, _ := k.st.Q.ListTariffs(ctx)
	clock := func() time.Time { return k.now }
	u, err := domain.NewUsers(k.st, domain.NewPool(k.st, clock), noChanges{}, clock).Create(ctx, domain.CreateInput{Name: "Аня", TariffID: tariffs[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	return "/" + subPath + "/" + u.SubToken
}

// pageReader gives k's panel a subscriber and reads the data of their page, as their browser
// opens it.
func pageReader(t *testing.T, k *keyHarness) func() subpage.Public {
	t.Helper()
	sub := subscriberOf(t, k)
	return func() subpage.Public {
		t.Helper()
		resp, body := k.do(http.MethodGet, sub, nil, browser)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("page: %d", resp.StatusCode)
		}
		return pageData(t, body)
	}
}

// appsReader reads the view of GET or PUT /sub-page/apps out of an answer; it fails the test
// on anything else.
func appsReader(t *testing.T) func(*http.Response, []byte) subPageApps {
	return func(resp *http.Response, raw []byte) subPageApps {
		t.Helper()
		var v subPageApps
		if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &v) != nil {
			t.Fatalf("apps: %d %s", resp.StatusCode, raw)
		}
		return v
	}
}

// appIDs are the ids of a platform's apps in their order.
func appIDs(list []subpage.CatalogApp) string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.ID)
	}
	return strings.Join(out, ",")
}

// subPageApps is the body of GET /sub-page/apps.
type subPageApps struct {
	Apps       subpage.AppsConfig           `json:"apps"`
	Presets    subpage.AppCatalog           `json:"presets"`
	Catalog    subpage.AppCatalog           `json:"catalog"`
	Icons      map[string]subpage.PageAsset `json:"icons"`
	Unreadable bool                         `json:"unreadable"`
}

// appOn is app id of a platform's list, the zero app when it is not there.
func appOn(list []subpage.CatalogApp, id string) subpage.CatalogApp {
	if i := slices.IndexFunc(list, func(a subpage.CatalogApp) bool { return a.ID == id }); i >= 0 {
		return list[i]
	}
	return subpage.CatalogApp{}
}

// oneDownload is a document that gives app one download button on platform.
func oneDownload(app, platform, url string) map[string]any {
	return map[string]any{"own": []any{}, "presets": map[string]any{app: map[string]any{"platforms": map[string]any{platform: map[string]any{
		"downloads": []any{map[string]any{"kind": "mirror", "url": url}},
	}}}}}
}

// The apps of the subscription page over HTTP (#278): the built-in ones as they come, what the
// admin changes of them for every page, and the page that shows it. A change is the session's
// alone, held to the rules and refused whole.
func TestSubPageAppsOverHTTP(t *testing.T) {
	k := newKeyHarness(t)
	ctx := context.Background()
	page := pageReader(t, k)
	read := appsReader(t)

	// Nothing changed: the built-in apps as they come, in the API and on the page.
	presets := subpage.Presets()
	v := read(k.do(http.MethodGet, k.api+"/sub-page/apps", nil, nil))
	if len(v.Apps.Presets) != 0 || !reflect.DeepEqual(v.Presets, presets) || !reflect.DeepEqual(v.Catalog, presets) {
		t.Fatalf("defaults: %+v", v)
	}
	if got := page().Catalog; !reflect.DeepEqual(got, presets) {
		t.Fatalf("the page's catalogue: %+v", got)
	}

	// A key may read the apps; it may not send every subscriber to a download of its own.
	if resp, raw := k.asKey(k.read, http.MethodGet, "/sub-page/apps", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("read key: %d %s", resp.StatusCode, raw)
	}
	if resp, raw := k.asKey(k.full, http.MethodPut, "/sub-page/apps", oneDownload("happ", "ios", "https://evil.example/happ.ipa")); resp.StatusCode != http.StatusForbidden || !strings.Contains(string(raw), "session_only") {
		t.Fatalf("full key: %d %s", resp.StatusCode, raw)
	}

	// A wrong field refuses the save and is named.
	for name, c := range map[string]struct {
		body           any
		location, code string
	}{
		"a script":              {oneDownload("happ", "ios", "javascript:alert(1)"), "body.presets.happ.platforms.ios.downloads[0].url", "url_invalid"},
		"an app's scheme":       {oneDownload("happ", "ios", "happ://add/x"), "body.presets.happ.platforms.ios.downloads[0].url", "url_invalid"},
		"http":                  {oneDownload("happ", "ios", "http://example.com/happ.ipa"), "body.presets.happ.platforms.ios.downloads[0].url", "url_invalid"},
		"an unknown app":        {oneDownload("nonesuch", "ios", "https://example.com/a"), "body.presets.nonesuch", "app_unknown"},
		"a platform it lacks":   {oneDownload("streisand", "windows", "https://example.com/a"), "body.presets.streisand.platforms.windows", "value_invalid"},
		"a missing instruction": {map[string]any{"own": []any{}, "presets": map[string]any{"hiddify": map[string]any{"platforms": map[string]any{"linux": map[string]any{"doc": 999}}}}}, "body.presets.hiddify.platforms.linux.doc", "doc_missing"},
	} {
		resp, raw := k.do(http.MethodPut, k.api+"/sub-page/apps", c.body, k.csrf)
		var r refusal
		if resp.StatusCode != http.StatusUnprocessableEntity || json.Unmarshal(raw, &r) != nil || r.code(c.location) != c.code {
			t.Errorf("%s: %d %s, want %s at %s", name, resp.StatusCode, raw, c.code, c.location)
		}
	}
	if v := read(k.do(http.MethodGet, k.api+"/sub-page/apps", nil, nil)); len(v.Apps.Presets) != 0 {
		t.Fatalf("a refused save was kept: %+v", v.Apps)
	}

	// Saved: the page shows the downloads; a draft instruction waits for its publishing.
	mkDoc := func(title string, published bool) int64 {
		resp, raw := k.do(http.MethodPost, k.api+"/sub-docs", map[string]any{"title": title, "emoji": "", "body": "Шаг 1", "platform": "", "published": published}, k.csrf)
		var d struct {
			ID int64 `json:"id"`
		}
		if resp.StatusCode != http.StatusCreated || json.Unmarshal(raw, &d) != nil {
			t.Fatalf("doc: %d %s", resp.StatusCode, raw)
		}
		return d.ID
	}
	shown, draft := mkDoc("iPhone", true), mkDoc("Черновик", false)
	store := subpage.AppDownload{Kind: subpage.KindAppStore, URL: "https://apps.example.com/happ"}
	beta := subpage.AppDownload{Kind: subpage.KindLink, URL: "https://example.com/happ-beta", Label: "Бета"}
	save := map[string]any{"own": []any{}, "presets": map[string]any{
		"happ": map[string]any{"platforms": map[string]any{
			"ios":     map[string]any{"downloads": []any{store, beta}, "doc": shown},
			"android": map[string]any{"downloads": []any{}},
			"macos":   map[string]any{},
		}},
		"hiddify": map[string]any{"platforms": map[string]any{"windows": map[string]any{"doc": draft}}},
	}}
	v = read(k.do(http.MethodPut, k.api+"/sub-page/apps", save, k.csrf))
	happ := v.Apps.Presets["happ"].Platforms
	if len(v.Apps.Presets) != 2 || happ["ios"].Doc != shown || len(*happ["ios"].Downloads) != 2 || !reflect.DeepEqual((*happ["ios"].Downloads)[0], store) || happ["android"].Downloads == nil || len(*happ["android"].Downloads) != 0 {
		t.Fatalf("saved: %+v", v.Apps)
	}
	if _, ok := happ["macos"]; ok {
		t.Fatalf("an edit that changes nothing is kept: %+v", happ)
	}
	if !reflect.DeepEqual(v.Presets, presets) {
		t.Fatal("the built-in apps took the edit")
	}
	if a := appOn(v.Catalog.IOS, "happ"); !reflect.DeepEqual(a.Downloads, []subpage.AppDownload{store, beta}) || a.Doc != shown || !a.Crypt {
		t.Fatalf("catalogue: %+v", a)
	}
	if a := appOn(v.Catalog.Windows, "hiddify"); a.Doc != draft {
		t.Fatalf("a draft's id is the admin's to keep: %+v", a)
	}
	p := page()
	if a := appOn(p.Catalog.IOS, "happ"); !reflect.DeepEqual(a.Downloads, []subpage.AppDownload{store, beta}) || a.Doc != shown {
		t.Fatalf("the page's happ: %+v", a)
	}
	if a := appOn(p.Catalog.Android, "happ"); a.ID != "happ" || a.Downloads == nil || len(a.Downloads) != 0 {
		t.Fatalf("no buttons on android: %+v", a)
	}
	if a := appOn(p.Catalog.Windows, "hiddify"); a.Doc != 0 {
		t.Fatalf("the page links a draft: %+v", a)
	}
	if !reflect.DeepEqual(p.Catalog.MacOS, presets.MacOS) || !reflect.DeepEqual(p.Catalog.Linux, presets.Linux) {
		t.Fatal("platforms nobody changed moved")
	}
	rows, err := k.st.Q.ListAudit(ctx, db.ListAuditParams{BeforeID: 1 << 62, Lim: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(rows, func(r db.AuditLog) bool { return r.Action == "sub_page.apps" }) {
		t.Fatal("the save is not in the activity log")
	}

	// An instruction deleted since leaves the page and what the API reads back, which is a
	// document PUT takes again: one gone instruction does not refuse every later save.
	if resp, _ := k.do(http.MethodDelete, k.api+"/sub-docs/"+strconv.FormatInt(shown, 10), nil, k.csrf); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete doc: %d", resp.StatusCode)
	}
	if a := appOn(page().Catalog.IOS, "happ"); a.Doc != 0 || len(a.Downloads) != 2 {
		t.Fatalf("a deleted instruction on the page: %+v", a)
	}
	v = read(k.do(http.MethodGet, k.api+"/sub-page/apps", nil, nil))
	if ios := v.Apps.Presets["happ"].Platforms["ios"]; ios.Doc != 0 || ios.Downloads == nil || len(*ios.Downloads) != 2 {
		t.Fatalf("a deleted instruction in the API: %+v", ios)
	}
	if a := appOn(v.Catalog.IOS, "happ"); a.Doc != 0 {
		t.Fatalf("a deleted instruction in the API's catalogue: %+v", a)
	}
	if resp, raw := k.do(http.MethodPut, k.api+"/sub-page/apps", v.Apps, k.csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("saved back as read: %d %s", resp.StatusCode, raw)
	}

	// A stored document that does not read is none: the page shows the built-in apps, and the
	// API says so, for the editor to tell before a save replaces it. A built-in app still takes
	// an icon.
	if err := k.st.Q.SetSetting(ctx, db.SetSettingParams{Key: settings.KeySubApps, Value: `{"presets": [`}); err != nil {
		t.Fatal(err)
	}
	if got := page().Catalog; !reflect.DeepEqual(got, presets) {
		t.Fatalf("a broken document: %+v", got)
	}
	if v := read(k.do(http.MethodGet, k.api+"/sub-page/apps", nil, nil)); len(v.Apps.Presets) != 0 || v.Apps.Own == nil || !v.Unreadable {
		t.Fatalf("a broken document in the API: %+v", v)
	}
	if resp, raw := k.do(http.MethodPut, k.api+"/sub-page/apps/happ/icon", map[string]string{"data": base64.StdEncoding.EncodeToString(pngBytes(16))}, k.csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("an icon beside a broken document: %d %s", resp.StatusCode, raw)
	}
	if v := read(k.do(http.MethodPut, k.api+"/sub-page/apps", appsDoc(), k.csrf)); v.Unreadable {
		t.Fatal("a document saved anew is said not to read")
	}
}

// A page saved while its lists held the apps' names keeps its order: the names read as ids,
// in the API and on the page.
func TestSubPageOldAppNames(t *testing.T) {
	k := newKeyHarness(t)
	ctx := context.Background()
	old := `{"apps": {"platform": "auto", "qr": true, "windows": {"order": ["Koala Clash", "Kabosu"], "hidden": ["Clash Verge Rev"]}}}`
	if err := k.st.Q.SetSetting(ctx, db.SetSettingParams{Key: settings.KeySubPage, Value: old}); err != nil {
		t.Fatal(err)
	}
	resp, raw := k.do(http.MethodGet, k.api+"/sub-page", nil, nil)
	var v struct {
		Config subpage.Page `json:"config"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &v) != nil {
		t.Fatalf("page: %d %s", resp.StatusCode, raw)
	}
	if w := v.Config.Apps.Windows; strings.Join(w.Order, ",") != "koala,kabosu" || strings.Join(w.Hidden, ",") != "verge" {
		t.Fatalf("windows: %+v", w)
	}
	// Saved again as it was read, the page is taken.
	if resp, raw := k.do(http.MethodPut, k.api+"/sub-page", map[string]any{"config": v.Config}, k.csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("save: %d %s", resp.StatusCode, raw)
	}
}

// appsDoc is a document of the apps with no edits of the built-in ones and own as its own apps.
func appsDoc(own ...map[string]any) map[string]any {
	return map[string]any{"presets": map[string]any{}, "own": append([]map[string]any{}, own...)}
}

// ourApp is an own app on the iPhone and Android, with its emoji, a link and a download on the iPhone.
func ourApp() map[string]any {
	return map[string]any{
		"id": "own-ours1", "name": "Наш VPN", "family": "xray", "icon": map[string]any{"kind": "emoji", "emoji": "🛡️"}, "link": "ourvpn://add/{url_enc}#{name}",
		"platforms": map[string]any{
			"ios":     map[string]any{"note": "Для iPhone", "downloads": []any{map[string]any{"kind": "appstore", "url": "https://apps.example.com/ourvpn"}}},
			"android": map[string]any{},
		},
	}
}

// The admin's own apps over HTTP (#278): saved with the document by the session alone, held
// to the rules and refused whole, shown after the built-in apps of their platforms, on the page
// too, where each page orders and hides them as any other app.
func TestSubPageOwnAppsOverHTTP(t *testing.T) {
	k := newKeyHarness(t)
	page, read := pageReader(t, k), appsReader(t)
	put := func(body any) (*http.Response, []byte) {
		return k.do(http.MethodPut, k.api+"/sub-page/apps", body, k.csrf)
	}
	mirror := map[string]any{"id": "own-mirror", "name": "Зеркало", "family": "other", "link": "",
		"platforms": map[string]any{"ios": map[string]any{"downloads": []any{map[string]any{"kind": "mirror", "url": "https://files.example.com/app.ipa"}}}}}

	if resp, raw := k.asKey(k.full, http.MethodPut, "/sub-page/apps", appsDoc(ourApp())); resp.StatusCode != http.StatusForbidden || !strings.Contains(string(raw), "session_only") {
		t.Fatalf("full key: %d %s", resp.StatusCode, raw)
	}
	changed := func(f func(map[string]any)) map[string]any {
		a := ourApp()
		f(a)
		return a
	}
	for name, c := range map[string]struct {
		body           any
		location, code string
	}{
		"a script":               {appsDoc(changed(func(a map[string]any) { a["link"] = "javascript:alert(1)//{url}" })), "body.own[0].link", "app_link_scheme"},
		"a link with no address": {appsDoc(changed(func(a map[string]any) { a["link"] = "ourvpn://add/" })), "body.own[0].link", "app_link_url"},
		"a built-in app's id":    {appsDoc(changed(func(a map[string]any) { a["id"] = "happ" })), "body.own[0].id", "app_id_invalid"},
		"the same id twice":      {appsDoc(ourApp(), ourApp()), "body.own[1].id", "app_duplicate"},
		"no platforms":           {appsDoc(changed(func(a map[string]any) { a["platforms"] = map[string]any{} })), "body.own[0].platforms", "platforms_required"},
		"a missing instruction":  {appsDoc(changed(func(a map[string]any) { a["platforms"] = map[string]any{"ios": map[string]any{"doc": 999}} })), "body.own[0].platforms.ios.doc", "doc_missing"},
		"a download over http": {appsDoc(changed(func(a map[string]any) {
			a["platforms"] = map[string]any{"ios": map[string]any{"downloads": []any{map[string]any{"kind": "mirror", "url": "http://files.example.com/a.ipa"}}}}
		})), "body.own[0].platforms.ios.downloads[0].url", "url_invalid"},
	} {
		resp, raw := put(c.body)
		var r refusal
		if resp.StatusCode != http.StatusUnprocessableEntity || json.Unmarshal(raw, &r) != nil || r.code(c.location) != c.code {
			t.Errorf("%s: %d %s, want %s at %s", name, resp.StatusCode, raw, c.code, c.location)
		}
	}
	if v := read(k.do(http.MethodGet, k.api+"/sub-page/apps", nil, nil)); len(v.Apps.Own) != 0 {
		t.Fatalf("a refused save was kept: %+v", v.Apps.Own)
	}
	// An app the rules drop moves none of the others: a missing instruction is named at its
	// app's place in the request, once the rest is right.
	needsDoc := map[string]any{"id": "own-doc", "name": "С инструкцией", "family": "other", "link": "", "platforms": map[string]any{"ios": map[string]any{"doc": 999}}}
	codeAt := func(body any, location string) string {
		t.Helper()
		resp, raw := put(body)
		var r refusal
		if resp.StatusCode != http.StatusUnprocessableEntity || json.Unmarshal(raw, &r) != nil {
			t.Fatalf("%d %s", resp.StatusCode, raw)
		}
		return r.code(location)
	}
	badID := appsDoc(changed(func(a map[string]any) { a["id"] = "Bad ID" }), needsDoc)
	if codeAt(badID, "body.own[0].id") != "app_id_invalid" || codeAt(badID, "body.own[0].platforms.ios.doc") != "" {
		t.Error("a missing instruction named at the place of the app dropped before it")
	}
	if code := codeAt(appsDoc(ourApp(), needsDoc), "body.own[1].platforms.ios.doc"); code != "doc_missing" {
		t.Errorf("the second app's missing instruction: %q", code)
	}

	v := read(put(appsDoc(ourApp(), mirror)))
	presets := subpage.Presets()
	if len(v.Apps.Own) != 2 || v.Apps.Own[0].ID != "own-ours1" || v.Apps.Own[1].Link != "" {
		t.Fatalf("saved: %+v", v.Apps.Own)
	}
	if got, want := appIDs(v.Catalog.IOS), appIDs(presets.IOS)+",own-ours1,own-mirror"; got != want {
		t.Fatalf("ios: %s, want %s", got, want)
	}
	if got, want := appIDs(v.Catalog.Android), appIDs(presets.Android)+",own-ours1"; got != want {
		t.Fatalf("android: %s, want %s", got, want)
	}
	if !reflect.DeepEqual(v.Catalog.Windows, presets.Windows) {
		t.Fatal("windows got an app it is not on")
	}
	p := page()
	ours := appOn(p.Catalog.IOS, "own-ours1")
	if !ours.Own || ours.Name != "Наш VPN" || ours.Note != "Для iPhone" || ours.Link != "ourvpn://add/{url_enc}#{name}" || ours.Icon != (subpage.CatalogIcon{Kind: subpage.IconEmoji, Emoji: "🛡️"}) || len(ours.Downloads) != 1 {
		t.Fatalf("the page's own app: %+v", ours)
	}
	if m := appOn(p.Catalog.IOS, "own-mirror"); m.ID != "own-mirror" || m.Link != "" || len(m.Downloads) != 1 {
		t.Fatalf("the app with no link: %+v", m)
	}

	// A page orders and hides an own app by its id, as any other.
	resp, raw := k.do(http.MethodGet, k.api+"/sub-page", nil, nil)
	var view struct {
		Config subpage.Page `json:"config"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &view) != nil {
		t.Fatalf("page: %d %s", resp.StatusCode, raw)
	}
	view.Config.Apps.IOS = subpage.PageAppList{Order: []string{"own-mirror", "happ"}, Hidden: []string{"own-ours1"}}
	if resp, raw := k.do(http.MethodPut, k.api+"/sub-page", map[string]any{"config": view.Config}, k.csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("order: %d %s", resp.StatusCode, raw)
	}
	if l := page().Config.Apps.IOS; strings.Join(l.Order, ",") != "own-mirror,happ" || strings.Join(l.Hidden, ",") != "own-ours1" {
		t.Fatalf("the page's ios list: %+v", l)
	}

	// Taken out of the document, an app leaves every page.
	if v := read(put(appsDoc(mirror))); len(v.Apps.Own) != 1 || appOn(v.Catalog.Android, "own-ours1").ID != "" {
		t.Fatalf("taken out: %+v", v.Apps.Own)
	}
	if a := appOn(page().Catalog.IOS, "own-ours1"); a.ID != "" {
		t.Fatalf("the page still offers it: %+v", a)
	}
}

// The apps' icons over HTTP (#278): pictures of a built-in app or of an own one already saved,
// sniffed and held to their limit, uploaded and removed by the session alone, and shown on the
// page once the document chooses them. An own app taken out of the document takes its icon
// along in the same save.
func TestSubPageAppIconsOverHTTP(t *testing.T) {
	k := newKeyHarness(t)
	ctx := context.Background()
	page, read := pageReader(t, k), appsReader(t)
	upload := func(id string, data []byte) (*http.Response, []byte) {
		return k.do(http.MethodPut, k.api+"/sub-page/apps/"+id+"/icon", map[string]string{"data": base64.StdEncoding.EncodeToString(data)}, k.csrf)
	}
	icon := pngBytes(200)
	for what, c := range map[string]struct {
		id     string
		data   []byte
		status int
		code   string
	}{
		"svg":                      {"happ", []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`), http.StatusUnprocessableEntity, "image_type"},
		"html":                     {"happ", []byte("<html><script>alert(1)</script></html>"), http.StatusUnprocessableEntity, "image_type"},
		"too big":                  {"happ", pngBytes(128 << 10), http.StatusUnprocessableEntity, "image_too_big"},
		"an app nobody has":        {"nonesuch", icon, http.StatusNotFound, ""},
		"a known app not built in": {"flclash", icon, http.StatusNotFound, ""},
		"an own app not saved":     {"own-ours1", icon, http.StatusNotFound, ""},
		"no id at all":             {"Happ", icon, http.StatusUnprocessableEntity, ""},
	} {
		resp, raw := upload(c.id, c.data)
		if resp.StatusCode != c.status || !strings.Contains(string(raw), c.code) {
			t.Errorf("%s: %d %s, want %d %s", what, resp.StatusCode, raw, c.status, c.code)
		}
	}
	for _, c := range []struct{ method, path string }{{http.MethodPut, "/sub-page/apps/happ/icon"}, {http.MethodDelete, "/sub-page/apps/happ/icon"}} {
		if resp, raw := k.asKey(k.full, c.method, c.path, map[string]string{"data": base64.StdEncoding.EncodeToString(icon)}); resp.StatusCode != http.StatusForbidden || !strings.Contains(string(raw), "session_only") {
			t.Errorf("full key, %s: %d %s", c.method, resp.StatusCode, raw)
		}
	}

	resp, raw := upload("happ", icon)
	var happ subpage.PageAsset
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &happ) != nil || happ.Name != "app-happ" || happ.Type != "image/png" || happ.Size != int64(len(icon)) {
		t.Fatalf("happ: %d %s", resp.StatusCode, raw)
	}
	// Uploaded, it waits for the document to choose it: the letter stays.
	v := read(k.do(http.MethodGet, k.api+"/sub-page/apps", nil, nil))
	if v.Icons["happ"] != happ || appOn(v.Catalog.IOS, "happ").Icon.Kind != subpage.IconLetter {
		t.Fatalf("an icon not chosen: %+v %+v", v.Icons, appOn(v.Catalog.IOS, "happ").Icon)
	}
	ours := ourApp()
	ours["icon"] = map[string]any{"kind": "image"}
	doc := appsDoc(ours)
	doc["presets"] = map[string]any{"happ": map[string]any{"icon": map[string]any{"kind": "image"}}}
	read(k.do(http.MethodPut, k.api+"/sub-page/apps", doc, k.csrf))
	resp, raw = upload("own-ours1", icon)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the own app saved: %d %s", resp.StatusCode, raw)
	}
	path := "brand/app-happ?v=" + happ.Hash
	p := page()
	if a := appOn(p.Catalog.MacOS, "happ"); a.Icon != (subpage.CatalogIcon{Kind: subpage.IconImage, Path: path}) {
		t.Fatalf("the page's happ: %+v", a.Icon)
	}
	if a := appOn(p.Catalog.Android, "own-ours1"); a.Icon.Kind != subpage.IconImage || !strings.HasPrefix(a.Icon.Path, "brand/app-own-ours1?v=") {
		t.Fatalf("the page's own app: %+v", a.Icon)
	}
	// The picture goes as what it is, kept for good by its hash, as the logo is.
	resp, body := k.do(http.MethodGet, "/"+subPath+"/"+path, nil, nil)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/png" || resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(resp.Header.Get("Cache-Control"), "immutable") || !bytes.Equal(body, icon) {
		t.Fatalf("the icon on the page: %d %v", resp.StatusCode, resp.Header)
	}
	if resp, body := k.do(http.MethodGet, k.api+"/sub-page/apps/happ/icon", nil, nil); resp.StatusCode != http.StatusOK || !bytes.Equal(body, icon) {
		t.Fatalf("the icon in the admin: %d", resp.StatusCode)
	}
	if resp, _ := k.asKey(k.read, http.MethodGet, "/sub-page/apps/happ/icon", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("a read key reads the icon: %d", resp.StatusCode)
	}

	// Removed, the app is drawn with its letter again.
	if resp, _ := k.do(http.MethodDelete, k.api+"/sub-page/apps/happ/icon", nil, k.csrf); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	if resp, _ := k.do(http.MethodDelete, k.api+"/sub-page/apps/happ/icon", nil, k.csrf); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete again: %d", resp.StatusCode)
	}
	if a := appOn(page().Catalog.MacOS, "happ"); a.Icon.Kind != subpage.IconLetter {
		t.Fatalf("happ without its picture: %+v", a.Icon)
	}
	if resp, _ := k.do(http.MethodGet, "/"+subPath+"/"+path, nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("a removed icon is still served: %d", resp.StatusCode)
	}

	// The own app taken out of the document takes its icon along.
	if v := read(k.do(http.MethodPut, k.api+"/sub-page/apps", appsDoc(), k.csrf)); len(v.Icons) != 0 {
		t.Fatalf("icons left: %+v", v.Icons)
	}
	if _, ok, err := subpage.NewService(k.st.Q).Image(ctx, "app-own-ours1"); err != nil || ok {
		t.Fatalf("the own app's icon is kept: %v %v", ok, err)
	}
	rows, err := k.st.Q.ListAudit(ctx, db.ListAuditParams{BeforeID: 1 << 62, Lim: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"sub_page.app_icon", "sub_page.app_icon_delete"} {
		if !slices.ContainsFunc(rows, func(r db.AuditLog) bool { return r.Action == action && r.TargetType.String == "sub_page_app" }) {
			t.Errorf("%s is not in the activity log", action)
		}
	}
}

// A page's data holds the apps it shows (#278): an app the page for everyone hides is on
// neither its subscribers' pages nor the Mini App's, so an app a group's page alone shows
// reaches that group's clients alone. A group's own page keeps what the page for everyone
// shows, since the Mini App's "Add" opens on it: only an app both hide is left out.
func TestSubPageHiddenAppsStayOut(t *testing.T) {
	k := newKeyHarness(t)
	ctx := context.Background()
	ok := func(resp *http.Response, raw []byte, what string) {
		t.Helper()
		if resp.StatusCode/100 != 2 {
			t.Fatalf("%s: %d %s", what, resp.StatusCode, raw)
		}
	}
	resp, raw := k.do(http.MethodPut, k.api+"/sub-page/apps", appsDoc(ourApp()), k.csrf)
	ok(resp, raw, "apps")
	resp, raw = k.do(http.MethodGet, k.api+"/sub-page", nil, nil)
	var view struct {
		Config subpage.Page `json:"config"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &view) != nil {
		t.Fatalf("page: %d %s", resp.StatusCode, raw)
	}
	panel := view.Config
	panel.Apps.IOS = subpage.PageAppList{Order: []string{}, Hidden: []string{"own-ours1", "happ", "streisand"}}
	resp, raw = k.do(http.MethodPut, k.api+"/sub-page", map[string]any{"config": panel}, k.csrf)
	ok(resp, raw, "the panel's page")

	tariffs, _ := k.st.Q.ListTariffs(ctx)
	clock := func() time.Time { return k.now }
	users := domain.NewUsers(k.st, domain.NewPool(k.st, clock), noChanges{}, clock)
	vip, err := users.Create(ctx, domain.CreateInput{Name: "vip", TariffID: tariffs[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	other, err := users.Create(ctx, domain.CreateInput{Name: "other", TariffID: tariffs[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	resp, raw = k.do(http.MethodPost, k.api+"/groups", map[string]any{"name": "VIP"}, k.csrf)
	var g struct {
		ID int64 `json:"id"`
	}
	if resp.StatusCode != http.StatusCreated || json.Unmarshal(raw, &g) != nil {
		t.Fatalf("group: %d %s", resp.StatusCode, raw)
	}
	resp, raw = k.do(http.MethodPatch, k.api+"/users/"+strconv.FormatInt(vip.ID, 10), map[string]any{"group_id": g.ID}, k.csrf)
	ok(resp, raw, "into the group")
	own := panel
	own.Apps.IOS = subpage.PageAppList{Order: []string{"own-ours1"}, Hidden: []string{"incy", "streisand"}}
	resp, raw = k.do(http.MethodPut, k.api+"/groups/"+strconv.FormatInt(g.ID, 10)+"/page", map[string]any{"config": own}, k.csrf)
	ok(resp, raw, "the group's page")

	ios := func(path string) string {
		t.Helper()
		resp, body := k.do(http.MethodGet, "/"+subPath+"/"+path, nil, browser)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
		return appIDs(pageData(t, body).Catalog.IOS)
	}
	for what, c := range map[string]struct{ path, want string }{
		"the panel's page": {other.SubToken, "incy,v2raytun"},
		"the Mini App":     {"tg", "incy,v2raytun"},
		"the group's page": {vip.SubToken, "happ,incy,v2raytun,own-ours1"},
	} {
		if got := ios(c.path); got != c.want {
			t.Errorf("%s: %s, want %s", what, got, c.want)
		}
	}
}

// A page whose settings do not read goes in its default look with the built-in apps rather
// than with no apps at all.
func TestSubPageFallbackKeepsTheApps(t *testing.T) {
	k := newKeyHarness(t)
	page := pageReader(t, k)
	if err := k.st.Q.SetSetting(context.Background(), db.SetSettingParams{Key: settings.KeySubPage, Value: `{"look": [`}); err != nil {
		t.Fatal(err)
	}
	def := subpage.Default()
	def.Blocks = slices.DeleteFunc(def.Blocks, func(b subpage.PageBlock) bool { return !b.On })
	if p := page(); !reflect.DeepEqual(p.Config, def) || !reflect.DeepEqual(p.Catalog, subpage.Presets()) {
		t.Fatalf("the page of broken settings: %+v", p)
	}
}
