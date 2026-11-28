package subpage

import (
	"regexp"
	"strconv"
)

// A group's own brand (#278): on the group's own page its logo and background stand in for
// the panel's, kept beside them under names of the group (migration 0088).

var groupAssetRe = regexp.MustCompile(`^(logo|background)-g[1-9][0-9]{0,18}$`)

// GroupAsset is the name of image name (AssetLogo, AssetBackground) of group's own brand.
func GroupAsset(name string, group int64) string { return name + "-g" + strconv.FormatInt(group, 10) }

// GroupAssets are the names of every image group may have of its own.
func GroupAssets(group int64) []string {
	return []string{GroupAsset(AssetLogo, group), GroupAsset(AssetBackground, group)}
}

// GroupOfAsset says whose own image name is: the group, and the panel's name of the image.
func GroupOfAsset(name string) (base string, group int64, ok bool) {
	m := groupAssetRe.FindStringSubmatch(name)
	if m == nil {
		return "", 0, false
	}
	group, err := strconv.ParseInt(name[len(m[1])+len("-g"):], 10, 64)
	return m[1], group, err == nil
}

// ImageOf is image name (AssetLogo, AssetBackground) as a page shows it, out of assets: the
// group's own when the page is a group's own page with one (group, 0 for the panel's page),
// the panel's otherwise; ok is false when there is neither.
func ImageOf(assets map[string]PageAsset, name string, group int64) (asset string, a PageAsset, ok bool) {
	if group != 0 {
		if a, ok := assets[GroupAsset(name, group)]; ok {
			return GroupAsset(name, group), a, true
		}
	}
	a, ok = assets[name]
	return name, a, ok
}
