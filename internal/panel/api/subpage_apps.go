package api

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"mikan/internal/panel/settings"
	"mikan/internal/panel/store/db"
	"mikan/internal/panel/subpage"
)

// The apps of the subscription page (#278): the built-in ones, what the admin changed of
// them and the admin's own, one document the panel's page and every group's share, with the
// apps' icons; a page keeps only its own order and hidden lists. Every change is the
// session's, as the page's is: a download button is a link every subscriber is sent to. A key
// may read it. Which apps get a subscription at all is another list (GET /api/v1/sub-apps,
// subrules.go).

// SubPageAppsView is the apps of the subscription page.
type SubPageAppsView struct {
	Apps    subpage.AppsConfig           `json:"apps" doc:"Что админ изменил у приложений и его свои приложения"`
	Presets subpage.AppCatalog           `json:"presets" doc:"Встроенные приложения без правок: к ним ведёт «Сбросить»"`
	Catalog subpage.AppCatalog           `json:"catalog" doc:"Каталог каждой страницы: встроенные приложения с правками, затем свои, по платформам, в порядке по умолчанию. Страница упорядочивает и скрывает их своими списками (config.apps)"`
	Icons   map[string]subpage.PageAsset `json:"icons" doc:"Загруженные картинки значков, по id приложения"`
	// Unreadable tells the editor that a save replaces a document the panel cannot read: the
	// admin learns it before the own apps in it are gone for good, their icons too.
	Unreadable bool `json:"unreadable,omitempty" doc:"Сохранённый документ не читается (его правили вручную или записала более новая версия): apps пуст, страницы показывают встроенные приложения, а PUT заменит документ"`
}

type subPageAppsOutput struct{ Body SubPageAppsView }

type putSubPageAppsInput struct{ Body subpage.AppsConfig }

// An app's id in a path: a built-in app's or an own one's, as long as an icon's name takes
// (subpage.AppIconAsset).
type appIconInput struct {
	ID string `path:"id" pattern:"^[a-z][a-z0-9-]{0,23}$" doc:"id приложения: встроенного (happ…) или своего, уже сохранённого (own-…)"`
}

// putAppIconInput spells out appIconInput's field: huma reads no path parameter from an
// embedded struct that is not exported.
type putAppIconInput struct {
	ID   string `path:"id" pattern:"^[a-z][a-z0-9-]{0,23}$" doc:"id приложения: встроенного (happ…) или своего, уже сохранённого (own-…)"`
	Body struct {
		Data string `json:"data" minLength:"1" maxLength:"180000" doc:"Картинка в base64: PNG, JPEG или WebP до 128 КБ, лучше квадратная. SVG не принимается"`
	}
}

func (h *handlers) registerSubPageApps() {
	tags := []string{"sub-page"}
	op := sessionOps("sub-page")
	huma.Register(h.api, huma.Operation{OperationID: "get-sub-page-apps", Method: http.MethodGet, Path: "/api/v1/sub-page/apps", Summary: "Приложения страницы подписки: встроенные, правки и свои", Tags: tags}, h.getSubPageApps)
	huma.Register(h.api, op("update-sub-page-apps", http.MethodPut, "/api/v1/sub-page/apps", "Сохранить приложения страницы подписки", 0), h.putSubPageApps)
	huma.Register(h.api, huma.Operation{OperationID: "get-sub-page-app-icon", Method: http.MethodGet, Path: "/api/v1/sub-page/apps/{id}/icon", Summary: "Значок приложения страницы подписки", Tags: tags,
		Responses: imageResponses()}, h.getAppIcon)
	put := op("put-sub-page-app-icon", http.MethodPut, "/api/v1/sub-page/apps/{id}/icon", "Загрузить значок приложения страницы подписки", 0)
	put.MaxBodyBytes = 256 << 10
	huma.Register(h.api, put, h.putAppIcon)
	huma.Register(h.api, op("delete-sub-page-app-icon", http.MethodDelete, "/api/v1/sub-page/apps/{id}/icon", "Убрать значок приложения страницы подписки", http.StatusNoContent), h.deleteAppIcon)
}

// getSubPageApps reads the document without the instructions deleted since an app was given
// one: it is sent back whole, and PUT refuses an instruction that is not there.
func (h *handlers) getSubPageApps(ctx context.Context, _ *struct{}) (*subPageAppsOutput, error) {
	c, err := h.subPages().Apps(ctx)
	unreadable := errors.Is(err, subpage.ErrUnreadable)
	switch {
	case unreadable:
		h.d.Log.Warn("sub page apps: the stored document does not read, the pages show the built-in apps", "err", err)
	case err != nil:
		return nil, err
	}
	gone, err := h.goneDocs(ctx, c)
	if err != nil {
		return nil, err
	}
	c.DropDocs(gone)
	return h.appsView(ctx, c, unreadable)
}

