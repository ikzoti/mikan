package subsrc

import (
	"reflect"
	"testing"

	"mikan/internal/panel/domain"
	"mikan/internal/panel/subpage"
	"mikan/internal/sub/apps"
	submodel "mikan/internal/sub/model"
	"mikan/internal/sub/serve"
)

// A group's appearance goes over the panel's config for its clients alone: the config every
// request shares is left as it was, and what the group leaves unset stays the panel's.
func TestForGroup(t *testing.T) {
	title, none, support, hours, on := "VIP {name}", "", "https://t.me/vip", 6, true
	base := Config{Config: serve.Config{Title: "Mikan", Announce: "news", AnnounceURL: "https://t.me/news", SupportURL: "https://t.me/all",
		Templates: []submodel.Template{
			{ID: 1, Core: submodel.CoreMihomo, Default: true},
			{ID: 2, Core: submodel.CoreMihomo},
			{ID: 3, Core: submodel.CoreXray, Default: true},
		}}}
	base.Appearance = map[int64]domain.GroupAppearance{
		7: {Templates: map[submodel.Core]int64{submodel.CoreMihomo: 2, submodel.CoreXray: 99}, Title: &title, Announce: &none, SupportURL: &support,
			UpdateHours: &hours, Headers: []apps.Header{{Name: "X-Group", Value: "vip"}}, RequireHWID: &on, Page: true},
		8: {Templates: map[submodel.Core]int64{}},
	}
	before := reflect.ValueOf(base.Templates).Pointer()
	got := base.forGroup(7)
	if got.Title != title || got.Announce != "" || got.AnnounceURL != "https://t.me/news" || got.SupportURL != support || got.UpdateHours != 6 ||
		len(got.Headers) != 1 || !got.RequireHWID || got.Page != 7 {
		t.Fatalf("the group's: %+v", got.Config)
	}
	// The group's Clash template is the core's default; a template gone since leaves Xray's.
	want := []bool{false, true, true}
	for i, x := range got.Templates {
		if x.Default != want[i] {
			t.Errorf("template %d default %v, want %v", x.ID, x.Default, want[i])
		}
	}
	if !base.Templates[0].Default || base.Templates[1].Default || reflect.ValueOf(base.Templates).Pointer() != before {
		t.Errorf("the shared config changed: %+v", base.Templates)
	}
	for _, g := range []int64{0, 8, 9} {
		if c := base.forGroup(g); !reflect.DeepEqual(c.Config, base.Config) {
			t.Errorf("group %d: %+v", g, c.Config)
		}
	}
}

// A group's own brand goes with its own page: its name, colour and logo for its clients and
// their apps. A group without its own page, or what a group leaves unset, keeps the panel's.
func TestForGroupBrand(t *testing.T) {
	name, accent := "VIP VPN", "#FF8800"
	panel := serve.AppBrand{Enabled: true, Accent: "#112233", LogoURL: "https://vpn.example.com/sub/brand/logo?v=1"}
	base := Config{Config: serve.Config{Brand: "Mikan", App: panel}}
	base.Appearance = map[int64]domain.GroupAppearance{
		7: {Page: true, Brand: &name, BrandAccent: &accent},
		8: {Brand: &name, BrandAccent: &accent},
		9: {Page: true},
	}
	base.GroupLogos = map[int64]string{7: "https://vpn.example.com/sub/brand/logo-g7?v=2", 9: "https://vpn.example.com/sub/brand/logo-g9?v=3"}
	if got := base.forGroup(7); got.Brand != name || got.App.Accent != accent || got.App.LogoURL != base.GroupLogos[7] {
		t.Fatalf("group 7: %+v", got.Config)
	}
	if got := base.forGroup(8); got.Brand != "Mikan" || got.App != panel {
		t.Fatalf("a group without its own page: %+v", got.Config)
	}
	if got := base.forGroup(9); got.Brand != "Mikan" || got.App.Accent != panel.Accent || got.App.LogoURL != base.GroupLogos[9] {
		t.Fatalf("a page with a logo alone: %+v", got.Config)
	}
	if base.Brand != "Mikan" || base.App != panel {
		t.Fatalf("the shared config changed: %+v", base.Config)
	}
}

// The apps of a group's clients take the group's own logo; the panel's and the other images
// are not a group's logo.
func TestGroupLogos(t *testing.T) {
	assets := map[string]subpage.PageAsset{"logo": {Hash: "a"}, "logo-g7": {Hash: "b"}, "background-g7": {Hash: "c"}, "app-happ": {Hash: "d"}}
	want := map[int64]string{7: "https://vpn.example.com/sub/" + subpage.AssetPath("logo-g7", "b")}
	if got := groupLogos("https://vpn.example.com/sub", assets); !reflect.DeepEqual(got, want) {
		t.Fatalf("group logos %v, want %v", got, want)
	}
}

// A group's address is found by group and inbound; group 0, and another group, get none.
func TestAddressOf(t *testing.T) {
	c := Config{Addresses: map[int64]map[int64]submodel.AddressOverride{7: {3: {Address: "edge.example.com"}}}}
	if a := c.addressOf(7, 3); a.Address != "edge.example.com" {
		t.Errorf("the group's: %+v", a)
	}
	for _, x := range [][2]int64{{7, 4}, {8, 3}, {0, 3}} {
		if a := c.addressOf(x[0], x[1]); a != (submodel.AddressOverride{}) {
			t.Errorf("%v: %+v", x, a)
		}
	}
}
