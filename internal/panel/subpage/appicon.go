package subpage

import (
	"regexp"
	"strings"
)

// The kinds of an app's icon: the first letter of its name, which every app has unless the
// admin picks another, an emoji, or a picture the admin uploads (AppIconAsset).
const (
	IconLetter = "letter"
	IconEmoji  = "emoji"
	IconImage  = "image"
)

// AppIcon is how the admin has an app drawn in its row of the page.
type AppIcon struct {
	Kind  string `json:"kind" enum:"letter,emoji,image" doc:"letter: первая буква названия; emoji; image: картинка из PUT /api/v1/sub-page/apps/{id}/icon"`
	Emoji string `json:"emoji,omitempty" maxLength:"32"`
}

// CatalogIcon is an app's icon as the page draws it: a picture chosen but not uploaded is
// the letter.
type CatalogIcon struct {
	Kind  string `json:"kind" enum:"letter,emoji,image"`
	Emoji string `json:"emoji,omitempty"`
	Path  string `json:"path,omitempty" doc:"Картинка: путь от пути подписки, как у логотипа (brand/app-<id>?v=<hash>)"`
}

// AppIconLimit is how big an app's icon may be: the page draws it 40 pixels wide.
const AppIconLimit = 128 << 10

// appIconPrefix names an app's icon among the page's images: app-<the app's id>.
const appIconPrefix = "app-"

// appIconRe is the name of an app's icon among the page's images, as the database takes it
// (migration 0086).
var appIconRe = regexp.MustCompile(`^app-[a-z0-9-]{1,24}$`)

// AppIconAsset is the name of app id's icon among the page's images.
func AppIconAsset(id string) string { return appIconPrefix + id }

// AppIcons are the apps' icons among the page's images (Service.Assets), by app id.
func AppIcons(assets map[string]PageAsset) map[string]PageAsset {
	out := map[string]PageAsset{}
	for name, a := range assets {
		if id, ok := strings.CutPrefix(name, appIconPrefix); ok && appIconRe.MatchString(name) {
			out[id] = a
		}
	}
	return out
}

// normalizeIcon cleans icon, at field. The letter is what every app has, so it is no edit:
// nil, as an icon that cannot be drawn is.
func normalizeIcon(field string, icon *AppIcon) (*AppIcon, []Problem) {
	if icon == nil {
		return nil, nil
	}
	kind, emoji := strings.TrimSpace(icon.Kind), strings.TrimSpace(icon.Emoji)
	switch kind {
	case IconLetter:
		return nil, nil
	case IconImage:
		return &AppIcon{Kind: IconImage}, nil
	case IconEmoji:
		if emoji == "" {
			return nil, []Problem{{Field: field + ".emoji", Code: "required"}}
		}
		if !shortMark(emoji) {
			return nil, []Problem{{Field: field + ".emoji", Code: "emoji_invalid"}}
		}
		return &AppIcon{Kind: IconEmoji, Emoji: emoji}, nil
	}
	return nil, []Problem{{Field: field + ".kind", Code: "value_invalid", Value: kind}}
}

// catalogIcon is how the page draws app id, the admin's icon of it: its picture when one is
// uploaded (icons, by app id), its emoji, or the letter.
func catalogIcon(id string, icon *AppIcon, icons map[string]PageAsset) CatalogIcon {
	switch {
	case icon == nil:
	case icon.Kind == IconEmoji:
		return CatalogIcon{Kind: IconEmoji, Emoji: icon.Emoji}
	case icon.Kind == IconImage:
		if a, ok := icons[id]; ok {
			return CatalogIcon{Kind: IconImage, Path: AssetPath(AppIconAsset(id), a.Hash)}
		}
	}
	return CatalogIcon{Kind: IconLetter}
}
