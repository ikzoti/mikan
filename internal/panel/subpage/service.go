package subpage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"slices"

	"mikan/internal/panel/settings"
	"mikan/internal/panel/store/db"
)

// The page's images, one of each, and an icon of each app (AppIconAsset).
const (
	AssetLogo       = "logo"
	AssetBackground = "background"
)

// AssetLimit is how big an image of name may be; 0: no such image. A group's own logo and
// background (GroupAsset) may be as big as the panel's.
func AssetLimit(name string) int {
	if base, _, ok := GroupOfAsset(name); ok {
		name = base
	}
	switch {
	case name == AssetLogo:
		return 512 << 10
	case name == AssetBackground:
		return 2 << 20
	case appIconRe.MatchString(name):
		return AppIconLimit
	}
	return 0
}

var (
	// ErrImageType: the bytes are no PNG, JPEG or WebP (an SVG, which may carry script,
	// or an HTML page named .png among them).
	ErrImageType = errors.New("image_type")
	// ErrImageTooBig: over AssetLimit.
	ErrImageTooBig = errors.New("image_too_big")
	// ErrUnreadable: a value the admin stored does not read, edited by hand or written by a
	// newer release. What reads it gives what does read along with the error, for the caller
	// to log: the built-in apps in place of the admin's (Apps), an instruction in its own words
	// in place of its translations (Doc, Public). It goes to the log, not to the web: its text
	// is no code of errors.api.
	ErrUnreadable = errors.New("unreadable value")
)

// fatal says whether err keeps what was asked from being given: any error but ErrUnreadable.
func fatal(err error) bool { return err != nil && !errors.Is(err, ErrUnreadable) }

// CheckImage says what kind of image data is, the way a browser would sniff it, and takes
// only PNG, JPEG and WebP within the image's limit: the type is fixed by what the bytes
// are, not by a name or a header someone sent.
func CheckImage(name string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrImageType
	}
	if len(data) > AssetLimit(name) {
		return "", ErrImageTooBig
	}
	switch ct := http.DetectContentType(data); ct {
	case "image/png", "image/jpeg", "image/webp":
		return ct, nil
	}
	return "", ErrImageType
}

// Hash names a version of an image in its links.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

// AssetPath is where the page finds an image, relative to the subscription path.
func AssetPath(name, hash string) string { return "brand/" + name + "?v=" + hash }

// Service reads the page's settings, instructions and images.
type Service struct {
	q   *db.Queries
	set *settings.Settings
}

func NewService(q *db.Queries) *Service { return &Service{q: q, set: settings.New(q)} }

// Page is the stored document over the defaults, cleaned: what does not fit is shown as
// the default (an older or a hand-edited document never breaks the page).
func (s *Service) Page(ctx context.Context) (Page, error) {
	c, _, err := settings.GetOver(ctx, s.set, settings.KeySubPage, Default())
	if err != nil {
		return Default(), err
	}
	_ = Normalize(&c)
	return c, nil
}

// PageOf is the page the clients of group see: the group's own when it has one, the panel's
// otherwise (and for group 0). A group's page that does not read is shown as the panel's.
func (s *Service) PageOf(ctx context.Context, group int64) (Page, error) {
	c, _, err := s.pageOf(ctx, group)
	return c, err
}

// pageOf is PageOf, and whether the page is the group's own.
func (s *Service) pageOf(ctx context.Context, group int64) (Page, bool, error) {
	panel := func() (Page, bool, error) {
		c, err := s.Page(ctx)
		return c, false, err
	}
	if group == 0 {
		return panel()
	}
	raw, err := s.q.GetGroupPage(ctx, group)
	if errors.Is(err, sql.ErrNoRows) || err == nil && !raw.Valid {
		return panel()
	}
	if err != nil {
		return Default(), false, err
	}
	c := Default()
	if json.Unmarshal([]byte(raw.String), &c) != nil {
		return panel()
	}
	_ = Normalize(&c)
	return c, true, nil
}

// Apps is what the admin changed of the page's apps, cleaned (NormalizeApps): what does not
// fit is left out. A document that does not read at all gives none of it, with ErrUnreadable:
// the page shows the built-in apps as they come rather than none at all.
func (s *Service) Apps(ctx context.Context) (AppsConfig, error) {
	raw, found, err := s.set.Raw(ctx, settings.KeySubApps)
	if err != nil {
		return AppsConfig{}, err
	}
	var c AppsConfig
	if found {
		if err = json.Unmarshal([]byte(raw), &c); err != nil {
			c, err = AppsConfig{}, fmt.Errorf("%w: setting %s: %w", ErrUnreadable, settings.KeySubApps, err)
		}
	}
	_ = NormalizeApps(&c)
	return c, err
}

