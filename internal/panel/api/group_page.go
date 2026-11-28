package api

import (
	"context"
	"database/sql"
	"encoding/json"
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

// A group's own subscription page (#278): the page's document the group's clients see in place
// of the panel's, with the panel's instructions, and a brand of its own: its name and colour
// go over the panel's there and in the clients' apps, its logo and background over the panel's
// on the page. Every change is the session's, as the panel's page is: a leaked key must not put
// a page in front of a group.

// GroupPageView is the page a group's clients see.
type GroupPageView struct {
	Own            bool               `json:"own" doc:"У группы своя страница; false — в config страница для всех"`
	Config         subpage.Page       `json:"config"`
	Defaults       subpage.Page       `json:"defaults" doc:"Страница по умолчанию — к ней ведёт «Сбросить»"`
	Brand          string             `json:"brand" doc:"Название бренда панели (settings.brand): его видят клиенты группы без своего"`
	BrandAccent    string             `json:"brand_accent" doc:"Цвет бренда панели (settings.brand_accent): его видят клиенты группы без своего"`
	Logo           *subpage.PageAsset `json:"logo,omitempty" doc:"Логотип панели"`
	Background     *subpage.PageAsset `json:"background,omitempty" doc:"Фон панели"`
	OwnBrand       string             `json:"own_brand" doc:"Своё название бренда группы на её странице и в приложениях её клиентов; пусто — как у панели"`
	OwnBrandAccent string             `json:"own_brand_accent" doc:"Свой цвет бренда группы, #RRGGBB; пусто — как у панели"`
	OwnLogo        *subpage.PageAsset `json:"own_logo,omitempty" doc:"Свой логотип группы: на её странице вместо логотипа панели (GET /api/v1/groups/{id}/page/images/logo)"`
	OwnBackground  *subpage.PageAsset `json:"own_background,omitempty" doc:"Свой фон группы: на её странице вместо фона панели"`
}

type groupPageOutput struct{ Body GroupPageView }

type putGroupPageInput struct {
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Config      subpage.Page `json:"config"`
		Brand       *string      `json:"brand,omitempty" maxLength:"40" doc:"Своё название бренда группы; пусто — как у панели, не передано — как было"`
		BrandAccent *string      `json:"brand_accent,omitempty" maxLength:"7" doc:"Свой цвет бренда группы, #RRGGBB; пусто — как у панели, не передано — как было"`
	}
}

type groupImageInput struct {
	ID   int64  `path:"id" minimum:"1"`
	Name string `path:"name" enum:"logo,background"`
}

type putGroupImageInput struct {
	ID   int64  `path:"id" minimum:"1"`
	Name string `path:"name" enum:"logo,background"`
	Body struct {
		Data string `json:"data" minLength:"1" maxLength:"2800000" doc:"Картинка в base64: PNG, JPEG или WebP; логотип до 512 КБ, фон до 2 МБ. SVG не принимается"`
	}
}

func (h *handlers) registerGroupPage() {
	tags := []string{"users"}
	op := sessionOps("users")
	huma.Register(h.api, huma.Operation{OperationID: "get-group-page", Method: http.MethodGet, Path: "/api/v1/groups/{id}/page", Summary: "Страница подписки группы", Tags: tags}, h.getGroupPage)
	huma.Register(h.api, op("put-group-page", http.MethodPut, "/api/v1/groups/{id}/page", "Своя страница подписки группы", 0), h.putGroupPage)
	huma.Register(h.api, op("delete-group-page", http.MethodDelete, "/api/v1/groups/{id}/page", "Группе — страница подписки для всех", http.StatusNoContent), h.deleteGroupPage)
	huma.Register(h.api, huma.Operation{OperationID: "get-group-page-image", Method: http.MethodGet, Path: "/api/v1/groups/{id}/page/images/{name}", Summary: "Свой логотип или фон группы", Tags: tags,
		Responses: imageResponses()}, h.getGroupImage)
	put := op("put-group-page-image", http.MethodPut, "/api/v1/groups/{id}/page/images/{name}", "Загрузить свой логотип или фон группы", 0)
	put.MaxBodyBytes = 3 << 20
	huma.Register(h.api, put, h.putGroupImage)
	huma.Register(h.api, op("delete-group-page-image", http.MethodDelete, "/api/v1/groups/{id}/page/images/{name}", "Убрать свой логотип или фон группы", http.StatusNoContent), h.deleteGroupImage)
}

// groupPageView is the page group id's clients see: its own, or the panel's, with the group's
// brand beside the panel's.
func (h *handlers) groupPageView(ctx context.Context, id int64) (GroupPageView, error) {
	if _, err := h.d.Store.Q.GetGroup(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return GroupPageView{}, huma.Error404NotFound("not_found")
	} else if err != nil {
		return GroupPageView{}, err
	}
	base, err := h.subPageView(ctx)
	if err != nil {
		return GroupPageView{}, err
	}
	v := GroupPageView{Config: base.Config, Defaults: base.Defaults, Brand: base.Brand, BrandAccent: base.BrandAccent, Logo: base.Logo, Background: base.Background}
	if _, v.Own, err = domain.GroupPage(ctx, h.d.Store.Q, id); err != nil {
		return v, err
	}
	if v.Own {
		if v.Config, err = h.subPages().PageOf(ctx, id); err != nil {
			return v, err
		}
	}
	if v.OwnBrand, v.OwnBrandAccent, err = domain.GroupBrand(ctx, h.d.Store.Q, id); err != nil {
		return v, err
	}
	assets, err := h.subPages().Assets(ctx)
	if err != nil {
		return v, err
	}
	if a, ok := assets[subpage.GroupAsset(subpage.AssetLogo, id)]; ok {
		v.OwnLogo = &a
	}
	if a, ok := assets[subpage.GroupAsset(subpage.AssetBackground, id)]; ok {
		v.OwnBackground = &a
	}
	return v, nil
}

