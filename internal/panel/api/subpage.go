package api

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"mikan/internal/panel/domain"
	"mikan/internal/panel/settings"
	"mikan/internal/panel/store/db"
	"mikan/internal/panel/subpage"
	"mikan/internal/sub/serve"
)

// The subscription page as the admin builds it (subpage): its settings document, its
// images and its instructions. Every change is the session's: it is what every subscriber
// sees, links and CSS included, and a leaked key must not be able to put a phishing page
// in front of them. A key may read it.

type SubPageView struct {
	Config      subpage.Page       `json:"config"`
	Defaults    subpage.Page       `json:"defaults" doc:"Страница по умолчанию — к ней ведёт «Сбросить»"`
	Brand       string             `json:"brand" doc:"Название бренда (settings.brand)"`
	BrandAccent string             `json:"brand_accent" doc:"Цвет бренда (settings.brand_accent); его же получают приложения с брендингом"`
	Logo        *subpage.PageAsset `json:"logo,omitempty"`
	Background  *subpage.PageAsset `json:"background,omitempty"`
}

type subPageOutput struct{ Body SubPageView }

type putSubPageInput struct {
	Body struct {
		Config      subpage.Page `json:"config"`
		Brand       *string      `json:"brand,omitempty" maxLength:"40"`
		BrandAccent *string      `json:"brand_accent,omitempty" maxLength:"7" doc:"#RRGGBB или пусто"`
	}
}

type subImageInput struct {
	Name string `path:"name" enum:"logo,background"`
}

type putSubImageInput struct {
	Name string `path:"name" enum:"logo,background"`
	Body struct {
		Data string `json:"data" minLength:"1" maxLength:"2800000" doc:"Картинка в base64: PNG, JPEG или WebP; логотип до 512 КБ, фон до 2 МБ. SVG не принимается"`
	}
}

type subImageOutput struct{ Body subpage.PageAsset }

type subImageBytes struct {
	ContentType  string `header:"Content-Type"`
	CacheControl string `header:"Cache-Control"`
	Nosniff      string `header:"X-Content-Type-Options"`
	Body         []byte
}

// SubDocView is an instruction of the subscription page.
type SubDocView struct {
	ID        int64                      `json:"id"`
	Title     string                     `json:"title"`
	Emoji     string                     `json:"emoji"`
	Body      string                     `json:"body" doc:"Markdown"`
	I18n      map[string]subpage.DocText `json:"i18n,omitempty" doc:"Заголовок и текст на языках страницы (ru, en): посетитель видит свой язык, а где перевода нет, title и body"`
	Platform  string                     `json:"platform" doc:"ios, android, windows, macos, linux; пусто — для всех"`
	Published bool                       `json:"published" doc:"Видна на странице подписки и в Mini App"`
	UpdatedAt int64                      `json:"updated_at"`
}

type SubDocBody struct {
	Title string `json:"title" minLength:"1" maxLength:"80"`
	Emoji string `json:"emoji" maxLength:"32"`
	Body  string `json:"body" maxLength:"20000" doc:"Markdown: заголовки #, списки, **жирный**, *курсив*, код в обратных кавычках, [ссылки](https://…), картинки ![](https://…)"`
	// I18n left out of a PUT keeps the translations the instruction has (UpdateSubDoc).
	I18n      map[string]subpage.DocText `json:"i18n,omitempty" doc:"Заголовок и текст на языках страницы (ru, en), по тем же правилам. Без поля PUT оставляет переводы, какие были; {} их убирает"`
	Platform  string                     `json:"platform" enum:"ios,android,windows,macos,linux,"`
	Published bool                       `json:"published"`
}

type subDocsOutput struct {
	Body struct {
		Items []SubDocView `json:"items"`
	}
}

type subDocOutput struct{ Body SubDocView }

type createSubDocInput struct{ Body SubDocBody }

