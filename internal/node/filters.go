package node

import (
	"net/netip"
	"slices"
	"strings"

	"mikan/internal/nodeapi"
)

const (
	relayRule = "IN-NAME," + nodeapi.RelayListener
	// Scoped rules are the entry node's: on an exit the relay stands in for the client.
	localRule = "NOT,((" + relayRule + "))"
)

// egressRules renders destination filters before cascade and WARP. The entry
// node evaluates scoped blocks, where the authenticated slot is known.
func egressRules(st nodeapi.DesiredState) (global, scoped []string) {
	if st.Filters == nil {
		return nil, nil
	}
	e := st.Filters.Egress
	slots := slotUUIDs(st.Slots)
	exceptions := map[string][]string{}
	for _, rule := range e.Scoped {
		target := egressTarget(rule.Kind, rule.Value)
		if target == "" {
			continue
		}
		ids := ruleUsers(rule.Slots, slots)
		if rule.Scope == "allow" {
			addException(exceptions, rule.Kind, target, ids)
		} else if r := scopedRule(rule.Scope, target, ids); r != "" {
			scoped = append(scoped, r)
		}
	}
	return globalRules(e, exceptions), scoped
}

// slotUUIDs maps each slot name a rule may name to the UUID TUIC reports for it.
func slotUUIDs(slots []nodeapi.Slot) map[string]string {
	out := make(map[string]string, len(slots))
	for _, slot := range slots {
		if ruleUser(slot.Name) && ruleUser(slot.UUID) {
			out[slot.Name] = strings.ToLower(slot.UUID)
		}
	}
	return out
}

// ruleUsers are the names and UUIDs of the rule's slots that this node serves.
func ruleUsers(names []string, slots map[string]string) []string {
	var ids []string
	for _, name := range names {
		if uuid, ok := slots[name]; ok {
			ids = append(ids, name, uuid)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// scopedRule puts the destination first: IN-USER is a linear scan, so it runs only for
// connections that already go there.
func scopedRule(scope, target string, ids []string) string {
	user := "IN-USER," + strings.Join(ids, "/")
	switch {
	case scope == "only" && len(ids) > 0:
		return "AND,((" + target + "),(" + localRule + "),(" + user + ")),REJECT"
	case scope == "except" && len(ids) > 0:
		return "AND,((" + target + "),(" + localRule + "),(NOT,((" + user + ")))),REJECT"
	case scope == "except":
		return "AND,((" + target + "),(" + localRule + ")),REJECT"
	}
	return ""
}

// addException exempts the slots from the global blocks on this domain and on its
// ancestors, not from narrower blocks below it. The exit trusts the entry's decision for
// these destinations.
func addException(exceptions map[string][]string, kind, target string, ids []string) {
	if kind != "domain" || len(ids) == 0 {
		return
	}
	match := "(AND,((" + target + "),(OR,((IN-USER," + strings.Join(ids, "/") + "),(" + relayRule + ")))))"
	host := strings.TrimPrefix(target, "DOMAIN-SUFFIX,")
	for {
		exceptions[host] = append(exceptions[host], match)
		_, parent, ok := strings.Cut(host, ".")
		if !ok {
			return
		}
		host = parent
	}
}

// globalRules close the lists for everyone, each domain but for its exceptions.
func globalRules(e nodeapi.Egress, exceptions map[string][]string) []string {
	var rules []string
	for _, p := range e.Ports {
		if target := egressTarget("port", p); target != "" {
			rules = append(rules, target+",REJECT")
		}
	}
	for _, n := range e.Networks {
		if target := egressTarget("network", n); target != "" {
			rules = append(rules, target+",REJECT")
		}
	}
	for _, d := range e.Domains {
		domain, ok := nodeapi.CanonDomain(d)
		if !ok {
			continue
		}
		target := "DOMAIN-SUFFIX," + domain
		if allow := exceptions[domain]; len(allow) > 0 {
			target = "AND,((" + target + "),(NOT,((OR,(" + strings.Join(allow, ",") + ")))))"
		}
		rules = append(rules, target+",REJECT")
	}
	return rules
}

func ruleUser(s string) bool {
	return safeRuleValue(s) && !strings.ContainsAny(s, "()/")
}

func egressTarget(kind, value string) string {
	v, ok := nodeapi.CanonEgress(kind, value)
	if !ok {
		return ""
	}
	switch kind {
	case "port":
		return "DST-PORT," + v
	case "network":
		if strings.Contains(v, ":") {
			return "IP-CIDR6," + v
		}
		// As with private ranges, let the core resolve domains to check the network.
		return "IP-CIDR," + v
	default:
		return "DOMAIN-SUFFIX," + v
	}
}

// ingress is the ingress filter as the tunnel checks it: the networks as sorted ranges
// that do not overlap, so a check is a binary search. It runs for every UDP packet, and
// a list may hold a thousand networks.
type ingress struct {
	allow  bool
	ranges []addrRange
}

type addrRange struct{ first, last netip.Addr }

func newIngress(allow bool, nets []netip.Prefix) *ingress {
	rs := make([]addrRange, 0, len(nets))
	for _, p := range nets {
		p = p.Masked()
		rs = append(rs, addrRange{p.Addr(), lastAddr(p)})
	}
	// netip.Addr orders IPv4 before IPv6, so one sorted list holds both.
	slices.SortFunc(rs, func(a, b addrRange) int { return a.first.Compare(b.first) })
	in := &ingress{allow: allow}
	for _, r := range rs {
		if n := len(in.ranges); n > 0 && r.first.Compare(in.ranges[n-1].last) <= 0 {
			if r.last.Compare(in.ranges[n-1].last) > 0 {
				in.ranges[n-1].last = r.last
			}
			continue
		}
		in.ranges = append(in.ranges, r)
	}
	return in
}

// lastAddr is the last address of a masked network.
func lastAddr(p netip.Prefix) netip.Addr {
	b := p.Addr().AsSlice()
	for i := range b {
		switch n := p.Bits() - i*8; {
		case n <= 0:
			b[i] = 0xff
		case n < 8:
			b[i] |= 0xff >> n
		}
	}
	a, _ := netip.AddrFromSlice(b)
	return a
}

func (in *ingress) admits(a netip.Addr) bool {
	a = a.Unmap()
	// i is the first range that starts after a: only the one before it can hold a.
	i, _ := slices.BinarySearchFunc(in.ranges, a, func(r addrRange, a netip.Addr) int {
		if r.first.Compare(a) <= 0 {
			return -1
		}
		return 1
	})
	if i > 0 && a.Compare(in.ranges[i-1].last) <= 0 {
		return in.allow
	}
	return !in.allow
}

// SetIngress takes the ingress filter of the state; without networks it lets everyone in.
func (r *Registry) SetIngress(f *nodeapi.Filters) {
	if f == nil {
		r.ingress.Store(nil)
		return
	}
	var nets []netip.Prefix
	for _, n := range f.Ingress.Networks {
		if p, ok := nodeapi.CanonNetwork(n); ok {
			nets = append(nets, p)
		}
	}
	// An allow list with nothing that parses would shut every user out; nothing listed
	// is no filter.
	if len(nets) == 0 {
		r.ingress.Store(nil)
		return
	}
	r.ingress.Store(newIngress(f.Ingress.Allow, nets))
}

// admitFrom says whether the ingress filter lets a connection from the address in.
func (r *Registry) admitFrom(a netip.Addr) bool {
	in := r.ingress.Load()
	return in == nil || in.admits(a)
}
