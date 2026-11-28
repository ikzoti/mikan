package node

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"

	C "github.com/metacubex/mihomo/constant"

	"mikan/internal/nodeapi"
	"mikan/internal/proto"
)

// listenerNames are the listeners the state runs, the relay included: rules may name
// only these.
func listenerNames(st nodeapi.DesiredState) map[string]bool {
	present := map[string]bool{}
	for _, in := range st.Inbounds {
		present[in.Name] = true
	}
	if st.Relay != nil {
		present[nodeapi.RelayListener] = true
	}
	return present
}

// relayListener is the hidden VLESS REALITY door other nodes of the panel come in by. On the
// panel's own node it may show the panel's own HTTPS, as the node's protocols may
// (selfSteal, the state's SelfStealPort).
func relayListener(r *nodeapi.Relay, cert proto.Cert, selfSteal int) (map[string]any, error) {
	t, err := proto.FromJSON(r.Config)
	if err != nil {
		return nil, err
	}
	if t.Type() != "vless" || t["reality-config"] == nil {
		return nil, fmt.Errorf("the relay is VLESS REALITY")
	}
	return proto.Listener(t, nodeapi.RelayListener, "", r.Port, r.Users, cert, proto.Options{SelfStealPort: selfSteal})
}

// exitProxyConfig is the outbound to another node's relay. The panel builds it; the node
// only names it and keeps it to the protocol relays speak.
func exitProxyConfig(e nodeapi.Exit) (map[string]any, error) {
	if !strings.HasPrefix(e.Name, "NODE-") || !safeRuleValue(e.Name) {
		return nil, fmt.Errorf("exit name %q", e.Name)
	}
	var p map[string]any
	if err := json.Unmarshal(e.Proxy, &p); err != nil {
		return nil, fmt.Errorf("exit %s: %w", e.Name, err)
	}
	if p["type"] != "vless" {
		return nil, fmt.Errorf("exit %s: the relay is VLESS", e.Name)
	}
	p["name"] = e.Name
	return p, nil
}

// exitRules send the inbounds of each exit to that node. An inbound named by several
// exits goes to the first.
func exitRules(st nodeapi.DesiredState) []string {
	present := listenerNames(st)
	taken := map[string]bool{}
	var r []string
	for _, e := range st.Exits {
		if !strings.HasPrefix(e.Name, "NODE-") || !safeRuleValue(e.Name) {
			continue
		}
		for _, name := range e.Inbounds {
			if present[name] && !taken[name] && safeRuleValue(name) {
				taken[name] = true
				r = append(r, "IN-NAME,"+name+","+e.Name)
			}
		}
	}
	return r
}

func hasScopedNetworks(f *nodeapi.Filters) bool {
	if f != nil {
		for _, r := range f.Egress.Scoped {
			if r.Kind == "network" && (r.Scope == "only" || r.Scope == "except") && egressTarget(r.Kind, r.Value) != "" {
				return true
			}
		}
	}
	return false
}

func installDestinationChecks(rules []C.Rule, pin bool) {
	if !pin {
		return
	}
	// Preserve literal destinations from the relay before checking the exit's rules.
	// Its sniffer may recover a hostname for domain rules, even on a pinned flow.
	rules[0] = relayAddressRule{rules[0]}
	for i, r := range rules {
		if r.Adapter() == "DIRECT" || r.Adapter() == warpProxy || strings.HasPrefix(r.Adapter(), "NODE-") {
			rules[i] = checkedAddressRule{r}
		}
	}
}

type relayAddressRule struct{ C.Rule }

func (r relayAddressRule) Match(m *C.Metadata, helper C.RuleMatchHelper) (bool, string) {
	if m.InName == nodeapi.RelayListener {
		// Citrus sets RawDstAddr only when the VLESS request supplied an IP.
		if raw, ok := m.RawDstAddr.(interface{ AddrPort() netip.AddrPort }); ok {
			if ip := raw.AddrPort().Addr().Unmap(); ip.IsValid() {
				m.DstIP, m.DstIPChecked = ip, true
				m.Host = ""
			}
		}
	}
	return r.Rule.Match(m, helper)
}

type checkedAddressRule struct{ C.Rule }

func (r checkedAddressRule) Match(m *C.Metadata, helper C.RuleMatchHelper) (bool, string) {
	matched, target := r.Rule.Match(m, helper)
	if !matched || m.Host == "" {
		return matched, target
	}
	// CIDR rules have already run. Send the checked IP so DNS at a VLESS
	// exit or inside WARP cannot change the destination.
	if !m.DstIP.IsValid() {
		if helper.ResolveIP != nil {
			helper.ResolveIP() // the core remembers any lookup the CIDR rules attempted
		}
		if !m.DstIP.IsValid() {
			return true, "REJECT"
		}
	}
	m.SniffHost, m.Host = m.RuleHost(), ""
	return true, target
}