// appsView is the view of document c with the uploaded icons; unreadable says c stands in for a
// stored document that does not read.
func (h *handlers) appsView(ctx context.Context, c subpage.AppsConfig, unreadable bool) (*subPageAppsOutput, error) {
	assets, err := h.subPages().Assets(ctx)
	if err != nil {
		return nil, err
	}
	icons := subpage.AppIcons(assets)
	return &subPageAppsOutput{Body: SubPageAppsView{Apps: c, Presets: subpage.Presets(), Catalog: c.Catalog(icons), Icons: icons, Unreadable: unreadable}}, nil
}

// putSubPageApps takes the whole document: what it leaves out goes back to the app's own, and
// an own app left out goes with its icon.
func (h *handlers) putSubPageApps(ctx context.Context, in *putSubPageAppsInput) (*subPageAppsOutput, error) {
	c := in.Body
	if ps := subpage.NormalizeApps(&c); len(ps) > 0 {
		return nil, fieldErrs(problems("body.", ps)...)
	}
	// The instructions are looked up once the rules dropped nothing: an own app's place in c
	// is then its place in the request, which names its fields.
	gone, err := h.goneDocs(ctx, c)
	if err != nil {
		return nil, err
	}
	if len(gone) > 0 {
		details := make([]error, 0, len(gone))
		for _, r := range gone {
			details = append(details, fieldDetail("body."+r.Field, "doc_missing", r.ID))
		}
		return nil, fieldErrs(details...)
	}
	// One serializable transaction with the icon uploads (putAppIcon): an icon is never kept
	// for an app this save takes out.
	err = h.d.Store.Tx(ctx, func(q *db.Queries) error {
		if err := settings.Set(ctx, settings.New(q), settings.KeySubApps, c); err != nil {
			return err
		}
		return dropOrphanIcons(ctx, q, c)
	})
	if err != nil {
		return nil, err
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_page.apps", "", "", nil)
	// c is the document saved, with every instruction it names there: nothing to read again.
	return h.appsView(ctx, c, false)
}

// dropOrphanIcons deletes the icons of the apps c has not.
func dropOrphanIcons(ctx context.Context, q *db.Queries, c subpage.AppsConfig) error {
	rows, err := q.ListSubAssetMeta(ctx)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.Name)
	}
	orphans := c.OrphanIcons(names)
	if len(orphans) == 0 {
		return nil
	}
	_, err = q.DeleteSubAssets(ctx, orphans)
	return err
}

// goneDocs are the instructions the apps of c link to that are not there, deleted or never
// made. A draft is there: the page leaves it out until it is published.
func (h *handlers) goneDocs(ctx context.Context, c subpage.AppsConfig) ([]subpage.DocRef, error) {
	refs := c.DocRefs()
	if len(refs) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(refs))
	for _, r := range refs {
		ids = append(ids, r.ID)
	}
	found, err := h.d.Store.Q.ListSubDocIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(refs, func(r subpage.DocRef) bool { return slices.Contains(found, r.ID) }), nil
}

func (h *handlers) getAppIcon(ctx context.Context, in *appIconInput) (*subImageBytes, error) {
	return h.subImage(ctx, subpage.AppIconAsset(in.ID))
}

// putAppIcon takes the icon of a built-in app or of an own one already saved: an icon of no
// app would stay in the database for good.
func (h *handlers) putAppIcon(ctx context.Context, in *putAppIconInput) (*subImageOutput, error) {
	a, data, err := h.decodeSubImage(subpage.AppIconAsset(in.ID), in.Body.Data)
	if err != nil {
		return nil, err
	}
	err = h.d.Store.Tx(ctx, func(q *db.Queries) error {
		// A document that does not read has the built-in apps alone.
		c, err := subpage.NewService(q).Apps(ctx)
		if err != nil && !errors.Is(err, subpage.ErrUnreadable) {
			return err
		}
		if !c.Has(in.ID) {
			return huma.Error404NotFound("not_found")
		}
		return putSubAsset(ctx, q, a, data)
	})
	if err != nil {
		return nil, err
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_page.app_icon", "sub_page_app", in.ID, map[string]any{"size": a.Size, "type": a.Type})
	return &subImageOutput{Body: a}, nil
}

func (h *handlers) deleteAppIcon(ctx context.Context, in *appIconInput) (*struct{}, error) {
	n, err := h.d.Store.Q.DeleteSubAsset(ctx, subpage.AppIconAsset(in.ID))
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, huma.Error404NotFound("not_found")
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "sub_page.app_icon_delete", "sub_page_app", in.ID, nil)
	return nil, nil
}
