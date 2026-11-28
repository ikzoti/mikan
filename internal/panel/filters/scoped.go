package filters

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"mikan/internal/nodeapi"
	"mikan/internal/panel/settings"
	"mikan/internal/panel/store/db"
)

// AudienceRule selects clients by ID, by client folder, or by tariff. Any selected group
// or client is enough for membership.
type AudienceRule struct {
	Kind    string  `json:"kind" enum:"port,network,domain"`
	Value   string  `json:"value" maxLength:"1024"`
	Scope   string  `json:"scope" enum:"only,except"`
	Users   []int64 `json:"users,omitempty" maxItems:"1000"`
	Folders []int64 `json:"folders,omitempty" maxItems:"1000"`
	Tariffs []int64 `json:"tariffs,omitempty" maxItems:"1000"`
	Groups  []int64 `json:"groups,omitempty" maxItems:"1000"`
}

// The core limits the number of clauses in an expression. Keep the compiled domain
// exceptions below that limit even when every personal rule is an exception.
const MaxSiteOverrides = 500

var ErrSiteLimit = errors.New("filters_site_limit")

// SiteFilterOverride is an individual override created in the client card.
type SiteFilterOverride struct {
	UserID int64  `json:"user_id"`
	Domain string `json:"domain"`
	Allow  bool   `json:"allow"`
}

func (e Egress) HasScoped() bool {
	return len(e.Sites) > 0 || e.Enabled && len(e.Rules) > 0
}

func (e *Egress) validateScoped() error {
	if len(e.Sites) > MaxSiteOverrides {
		return ErrSiteLimit
	}
	if len(e.Rules) > MaxItems {
		return ErrConfig
	}
	for i := range e.Rules {
		r := &e.Rules[i]
		value, ok := nodeapi.CanonEgress(r.Kind, r.Value)
		if !ok {
			return &Problem{List: "egress.rules", Index: i, Value: entryValue(r.Value)}
		}
		r.Value = value
		if r.Scope != "only" && r.Scope != "except" || !r.hasAudience() {
			return &Problem{List: "egress.rules", Index: i, Value: entryValue(r.Value)}
		}
		for _, ids := range []*[]int64{&r.Users, &r.Folders, &r.Tariffs, &r.Groups} {
			if len(*ids) > MaxItems || slices.ContainsFunc(*ids, func(id int64) bool { return id < 1 }) {
				return &Problem{List: "egress.rules", Index: i, Value: entryValue(r.Value)}
			}
			*ids = slices.Clone(*ids)
			slices.Sort(*ids)
			*ids = slices.Compact(*ids)
		}
	}
	seen := make(map[struct {
		user int64
		host string
	}]bool, len(e.Sites))
	for i := range e.Sites {
		site := &e.Sites[i]
		host, ok := nodeapi.CanonDomain(site.Domain)
		key := struct {
			user int64
			host string
		}{site.UserID, host}
		if !ok || site.UserID < 1 || seen[key] {
			return ErrConfig
		}
		seen[key] = true
		site.Domain = host
	}
	return nil
}

func (r AudienceRule) hasAudience() bool {
	return len(r.Users)+len(r.Folders)+len(r.Tariffs)+len(r.Groups) > 0
}

// DropAudience removes deleted references while holding the settings row lock. A
// READ COMMITTED bulk delete then sees the last saved filters before changing them.
func DropAudience(ctx context.Context, q *db.Queries, kind string, ids []int64) error {
	if _, err := q.LockSetting(ctx, KeyConfig); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	set := settings.New(q)
	c, err := Load(ctx, set)
	if err != nil {
		return err
	}
	removed := make(map[int64]bool, len(ids))
	for _, id := range ids {
		removed[id] = true
	}
	changed := false
	if kind == "users" {
		before := len(c.Egress.Sites)
		c.Egress.Sites = slices.DeleteFunc(c.Egress.Sites, func(site SiteFilterOverride) bool { return removed[site.UserID] })
		changed = len(c.Egress.Sites) != before
	}
	rules := c.Egress.Rules[:0]
	for _, rule := range c.Egress.Rules {
		var selected *[]int64
		switch kind {
		case "users":
			selected = &rule.Users
		case "folders":
			selected = &rule.Folders
		case "groups":
			selected = &rule.Groups
		default:
			return ErrConfig
		}
		before := len(*selected)
		*selected = slices.DeleteFunc(*selected, func(id int64) bool { return removed[id] })
		changed = changed || len(*selected) != before
		if rule.hasAudience() {
			rules = append(rules, rule)
		}
	}
	c.Egress.Rules = rules
	if changed {
		return settings.Set(ctx, set, KeyConfig, c)
	}
	return nil
}