type updateSubDocInput struct {
	ID   int64 `path:"id" minimum:"1"`
	Body SubDocBody
}

type subDocIDInput struct {
	ID int64 `path:"id" minimum:"1"`
}

type orderSubDocsInput struct {
	Body struct {
		IDs []int64 `json:"ids" maxItems:"50" doc:"Все инструкции в новом порядке"`
	}
}

func (h *handlers) registerSubPage() {
	tags := []string{"sub-page"}
	op := sessionOps("sub-page")
	huma.Register(h.api, huma.Operation{OperationID: "get-sub-page", Method: http.MethodGet, Path: "/api/v1/sub-page", Summary: "Страница подписки: вид, блоки, приложения", Tags: tags}, h.getSubPage)
	huma.Register(h.api, op("update-sub-page", http.MethodPut, "/api/v1/sub-page", "Сохранить страницу подписки", 0), h.putSubPage)
	huma.Register(h.api, huma.Operation{OperationID: "get-sub-page-image", Method: http.MethodGet, Path: "/api/v1/sub-page/images/{name}", Summary: "Загруженная картинка страницы подписки", Tags: tags,
		Responses: imageResponses()}, h.getSubImage)
	put := op("put-sub-page-image", http.MethodPut, "/api/v1/sub-page/images/{name}", "Загрузить логотип или фон страницы подписки", 0)
	put.MaxBodyBytes = 3 << 20
	huma.Register(h.api, put, h.putSubImage)
	huma.Register(h.api, op("delete-sub-page-image", http.MethodDelete, "/api/v1/sub-page/images/{name}", "Убрать логотип или фон страницы подписки", http.StatusNoContent), h.deleteSubImage)
	huma.Register(h.api, huma.Operation{OperationID: "list-sub-docs", Method: http.MethodGet, Path: "/api/v1/sub-docs", Summary: "Инструкции страницы подписки", Tags: tags}, h.listSubDocs)
	huma.Register(h.api, op("create-sub-doc", http.MethodPost, "/api/v1/sub-docs", "Новая инструкция", http.StatusCreated), h.createSubDoc)
	huma.Register(h.api, op("update-sub-doc", http.MethodPut, "/api/v1/sub-docs/{id}", "Изменить инструкцию", 0), h.updateSubDoc)
	huma.Register(h.api, op("delete-sub-doc", http.MethodDelete, "/api/v1/sub-docs/{id}", "Удалить инструкцию", http.StatusNoContent), h.deleteSubDoc)
	huma.Register(h.api, op("order-sub-docs", http.MethodPut, "/api/v1/sub-docs/order", "Порядок инструкций", 0), h.orderSubDocs)
}

func (h *handlers) subPages() *subpage.Service { return subpage.NewService(h.d.Store.Q) }

func (h *handlers) subPageView(ctx context.Context) (SubPageView, error) {
	s := h.subPages()
	c, err := s.Page(ctx)
	if err != nil {
		return SubPageView{}, err
	}
	v := SubPageView{Config: c, Defaults: subpage.Default()}
	if v.Brand, err = h.d.Settings.String(ctx, settings.KeyBrand); err != nil {
		return v, err
	}
	if v.Brand == "" {
		v.Brand = "VPN"
	}
	if v.BrandAccent, err = h.d.Settings.String(ctx, settings.KeyBrandAccent); err != nil {
		return v, err
	}
	assets, err := s.Assets(ctx)
	if err != nil {
		return v, err
	}
	if a, ok := assets[subpage.AssetLogo]; ok {
		v.Logo = &a
	}
	if a, ok := assets[subpage.AssetBackground]; ok {
		v.Background = &a
	}
	return v, nil
}

func (h *handlers) getSubPage(ctx context.Context, _ *struct{}) (*subPageOutput, error) {
	v, err := h.subPageView(ctx)
	if err != nil {
		return nil, err
	}
	return &subPageOutput{Body: v}, nil
}