// PageAsset is an uploaded image without its bytes.
type PageAsset struct {
	Name      string `json:"name"`
	Type      string `json:"content_type"`
	Hash      string `json:"hash"`
	Size      int64  `json:"size"`
	UpdatedAt int64  `json:"updated_at"`
}

// Assets are the uploaded images by name.
func (s *Service) Assets(ctx context.Context) (map[string]PageAsset, error) {
	rows, err := s.q.ListSubAssetMeta(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]PageAsset{}
	for _, r := range rows {
		out[r.Name] = PageAsset{Name: r.Name, Type: r.ContentType, Hash: r.Hash, Size: r.Size, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}

// DocItem is an instruction as the page lists it.
type DocItem struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Emoji    string `json:"emoji,omitempty"`
	Platform string `json:"platform,omitempty"`
	// I18n are its title in the page's languages, and on its own screen (DocView) its text too.
	I18n map[string]DocText `json:"i18n,omitempty"`
}

// Public is what the page is told about itself: the document (its CSS goes into a style
// of its own), the brand, links to the images and the catalogue of apps.
type Public struct {
	Config Page   `json:"config"`
	Brand  string `json:"brand"`
	// Accent is the brand's colour when the page takes it; "" keeps the palette's.
	Accent     string    `json:"accent,omitempty"`
	Logo       string    `json:"logo,omitempty"`
	Background string    `json:"background,omitempty"`
	Docs       []DocItem `json:"docs"`
	// Catalog is the catalogue without the apps the page hides (catalogOf): Config.Apps orders it.
	Catalog AppCatalog `json:"catalog"`
	css     string
}

// Public gathers the page's data, the page of group's clients (PageOf). brand and accent are
// the panel's (settings.KeyBrand, KeyBrandAccent), and so are the images, instructions and
// apps. What the admin stored that does not read is left out and the rest shown: the data then
// comes with ErrUnreadable, for the caller to log.
func (s *Service) Public(ctx context.Context, group int64, brand, accent string) (Public, error) {
	c, own, err := s.pageOf(ctx, group)
	if err != nil {
		return Public{}, err
	}
	assets, err := s.Assets(ctx)
	if err != nil {
		return Public{}, err
	}
	docs, docsErr := s.publishedDocs(ctx)
	if fatal(docsErr) {
		return Public{}, docsErr
	}
	catalog, appsErr := s.catalogOf(ctx, c.Apps, assets)
	if fatal(appsErr) {
		return Public{}, appsErr
	}
	p := sentPage(c, brand)
	p.Docs, p.Catalog = docs, catalog
	// A group's own page shows the group's own images where it has them.
	var images int64
	if own {
		images = group
	}
	p.linkImages(assets, accent, images)
	published := make(map[int64]bool, len(docs))
	for _, d := range docs {
		published[d.ID] = true
	}
	// An app's instruction that is a draft, or deleted since, is no step the visitor can open.
	p.Catalog.onlyDocs(published)
	return p, errors.Join(docsErr, appsErr)
}

// Fallback is the page's data when its settings do not read (Public fails): the default page
// with the built-in apps, which takes nothing from the database, rather than no apps at all.
func Fallback(brand string) Public {
	p := sentPage(Default(), brand)
	p.Catalog = Presets()
	return p
}

// sentPage is the data of page c as the page's source holds it, apart from its images,
// instructions and apps: its CSS goes into a style of its own (Head), and a block turned off,
// the admin's draft, is left out.
func sentPage(c Page, brand string) Public {
	p := Public{Config: c, Brand: brand, Docs: []DocItem{}, css: c.CSS}
	p.Config.CSS = ""
	p.Config.Blocks = slices.DeleteFunc(slices.Clone(c.Blocks), func(b PageBlock) bool { return !b.On })
	return p
}

// linkImages links the page's images among assets (group's own over the panel's, ImageOf), and
// the brand's colour when the page takes it. An image chosen but gone shows as the default:
// the letter, the theme.
func (p *Public) linkImages(assets map[string]PageAsset, accent string, group int64) {
	c := &p.Config
	if c.Look.Accent == "brand" && ValidColor(accent) {
		p.Accent = accent
	}
	if name, a, ok := ImageOf(assets, AssetLogo, group); ok {
		p.Logo = AssetPath(name, a.Hash)
	} else if c.Brand.Logo == "image" {
		c.Brand.Logo = "letter"
	}
	if name, a, ok := ImageOf(assets, AssetBackground, group); ok && c.Look.Background.Kind == "image" {
		p.Background = AssetPath(name, a.Hash)
	} else if c.Look.Background.Kind == "image" {
		c.Look.Background.Kind = "theme"
	}
}

// publishedDocs are the instructions the page lists, with the titles of their translations.
// One whose titles do not read is listed in its own words, with ErrUnreadable.
func (s *Service) publishedDocs(ctx context.Context) ([]DocItem, error) {
	rows, err := s.q.ListPublishedSubDocs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DocItem, 0, len(rows))
	var unread []error
	for _, d := range rows {
		titles, err := UnmarshalDocI18n(d.TranslatedTitles)
		if err != nil {
			unread = append(unread, fmt.Errorf("instruction %d: %w", d.ID, err))
		}
		out = append(out, DocItem{ID: d.ID, Title: d.Title, Emoji: d.Emoji, Platform: d.Platform, I18n: titles})
	}
	return out, errors.Join(unread...)
}

