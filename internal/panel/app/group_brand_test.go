package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"mikan/internal/panel/domain"
	"mikan/internal/panel/store/db"
	"mikan/internal/panel/subpage"
)

// groupPageBody is the body of GET and PUT /groups/{id}/page.
type groupPageBody struct {
	Own            bool               `json:"own"`
	Config         subpage.Page       `json:"config"`
	Brand          string             `json:"brand"`
	OwnBrand       string             `json:"own_brand"`
	OwnBrandAccent string             `json:"own_brand_accent"`
	OwnLogo        *subpage.PageAsset `json:"own_logo"`
	OwnBackground  *subpage.PageAsset `json:"own_background"`
}

// A group's own brand over HTTP (#278): its name and colour go with its own page, there and in
// the info its clients' page reads, and its logo stands in for the panel's on it. The session
// alone changes it; the page taken back, or the group deleted, takes it along.
func TestGroupBrandOverHTTP(t *testing.T) {
	k := newKeyHarness(t)
	ctx := context.Background()
	resp, raw := k.do(http.MethodPost, k.api+"/groups", map[string]any{"name": "VIP"}, k.csrf)
	var g struct {
		ID int64 `json:"id"`
	}
	if resp.StatusCode != http.StatusCreated || json.Unmarshal(raw, &g) != nil {
		t.Fatalf("group: %d %s", resp.StatusCode, raw)
	}
	gid := strconv.FormatInt(g.ID, 10)
	tariffs, _ := k.st.Q.ListTariffs(ctx)
	clock := func() time.Time { return k.now }
	vip, err := domain.NewUsers(k.st, domain.NewPool(k.st, clock), noChanges{}, clock).Create(ctx, domain.CreateInput{Name: "vip", TariffID: tariffs[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if resp, raw := k.do(http.MethodPatch, k.api+"/users/"+strconv.FormatInt(vip.ID, 10), map[string]any{"group_id": g.ID}, k.csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("into the group: %d %s", resp.StatusCode, raw)
	}
	others := pageReader(t, k)
	page := func() subpage.Public {
		t.Helper()
		resp, body := k.do(http.MethodGet, "/"+subPath+"/"+vip.SubToken, nil, browser)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the group's page: %d", resp.StatusCode)
		}
		return pageData(t, body)
	}
	readPage := func(resp *http.Response, raw []byte) groupPageBody {
		t.Helper()
		var v groupPageBody
		if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &v) != nil {
			t.Fatalf("group page: %d %s", resp.StatusCode, raw)
		}
		return v
	}
	images := k.api + "/groups/" + gid + "/page/images/"
	upload := func(name string, data []byte) (*http.Response, []byte) {
		return k.do(http.MethodPut, images+name, map[string]string{"data": base64.StdEncoding.EncodeToString(data)}, k.csrf)
	}

	// The group's own logo, by the session alone, of a group there is.
	logo := pngBytes(300)
	for what, c := range map[string]struct {
		resp   *http.Response
		status int
	}{
		"an image no page has": {first(upload("icon", logo)), http.StatusUnprocessableEntity},
		"no group":             {first(k.do(http.MethodPut, k.api+"/groups/999999/page/images/logo", map[string]string{"data": base64.StdEncoding.EncodeToString(logo)}, k.csrf)), http.StatusNotFound},
		"svg":                  {first(upload("logo", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))), http.StatusUnprocessableEntity},
	} {
		if c.resp.StatusCode != c.status {
			t.Errorf("%s: %d, want %d", what, c.resp.StatusCode, c.status)
		}
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		if resp, raw := k.asKey(k.full, method, "/groups/"+gid+"/page/images/logo", map[string]string{"data": base64.StdEncoding.EncodeToString(logo)}); resp.StatusCode != http.StatusForbidden || !strings.Contains(string(raw), "session_only") {
			t.Errorf("full key, %s: %d %s", method, resp.StatusCode, raw)
		}
	}
	resp, raw = upload("logo", logo)
	var own subpage.PageAsset
	if resp.StatusCode != http.StatusOK || json.Unmarshal(raw, &own) != nil || own.Name != "logo-g"+gid {
		t.Fatalf("logo: %d %s", resp.StatusCode, raw)
	}
	v := readPage(k.do(http.MethodGet, k.api+"/groups/"+gid+"/page", nil, nil))
	if v.Own || v.OwnLogo == nil || *v.OwnLogo != own || v.OwnBrand != "" {
		t.Fatalf("the group before its page: %+v", v)
	}

	// Its own page: the panel has neither a logo nor a colour, the group's make the page pass.
	c := v.Config
	c.Brand.Logo = "image"
	c.Look.Accent = "brand"
	for what, body := range map[string]map[string]any{
		"a colour that is none":    {"config": c, "brand": "VIP", "brand_accent": "orange"},
		"a name of two lines":      {"config": c, "brand": "VIP\nVPN", "brand_accent": "#ff8800"},
		"the brand's colour, none": {"config": c, "brand": "VIP"},
		"a name too long for apps": {"config": c, "brand": strings.Repeat("я", 41), "brand_accent": "#ff8800"},
	} {
		if resp, raw := k.do(http.MethodPut, k.api+"/groups/"+gid+"/page", body, k.csrf); resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", what, resp.StatusCode, raw)
		}
	}
	v = readPage(k.do(http.MethodPut, k.api+"/groups/"+gid+"/page", map[string]any{"config": c, "brand": " VIP VPN ", "brand_accent": "#ff8800"}, k.csrf))
	if !v.Own || v.OwnBrand != "VIP VPN" || v.OwnBrandAccent != "#FF8800" {
		t.Fatalf("the group's page: %+v", v)
	}
	p := page()
	if p.Brand != "VIP VPN" || p.Accent != "#FF8800" || p.Logo != "brand/logo-g"+gid+"?v="+own.Hash {
		t.Fatalf("what the group's clients see: brand %q, accent %q, logo %q", p.Brand, p.Accent, p.Logo)
	}
	resp, body := k.do(http.MethodGet, "/"+subPath+"/"+vip.SubToken+"/info", nil, browser)
	var info struct {
		Brand string `json:"brand"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &info) != nil || info.Brand != "VIP VPN" {
		t.Fatalf("the info: %d %s", resp.StatusCode, body)
	}
	if resp, body := k.do(http.MethodGet, "/"+subPath+"/"+p.Logo, nil, nil); resp.StatusCode != http.StatusOK || !bytes.Equal(body, logo) {
		t.Fatalf("the group's logo on its page: %d", resp.StatusCode)
	}
	if o := others(); o.Brand == "VIP VPN" || o.Logo != "" {
		t.Fatalf("another client sees the group's brand: %q %q", o.Brand, o.Logo)
	}
	// A save that names no brand keeps the one there is.
	if v = readPage(k.do(http.MethodPut, k.api+"/groups/"+gid+"/page", map[string]any{"config": c}, k.csrf)); v.OwnBrand != "VIP VPN" || v.OwnBrandAccent != "#FF8800" {
		t.Fatalf("a save without the brand: %+v", v)
	}

	// An image removed, and the page taken back, which takes the brand along.
	if resp, raw := upload("background", pngBytes(400)); resp.StatusCode != http.StatusOK {
		t.Fatalf("background: %d %s", resp.StatusCode, raw)
	}
	if resp, _ := k.do(http.MethodDelete, images+"background", nil, k.csrf); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete the background: %d", resp.StatusCode)
	}
	if resp, _ := k.do(http.MethodDelete, images+"background", nil, k.csrf); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete it again: %d", resp.StatusCode)
	}
	if resp, raw := k.do(http.MethodDelete, k.api+"/groups/"+gid+"/page", nil, k.csrf); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("the page taken back: %d %s", resp.StatusCode, raw)
	}
	v = readPage(k.do(http.MethodGet, k.api+"/groups/"+gid+"/page", nil, nil))
	if v.Own || v.OwnBrand != "" || v.OwnBrandAccent != "" || v.OwnLogo != nil {
		t.Fatalf("the brand after the page: %+v", v)
	}
	if resp, _ := k.do(http.MethodGet, images+"logo", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("the logo after the page: %d", resp.StatusCode)
	}
	if p := page(); p.Brand == "VIP VPN" {
		t.Fatalf("the group's clients after the page: %q", p.Brand)
	}

	// The group deleted takes its images along.
	if resp, raw := upload("background", pngBytes(400)); resp.StatusCode != http.StatusOK {
		t.Fatalf("background again: %d %s", resp.StatusCode, raw)
	}
	if resp, raw := k.do(http.MethodDelete, k.api+"/groups/"+gid, nil, k.csrf); resp.StatusCode/100 != 2 {
		t.Fatalf("delete the group: %d %s", resp.StatusCode, raw)
	}
	if _, ok, err := subpage.NewService(k.st.Q).Image(ctx, "background-g"+gid); err != nil || ok {
		t.Fatalf("the group's background outlived it: %v %v", ok, err)
	}
	rows, err := k.st.Q.ListAudit(ctx, db.ListAuditParams{BeforeID: 1 << 62, Lim: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"group.page_image", "group.page_image_delete"} {
		if !slices.ContainsFunc(rows, func(r db.AuditLog) bool { return r.Action == action && r.TargetID.String == gid }) {
			t.Errorf("%s is not in the activity log", action)
		}
	}
}

// first is the response of a call of k.do, without its body.
func first(resp *http.Response, _ []byte) *http.Response { return resp }