// problems turns what subpage found into the API's field errors under prefix.
func problems(prefix string, ps []subpage.Problem) []error {
	out := make([]error, 0, len(ps))
	for _, p := range ps {
		out = append(out, fieldDetail(prefix+p.Field, p.Code, p.Value))
	}
	return out
}

func (h *handlers) putSubPage(ctx context.Context, in *putSubPageInput) (*subPageOutput, error) {
	c := in.Body.Config
	details := problems("body.config.", subpage.Normalize(&c))
	accent := ""
	if in.Body.BrandAccent != nil {
		accent = strings.ToUpper(strings.TrimSpace(*in.Body.BrandAccent))
		if accent != "" && !serve.ValidAccent(accent) {
			details = append(details, fieldDetail("body.brand_accent", "color_invalid", nil))
		}
	} else {
		saved, err := h.d.Settings.String(ctx, settings.KeyBrandAccent)
		if err != nil {
			return nil, err
		}
		accent = saved
	}
	if c.Look.Accent == "brand" && accent == "" {
		details = append(details, fieldDetail("body.brand_accent", "color_required", nil))
	}
	if in.Body.Brand != nil && (strings.ContainsAny(*in.Body.Brand, "\r\n") || subpage.HasControl(*in.Body.Brand)) {
		details = append(details, fieldDetail("body.brand", "one_line", nil))
	}
	missing, err := h.pageImagesMissing(ctx, c, 0)
	if err != nil {
		return nil, err
	}
	details = append(details, missing...)
	if len(details) > 0 {
		return nil, fieldErrs(details...)
	}
	err = h.d.Store.TxRC(ctx, func(q *db.Queries) error {
		set := settings.New(q)
		if err := settings.Set(ctx, set, settings.KeySubPage, c); err != nil {
			return err
		}
		if in.Body.Brand != nil {
			if err := settings.Set(ctx, set, settings.KeyBrand, strings.TrimSpace(*in.Body.Brand)); err != nil {
				return err
			}
		}
		if in.Body.BrandAccent != nil {
			return settings.Set(ctx, set, settings.KeyBrandAccent, accent)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_page.update", "", "", nil)
	return h.getSubPage(ctx, nil)
}

// pageImagesMissing refuses a page that shows an image not uploaded: the logo, the link
// preview's picture or the background, the panel's, or for group's own page the group's or the
// panel's (subpage.ImageOf; 0: the panel's page).
func (h *handlers) pageImagesMissing(ctx context.Context, c subpage.Page, group int64) ([]error, error) {
	assets, err := h.subPages().Assets(ctx)
	if err != nil {
		return nil, err
	}
	var out []error
	if _, _, ok := subpage.ImageOf(assets, subpage.AssetLogo, group); !ok && (c.Brand.Logo == "image" || c.OG.Image) {
		field := "body.config.brand.logo"
		if c.Brand.Logo != "image" {
			field = "body.config.og.image"
		}
		out = append(out, fieldDetail(field, "logo_missing", nil))
	}
	if _, _, ok := subpage.ImageOf(assets, subpage.AssetBackground, group); !ok && c.Look.Background.Kind == "image" {
		out = append(out, fieldDetail("body.config.look.background.kind", "background_missing", nil))
	}
	return out, nil
}

// imageResponses is what a GET of an uploaded image answers: its bytes, of the type sniffed when
// it was uploaded (subpage.CheckImage).
func imageResponses() map[string]*huma.Response {
	return map[string]*huma.Response{"200": {Description: "PNG, JPEG или WebP", Content: map[string]*huma.MediaType{"image/png": {}, "image/jpeg": {}, "image/webp": {}}}}
}

func (h *handlers) getSubImage(ctx context.Context, in *subImageInput) (*subImageBytes, error) {
	return h.subImage(ctx, in.Name)
}

// subImage is the uploaded image name for the editor, which no cache keeps.
func (h *handlers) subImage(ctx context.Context, name string) (*subImageBytes, error) {
	a, ok, err := h.subPages().Image(ctx, name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, huma.Error404NotFound("not_found")
	}
	return &subImageBytes{ContentType: a.ContentType, CacheControl: "private, no-cache", Nosniff: "nosniff", Body: a.Data}, nil
}

// decodeSubImage is the upload of image name, its base64 in data: refused at body.data unless it
// is a PNG, JPEG or WebP within the image's limit (subpage.CheckImage).
func (h *handlers) decodeSubImage(name, data string) (subpage.PageAsset, []byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
	if err != nil {
		return subpage.PageAsset{}, nil, fieldErr("body.data", "image_type")
	}
	ct, err := subpage.CheckImage(name, raw)
	switch {
	case errors.Is(err, subpage.ErrImageTooBig):
		return subpage.PageAsset{}, nil, fieldErrValue("body.data", "image_too_big", subpage.AssetLimit(name)>>10)
	case err != nil:
		return subpage.PageAsset{}, nil, fieldErr("body.data", "image_type")
	}
	return subpage.PageAsset{Name: name, Type: ct, Hash: subpage.Hash(raw), Size: int64(len(raw)), UpdatedAt: h.d.Now().Unix()}, raw, nil
}

// putSubAsset keeps image a with its bytes, data.
func putSubAsset(ctx context.Context, q *db.Queries, a subpage.PageAsset, data []byte) error {
	return q.PutSubAsset(ctx, db.PutSubAssetParams{Name: a.Name, ContentType: a.Type, Data: data, Hash: a.Hash, UpdatedAt: a.UpdatedAt})
}

func (h *handlers) putSubImage(ctx context.Context, in *putSubImageInput) (*subImageOutput, error) {
	a, data, err := h.decodeSubImage(in.Name, in.Body.Data)
	if err != nil {
		return nil, err
	}
	if err := putSubAsset(ctx, h.d.Store.Q, a, data); err != nil {
		return nil, err
	}
	// The apps' logo comes from the subscription config, built again on the next fetch.
	settings.Touch()
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_page.image", "sub_page_image", in.Name, map[string]any{"size": a.Size, "type": a.Type})
	return &subImageOutput{Body: a}, nil
}

func (h *handlers) deleteSubImage(ctx context.Context, in *subImageInput) (*struct{}, error) {
	n, err := h.d.Store.Q.DeleteSubAsset(ctx, in.Name)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, huma.Error404NotFound("not_found")
	}
	settings.Touch()
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_page.image_delete", "sub_page_image", in.Name, nil)
	return nil, nil
}

// subDocView is d as the API gives it. Translations that do not read are left out, and said in
// the log: the editor's next save of the instruction replaces them.
func (h *handlers) subDocView(d db.SubDoc) SubDocView {
	i18n, err := subpage.UnmarshalDocI18n(d.Translations)
	if err != nil {
		h.d.Log.Warn("sub docs: an instruction's translations do not read, they are left out", "doc", d.ID, "err", err)
	}
	return SubDocView{ID: d.ID, Title: d.Title, Emoji: d.Emoji, Body: d.Body, I18n: i18n, Platform: d.Platform, Published: d.Published == 1, UpdatedAt: d.UpdatedAt}
}

func (h *handlers) listSubDocs(ctx context.Context, _ *struct{}) (*subDocsOutput, error) {
	rows, err := h.d.Store.Q.ListSubDocs(ctx)
	if err != nil {
		return nil, err
	}
	out := &subDocsOutput{}
	out.Body.Items = make([]SubDocView, 0, len(rows))
	for _, d := range rows {
		out.Body.Items = append(out.Body.Items, h.subDocView(d))
	}
	return out, nil
}

// docTranslations are an instruction's translations as the database keeps them: whole, and
// their titles apart (subpage.MarshalDocTitles).
type docTranslations struct{ all, titles string }

// checkSubDoc cleans b and refuses what the page could not show; it gives the translations
// as the database keeps them.
func checkSubDoc(b *SubDocBody) (subpage.Doc, docTranslations, error) {
	d := subpage.Doc{Title: b.Title, Emoji: b.Emoji, Body: b.Body, Platform: b.Platform, I18n: b.I18n}
	if ps := subpage.NormalizeDoc(&d); len(ps) > 0 {
		return d, docTranslations{}, fieldErrs(problems("body.", ps)...)
	}
	all, err := subpage.MarshalDocI18n(d.I18n)
	if err != nil {
		return d, docTranslations{}, err
	}
	titles, err := subpage.MarshalDocTitles(d.I18n)
	return d, docTranslations{all: all, titles: titles}, err
}

func (h *handlers) createSubDoc(ctx context.Context, in *createSubDocInput) (*subDocOutput, error) {
	d, translations, err := checkSubDoc(&in.Body)
	if err != nil {
		return nil, err
	}
	var row db.SubDoc
	err = h.d.Store.Tx(ctx, func(q *db.Queries) error {
		n, err := q.CountSubDocs(ctx)
		if err != nil {
			return err
		}
		if n >= subpage.MaxDocs {
			return huma.Error409Conflict("too_many_docs", &huma.ErrorDetail{Message: "too_many_docs", Value: subpage.MaxDocs})
		}
		row, err = q.CreateSubDoc(ctx, db.CreateSubDocParams{Title: d.Title, Emoji: d.Emoji, Body: d.Body, Platform: d.Platform, Translations: translations.all, TranslatedTitles: translations.titles, Published: domain.Flag(in.Body.Published), At: h.d.Now().Unix()})
		return err
	})
	if err != nil {
		return nil, err
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_doc.create", "sub_doc", strconv.FormatInt(row.ID, 10), nil)
	return &subDocOutput{Body: h.subDocView(row)}, nil
}

func (h *handlers) updateSubDoc(ctx context.Context, in *updateSubDocInput) (*subDocOutput, error) {
	d, translations, err := checkSubDoc(&in.Body)
	if err != nil {
		return nil, err
	}
	// A body without i18n, a script's that knows nothing of them, keeps the translations.
	given := in.Body.I18n != nil
	row, err := h.d.Store.Q.UpdateSubDoc(ctx, db.UpdateSubDocParams{
		ID: in.ID, Title: d.Title, Emoji: d.Emoji, Body: d.Body, Platform: d.Platform, Published: domain.Flag(in.Body.Published), UpdatedAt: h.d.Now().Unix(),
		Translations:     sql.NullString{String: translations.all, Valid: given},
		TranslatedTitles: sql.NullString{String: translations.titles, Valid: given},
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, huma.Error404NotFound("not_found")
	}
	if err != nil {
		return nil, err
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_doc.update", "sub_doc", strconv.FormatInt(row.ID, 10), nil)
	return &subDocOutput{Body: h.subDocView(row)}, nil
}

func (h *handlers) deleteSubDoc(ctx context.Context, in *subDocIDInput) (*struct{}, error) {
	n, err := h.d.Store.Q.DeleteSubDoc(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, huma.Error404NotFound("not_found")
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_doc.delete", "sub_doc", strconv.FormatInt(in.ID, 10), nil)
	return nil, nil
}

func (h *handlers) orderSubDocs(ctx context.Context, in *orderSubDocsInput) (*subDocsOutput, error) {
	err := h.d.Store.Tx(ctx, func(q *db.Queries) error {
		seen := map[int64]bool{}
		for i, id := range in.Body.IDs {
			if seen[id] {
				return fieldErrValue("body.ids", "duplicate_id", id)
			}
			seen[id] = true
			if _, err := q.SetSubDocPosition(ctx, db.SetSubDocPositionParams{ID: id, Position: int64(i + 1)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_doc.order", "", "", nil)
	return h.listSubDocs(ctx, nil)
}