// catalogOf is the catalogue the page of lists sends its visitors. The apps the page hides are
// not in it, so an app only one group's page shows reaches that group's clients alone: the
// Mini App shows a subscriber their own page too (/<token>/page), where its "Add" goes on.
func (s *Service) catalogOf(ctx context.Context, lists PageApps, assets map[string]PageAsset) (AppCatalog, error) {
	edits, unread := s.Apps(ctx)
	if fatal(unread) {
		return AppCatalog{}, unread
	}
	c := edits.Catalog(AppIcons(assets))
	c.keepShown(lists)
	if lists.DownloadsOff {
		c.dropDownloads()
	}
	return c, unread
}

// RemoteImages says whether the page may show images from other sites: an instruction or
// a text block, in any of its languages, may hold https images. The admin's CSS may not
// (CleanCSS).
func (p Public) RemoteImages() bool {
	if len(p.Docs) > 0 {
		return true
	}
	return slices.ContainsFunc(p.Config.Blocks, func(b PageBlock) bool { return b.Type == TypeText && b.On && b.hasText() })
}

// hasText says whether b shows a text in some language: its own or a translation.
func (b PageBlock) hasText() bool {
	if b.Text != "" {
		return true
	}
	for _, t := range b.I18n {
		if t.Text != "" {
			return true
		}
	}
	return false
}

// Head is what goes into the page's <head>: the page's data for its script (a JSON block,
// which the browser does not run), the link preview messengers read and the admin's own
// CSS. root is the subscription path's absolute address (https://host/<sub path>), for
// the preview's image.
func (p Public) Head(root string) []byte {
	var b bytes.Buffer
	og := p.Config.OG
	image := og.Image && p.Logo != ""
	if og.Title != "" || og.Description != "" || image {
		meta := func(prop, content string) {
			if content != "" {
				b.WriteString(`<meta property="` + prop + `" content="` + html.EscapeString(content) + `">`)
			}
		}
		title := og.Title
		if title == "" {
			title = p.Brand
		}
		meta("og:type", "website")
		meta("og:site_name", p.Brand)
		meta("og:title", title)
		meta("og:description", og.Description)
		if og.Description != "" {
			b.WriteString(`<meta name="description" content="` + html.EscapeString(og.Description) + `">`)
		}
		if image {
			meta("og:image", root+"/"+p.Logo)
		}
		b.WriteString(`<meta name="twitter:card" content="summary">`)
	}
	// json.Marshal writes <, > and & as \u escapes: nothing in it can close the script.
	data, _ := json.Marshal(p)
	b.WriteString(`<script type="application/json" id="mikan-page">`)
	b.Write(data)
	b.WriteString(`</script>`)
	if css := p.Style(); css != "" {
		b.WriteString(`<style id="mikan-css">`)
		b.WriteString(css)
		b.WriteString(`</style>`)
	}
	return b.Bytes()
}

// Style is the admin's own CSS of the page as the page takes it (CleanCSS).
func (p Public) Style() string { return CleanCSS(p.css) }

// DocView is an instruction with its text, as the page opens it.
type DocView struct {
	DocItem
	Body string `json:"body"`
}

// Doc is a published instruction; ok is false when there is none by that id. Translations
// that do not read leave it in its own words, with ErrUnreadable.
func (s *Service) Doc(ctx context.Context, id int64) (DocView, bool, error) {
	d, err := s.q.GetPublishedSubDoc(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return DocView{}, false, nil
	}
	if err != nil {
		return DocView{}, false, err
	}
	i18n, err := UnmarshalDocI18n(d.Translations)
	item := DocItem{ID: d.ID, Title: d.Title, Emoji: d.Emoji, Platform: d.Platform, I18n: i18n}
	return DocView{DocItem: item, Body: d.Body}, true, err
}

// Image is an uploaded image with its bytes; ok is false when there is none.
func (s *Service) Image(ctx context.Context, name string) (db.SubAsset, bool, error) {
	a, err := s.q.GetSubAsset(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return a, false, nil
		}
		return a, false, err
	}
	return a, true, nil
}
