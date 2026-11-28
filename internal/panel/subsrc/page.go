package subsrc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mikan/internal/panel/server"
	"mikan/internal/panel/subpage"
	"mikan/internal/sub/serve"
)

// headPage is a page that takes extra markup for its <head> (server.SPA).
type headPage interface {
	ServeHead(w http.ResponseWriter, r *http.Request, head []byte)
}

// servePage sends the subscription page with what the admin made of it (subpage): its data,
// the link preview and the admin's CSS, of the page of the subscriber's group when it has one
// (cfg.Page, Config.forGroup). A part of it that does not read is left out and the rest shown;
// when it cannot be read at all, the page goes in its default look with the built-in apps
// (subpage.Fallback) rather than with no apps.
func (h *Handler) servePage(w http.ResponseWriter, r *http.Request, cfg serve.Config) {
	hp, ok := h.page.(headPage)
	if !ok || h.pages == nil {
		h.page.ServeHTTP(w, r)
		return
	}
	pub := h.publicOf(r.Context(), cfg)
	if pub.RemoteImages() {
		allowRemoteImages(w.Header())
	}
	hp.ServeHead(w, r, pub.Head(subRootURL(r, cfg)))
}

// publicOf is the data of the page cfg's subscriber sees: their group's own page and brand, or
// the panel's. A part that does not read is left out and the rest shown; when none reads, the
// default page with the built-in apps (subpage.Fallback) rather than no apps.
func (h *Handler) publicOf(ctx context.Context, cfg serve.Config) subpage.Public {
	pub, err := h.pages.Public(ctx, cfg.Page, cfg.Brand, cfg.App.Accent)
	switch {
	case errors.Is(err, subpage.ErrUnreadable):
		h.warn("page-unreadable", "subscription page: some of its settings do not read, the page goes without them", "err", err)
	case err != nil:
		h.warn("page", "subscription page: its settings were not read, the default page with the built-in apps is shown", "err", err)
		pub = subpage.Fallback(cfg.Brand)
	}
	return pub
}

// pageData answers "/<token>/page": the data of the subscriber's own page, for the Mini App,
// which opens with the page for everyone (/tg) and shows a group's own page and brand once it
// knows whose subscription is on screen. The admin's CSS goes beside it.
func (h *Handler) pageData(ctx context.Context, w http.ResponseWriter, cfg serve.Config) {
	pub := h.publicOf(ctx, cfg)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	err := json.NewEncoder(w).Encode(struct {
		Page subpage.Public `json:"page"`
		CSS  string         `json:"css,omitempty"`
	}{pub, pub.Style()})
	if err != nil {
		h.warn("page-data", "subscription page: its data was not sent", "err", err)
	}
}

// allowRemoteImages lets the page show images from other sites, by https only: an instruction
// or a text block may hold some.
func allowRemoteImages(hd http.Header) {
	csp := hd.Get("Content-Security-Policy")
	if !strings.Contains(csp, "img-src 'self' data: blob: https:") {
		hd.Set("Content-Security-Policy", strings.Replace(csp, "img-src 'self' data: blob:", "img-src 'self' data: blob: https:", 1))
	}
}

// subRootURL is the subscription path's absolute address: the panel's own when it has
// one, or the page's address without its last part (the token or "tg").
func subRootURL(r *http.Request, cfg serve.Config) string {
	if cfg.SubBase != "" {
		return strings.TrimSuffix(cfg.SubBase, "/")
	}
	page := serve.PageURL(r)
	if i := strings.LastIndex(page, "/"); i > len("https://") {
		return page[:i]
	}
	return page
}

// image sends an image the admin uploaded for the page: the logo, the background or an app's
// icon. The type is the one sniffed when it was uploaded; a link with its current hash is kept
// by the browser for good.
func (h *Handler) image(w http.ResponseWriter, r *http.Request, name string) {
	if subpage.AssetLimit(name) == 0 {
		server.NotFound(w)
		return
	}
	a, ok, err := h.pages.Image(r.Context(), name)
	switch {
	case err != nil:
		internalError(w)
		return
	case !ok:
		server.NotFound(w)
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", a.ContentType)
	hd.Set("X-Content-Type-Options", "nosniff")
	// The apps that show the logo (app branding) fetch it from elsewhere than this page.
	hd.Set("Cross-Origin-Resource-Policy", "cross-origin")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hd.Set("ETag", `"`+a.Hash+`"`)
	if r.URL.Query().Get("v") == a.Hash {
		hd.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		hd.Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, "", time.Unix(a.UpdatedAt, 0), bytes.NewReader(a.Data))
}

// doc sends a published instruction for the page to open, with its translations: the page
// shows the one of its visitor's language.
func (h *Handler) doc(w http.ResponseWriter, r *http.Request, rest string) {
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id <= 0 {
		server.NotFound(w)
		return
	}
	d, ok, err := h.pages.Doc(r.Context(), id)
	if errors.Is(err, subpage.ErrUnreadable) {
		h.warn("doc-unreadable", "subscription page: an instruction's translations do not read, it is shown in its own words", "doc", id, "err", err)
		err = nil
	}
	switch {
	case err != nil:
		internalError(w)
		return
	case !ok:
		server.NotFound(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	_ = json.NewEncoder(w).Encode(d)
}
