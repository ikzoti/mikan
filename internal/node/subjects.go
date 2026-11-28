package node

import (
	"slices"

	C "github.com/metacubex/mihomo/constant"

	"mikan/internal/nodeapi"
)

// subjects are the clients that scoped rules decide for on this node: the slots an
// "only" rule names and everyone an "except" rule does not exempt. Only they pay for the
// per-destination checks; everyone else keeps the core's usual routing.
type subjects struct {
	named  map[string]bool
	exempt []map[string]bool // one set per "except" rule
}

// scopedSubjects collects the clients of the blocking scoped rules of these kinds, of
// every kind when none is given; nil when such a rule decides for no one.
func scopedSubjects(st nodeapi.DesiredState, kinds ...string) *subjects {
	if st.Filters == nil {
		return nil
	}
	slots := slotUUIDs(st.Slots)
	s := &subjects{named: map[string]bool{}}
	for _, rule := range st.Filters.Egress.Scoped {
		if len(kinds) > 0 && !slices.Contains(kinds, rule.Kind) || egressTarget(rule.Kind, rule.Value) == "" {
			continue
		}
		ids := ruleUsers(rule.Slots, slots)
		switch rule.Scope {
		case "only":
			for _, id := range ids {
				s.named[id] = true
			}
		case "except":
			exempt := make(map[string]bool, len(ids))
			for _, id := range ids {
				exempt[id] = true
			}
			s.exempt = append(s.exempt, exempt)
		}
	}
	if len(s.named) == 0 && len(s.exempt) == 0 {
		return nil
	}
	return s
}

// has reports whether the rules decide for a local connection. Relayed traffic was the
// entry node's to check: the exit sees the relay, not the client.
func (s *subjects) has(m *C.Metadata) bool {
	if s == nil || m.InName == nodeapi.RelayListener {
		return false
	}
	if s.named[m.InUser] {
		return true
	}
	for _, exempt := range s.exempt {
		if !exempt[m.InUser] {
			return true
		}
	}
	return false
}
