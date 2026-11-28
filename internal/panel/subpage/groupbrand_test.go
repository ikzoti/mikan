package subpage

import "testing"

// A group's own images are named for the group, as big as the panel's, and a group's own page
// shows them over the panel's where it has them.
func TestGroupAssets(t *testing.T) {
	if got := GroupAsset(AssetLogo, 12); got != "logo-g12" {
		t.Fatalf("GroupAsset = %q", got)
	}
	type whose struct {
		base  string
		group int64
		ok    bool
	}
	for name, want := range map[string]whose{
		"logo-g12":      {"logo", 12, true},
		"background-g7": {"background", 7, true},
		"logo":          {},
		"logo-g0":       {},
		"logo-g":        {},
		"logo-gx":       {},
		"logo-g12x":     {},
		"app-happ":      {},
		"icon-g3":       {},
	} {
		if base, group, ok := GroupOfAsset(name); base != want.base || group != want.group || ok != want.ok {
			t.Errorf("GroupOfAsset(%q) = %q, %d, %v; want %+v", name, base, group, ok, want)
		}
	}
	if AssetLimit("logo-g12") != AssetLimit(AssetLogo) || AssetLimit("background-g7") != AssetLimit(AssetBackground) || AssetLimit("logo-g0") != 0 {
		t.Fatal("a group's images take the limits of the panel's")
	}
	assets := map[string]PageAsset{AssetLogo: {Hash: "p"}, "logo-g7": {Hash: "g"}, AssetBackground: {Hash: "b"}}
	if name, a, ok := ImageOf(assets, AssetLogo, 7); !ok || name != "logo-g7" || a.Hash != "g" {
		t.Errorf("the group's logo: %q %+v %v", name, a, ok)
	}
	if name, a, ok := ImageOf(assets, AssetBackground, 7); !ok || name != AssetBackground || a.Hash != "b" {
		t.Errorf("the panel's background under the group's page: %q %+v %v", name, a, ok)
	}
	if name, _, ok := ImageOf(assets, AssetLogo, 0); !ok || name != AssetLogo {
		t.Errorf("the panel's page: %q %v", name, ok)
	}
	if _, _, ok := ImageOf(map[string]PageAsset{}, AssetLogo, 7); ok {
		t.Error("no image at all")
	}
}