func (h *handlers) getGroupPage(ctx context.Context, in *groupIDInput) (*groupPageOutput, error) {
	v, err := h.groupPageView(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return &groupPageOutput{Body: v}, nil
}

// putGroupPage gives the group its own page, held to the rules of the panel's, and its own
// brand as the body names it: the page shows only the images there are (the group's or the
// panel's) and the brand's colour only when the group or the panel has one.
func (h *handlers) putGroupPage(ctx context.Context, in *putGroupPageInput) (*groupPageOutput, error) {
	c := in.Body.Config
	details := problems("body.config.", subpage.Normalize(&c))
	brand, accent, err := domain.GroupBrand(ctx, h.d.Store.Q, in.ID)
	if err != nil {
		return nil, err
	}
	if in.Body.Brand != nil {
		brand = strings.TrimSpace(*in.Body.Brand)
		if strings.ContainsAny(brand, "\r\n") || subpage.HasControl(brand) {
			details = append(details, fieldDetail("body.brand", "one_line", nil))
		}
	}
	if in.Body.BrandAccent != nil {
		accent = strings.ToUpper(strings.TrimSpace(*in.Body.BrandAccent))
		if accent != "" && !serve.ValidAccent(accent) {
			details = append(details, fieldDetail("body.brand_accent", "color_invalid", nil))
		}
	}
	shown := accent
	if shown == "" {
		if shown, err = h.d.Settings.String(ctx, settings.KeyBrandAccent); err != nil {
			return nil, err
		}
	}
	if c.Look.Accent == "brand" && shown == "" {
		details = append(details, fieldDetail("body.config.look.accent", "color_required", nil))
	}
	missing, err := h.pageImagesMissing(ctx, c, in.ID)
	if err != nil {
		return nil, err
	}
	if details = append(details, missing...); len(details) > 0 {
		return nil, fieldErrs(details...)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	err = h.writeGroupPage(ctx, in.ID, func(q *db.Queries) error {
		if err := domain.SetGroupPage(ctx, q, in.ID, string(raw)); err != nil {
			return err
		}
		return domain.SetGroupBrand(ctx, q, in.ID, brand, accent)
	})
	if err != nil {
		return nil, err
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "group.page", "group", strconv.FormatInt(in.ID, 10), nil)
	return h.getGroupPage(ctx, &groupIDInput{ID: in.ID})
}

// deleteGroupPage gives the group the panel's page again, and the panel's brand: the group's
// own name, colour and images go with its page.
func (h *handlers) deleteGroupPage(ctx context.Context, in *groupIDInput) (*struct{}, error) {
	err := h.writeGroupPage(ctx, in.ID, func(q *db.Queries) error {
		if err := domain.SetGroupPage(ctx, q, in.ID, ""); err != nil {
			return err
		}
		if err := domain.SetGroupBrand(ctx, q, in.ID, "", ""); err != nil {
			return err
		}
		_, err := q.DeleteSubAssets(ctx, subpage.GroupAssets(in.ID))
		return err
	})
	if err != nil {
		return nil, err
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "group.page_delete", "group", strconv.FormatInt(in.ID, 10), nil)
	return nil, nil
}

func (h *handlers) getGroupImage(ctx context.Context, in *groupImageInput) (*subImageBytes, error) {
	return h.subImage(ctx, subpage.GroupAsset(in.Name, in.ID))
}

// putGroupImage takes the group's own logo or background, which its own page shows in place of
// the panel's; the apps of its clients take its logo.
func (h *handlers) putGroupImage(ctx context.Context, in *putGroupImageInput) (*subImageOutput, error) {
	a, data, err := h.decodeSubImage(subpage.GroupAsset(in.Name, in.ID), in.Body.Data)
	if err != nil {
		return nil, err
	}
	if err := h.writeGroupPage(ctx, in.ID, func(q *db.Queries) error { return putSubAsset(ctx, q, a, data) }); err != nil {
		return nil, err
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "group.page_image", "group", strconv.FormatInt(in.ID, 10), map[string]any{"image": in.Name, "size": a.Size, "type": a.Type})
	return &subImageOutput{Body: a}, nil
}

func (h *handlers) deleteGroupImage(ctx context.Context, in *groupImageInput) (*struct{}, error) {
	var n int64
	err := h.writeGroupPage(ctx, in.ID, func(q *db.Queries) error {
		var err error
		n, err = q.DeleteSubAsset(ctx, subpage.GroupAsset(in.Name, in.ID))
		return err
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, huma.Error404NotFound("not_found")
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "group.page_image_delete", "group", strconv.FormatInt(in.ID, 10), map[string]any{"image": in.Name})
	return nil, nil
}

// writeGroupPage runs write on one transaction with group id held from being deleted meanwhile
// (404 when there is none). The subscriptions' config, which says which groups have a page and
// a brand of their own, is read again.
func (h *handlers) writeGroupPage(ctx context.Context, id int64, write func(q *db.Queries) error) error {
	err := h.d.Store.Tx(ctx, func(q *db.Queries) error {
		held, err := q.LockGroups(ctx, []int64{id})
		if err != nil {
			return err
		}
		if len(held) == 0 {
			return huma.Error404NotFound("not_found")
		}
		return write(q)
	})
	if err == nil {
		settings.Touch()
	}
	return err
}
