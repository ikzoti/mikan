package subsrc

import (
	"context"
	"slices"

	"mikan/internal/panel/domain"
	"mikan/internal/panel/store/db"
	"mikan/internal/panel/subpage"
	submodel "mikan/internal/sub/model"
)

// The appearance of a group's subscriptions (#290) is laid over the panel's config here, for
// the clients of the group alone: serve knows nothing of groups, it is served a config.

// loadAppearance reads every group's appearance and addresses into cfg.
func loadAppearance(ctx context.Context, q *db.Queries, cfg *Config) error {
	looks, err := domain.LoadGroupAppearance(ctx, q)
	if err != nil {
		return err
	}
	addrs, err := domain.LoadGroupAddresses(ctx, q)
	if err != nil {
		return err
	}
	cfg.Appearance = looks
	cfg.Addresses = make(map[int64]map[int64]submodel.AddressOverride, len(addrs))
	for group, list := range addrs {
		by := make(map[int64]submodel.AddressOverride, len(list))
		for _, a := range list {
			by[a.InboundID] = submodel.AddressOverride{Address: a.Address, Port: a.Port, SNI: a.SNI, Host: a.Host, Name: a.Name, CDN: a.CDN}
		}
		cfg.Addresses[group] = by
	}
	return nil
}

// forGroup is c as the clients of group are served by it: the group's appearance over the
// panel's settings. Group 0, and a group with none, get c as it is. c is the config every
// request shares: what changes is copied first.
func (c Config) forGroup(group int64) Config {
	a, ok := c.Appearance[group]
	if group == 0 || !ok {
		return c
	}
	if len(a.Templates) > 0 {
		c.Templates = groupTemplates(c.Templates, a.Templates)
	}
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = *v
		}
	}
	set(&c.Title, a.Title)
	set(&c.Announce, a.Announce)
	set(&c.AnnounceURL, a.AnnounceURL)
	set(&c.SupportURL, a.SupportURL)
	if a.UpdateHours != nil {
		c.UpdateHours = *a.UpdateHours
	}
	if len(a.Headers) > 0 {
		c.Headers = a.Headers
	}
	if a.RequireHWID != nil {
		c.RequireHWID = *a.RequireHWID
	}
	if a.Page {
		c.Page = group
		// The group's own brand goes with its own page: there and in its clients' apps.
		set(&c.Brand, a.Brand)
		set(&c.App.Accent, a.BrandAccent)
		if logo, ok := c.GroupLogos[group]; ok {
			c.App.LogoURL = logo
		}
	}
	return c
}

// groupLogos are the links of the groups' own logos among assets, by group: what the apps of a
// group's clients take (forGroup). base is the subscriptions' address.
func groupLogos(base string, assets map[string]subpage.PageAsset) map[int64]string {
	out := map[int64]string{}
	for name, a := range assets {
		if image, group, ok := subpage.GroupOfAsset(name); ok && image == subpage.AssetLogo {
			out[group] = base + "/" + subpage.AssetPath(name, a.Hash)
		}
	}
	return out
}

// groupTemplates are ts with a group's template of each core (own) as the core's default: a
// response rule that names a template still wins (serve.Template). A template of own that is
// gone since leaves the core's default as it is.
func groupTemplates(ts []submodel.Template, own map[submodel.Core]int64) []submodel.Template {
	out := slices.Clone(ts)
	for core, id := range own {
		if !slices.ContainsFunc(out, func(t submodel.Template) bool { return t.ID == id && t.Core == core }) {
			continue
		}
		for i := range out {
			if out[i].Core == core {
				out[i].Default = out[i].ID == id
			}
		}
	}
	return out
}

// addressOf is where the clients of group reach inbound in place of its default address; the
// zero one: as everyone does.
func (c Config) addressOf(group, inbound int64) submodel.AddressOverride {
	if group == 0 {
		return submodel.AddressOverride{}
	}
	return c.Addresses[group][inbound]
}
