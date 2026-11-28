package node

import (
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/hub/executor"

	"mikan/internal/nodeapi"
	"mikan/internal/proto"
)

const relayKey = "SN3ZEwGwUfpBNbJ6EXWCIqaUD0g8F3iKTi8NJnHaX0s"

func cascadeState(t *testing.T) nodeapi.DesiredState {
	t.Helper()
	st := warpState()
	relay := proto.Template{"type": "vless", "reality-config": map[string]any{"dest": "www.microsoft.com:443", "private-key": relayKey,
		"short-id": []any{"a1b2c3d4"}, "server-names": []any{"www.microsoft.com"}}}
	st.Relay = &nodeapi.Relay{Port: "7443", Config: relay.JSON(), Users: []nodeapi.Slot{{Name: "relay-2", UUID: "0b4ddc4c-7c4f-4a36-9d62-6f1a44b8c4e1"}}}
	exit, _ := json.Marshal(map[string]any{"name": "ignored", "type": "vless", "server": "198.51.100.30", "port": 7443,
		"uuid": "1c5eed5d-8d5a-4b47-8e73-7a2b55c9d5f2", "tls": true, "servername": "www.microsoft.com", "network": "tcp", "udp": true,
		"client-fingerprint": "chrome", "reality-opts": map[string]any{"public-key": "kBPI_JRK3J0k3ShNb_g_6R1H4G4sdvIdC6V4hxGjyAw", "short-id": "a1b2c3d4"}})
	// The relay's own traffic leaves through node 3; in-anytls through node 3 too, which
	// wins over its WARP entry (exits come first).
	st.Exits = []nodeapi.Exit{{Name: nodeapi.ExitName(3), Proxy: exit, Inbounds: []string{nodeapi.RelayListener, "in-anytls", "in-gone"}}}
	return st
}

// A cascade: the relay listener and the exits parse with mihomo, exit rules come after
// the REJECT ones and before WARP's, and only listeners that exist are named.
func TestCascadeConfig(t *testing.T) {
	st := cascadeState(t)
	raw, _, err := buildConfig(st, proto.Cert{CertPath: "/tmp/c.pem", KeyPath: "/tmp/k.pem"}, false, "")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Proxies   []map[string]any `json:"proxies"`
		Rules     []string         `json:"rules"`
		Listeners []map[string]any `json:"listeners"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(cfg.Rules, "\n")
	want := "DST-PORT,25,REJECT\nIN-NAME,mikan~relay,NODE-3\nIN-NAME,in-anytls,NODE-3\nDOMAIN-SUFFIX,openai.com,WARP"
	if !strings.Contains(got, want) || strings.Contains(got, "in-gone") || strings.Contains(got, "IN-NAME,in-anytls,WARP") {
		t.Fatalf("rules:\n%s", got)
	}
	var relay map[string]any
	for _, l := range cfg.Listeners {
		if l["name"] == nodeapi.RelayListener {
			relay = l
		}
	}
	users, _ := relay["users"].([]any)
	if relay == nil || relay["port"] != "7443" || len(users) != 1 {
		t.Fatalf("relay listener: %v", relay)
	}
	names := []string{}
	for _, p := range cfg.Proxies {
		names = append(names, p["name"].(string))
	}
	if strings.Join(names, ",") != "WARP,NODE-3" {
		t.Fatalf("proxies: %v", names)
	}
	parsed, err := executor.ParseWithBytes(raw)
	if err != nil {
		t.Fatalf("mihomo refuses the config: %v", err)
	}
	if _, ok := parsed.Proxies["NODE-3"]; !ok {
		t.Fatal("NODE-3 not parsed")
	}
	if _, ok := parsed.Listeners[nodeapi.RelayListener]; !ok {
		t.Fatal("relay listener not parsed")
	}

	// Only VLESS relays and NODE-<id> names.
	bad := cascadeState(t)
	bad.Exits[0].Proxy = json.RawMessage(`{"type":"socks5","server":"1.2.3.4","port":1}`)
	if _, _, err := buildConfig(bad, proto.Cert{}, false, ""); err == nil {
		t.Fatal("a non-VLESS exit was accepted")
	}
	bad = cascadeState(t)
	bad.Exits[0].Name = "DIRECT"
	if _, _, err := buildConfig(bad, proto.Cert{}, false, ""); err == nil {
		t.Fatal("an exit named like a built-in policy was accepted")
	}
	if outboundKey(cascadeState(t)) == outboundKey(warpState()) {
		t.Fatal("outboundKey must follow the exits")
	}
}

func TestScopedNetworkKeepsCheckedDestination(t *testing.T) {
	st := cascadeState(t)
	st.Slots = []nodeapi.Slot{{Name: "s1", UUID: "a83de115-cfed-416d-9f72-b0b6463a7c0f"}}
	st.Filters = &nodeapi.Filters{Egress: nodeapi.Egress{
		Domains: []string{"deny.example"},
		Scoped:  []nodeapi.ScopedEgress{{Kind: "network", Value: "203.0.113.0/24", Scope: "only", Slots: []string{"s1"}}},
	}}
	raw, _, err := buildConfig(st, proto.Cert{}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := executor.ParseWithBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	match := func(t *testing.T, m *C.Metadata, resolved netip.Addr) string {
		t.Helper()
		for _, r := range cfg.Rules {
			if ok, target := r.Match(m, C.RuleMatchHelper{ResolveIP: func() {
				if !m.Resolved() {
					m.DstIP = resolved
				}
			}}); ok {
				return target
			}
		}
		t.Fatal("no route matched")
		return ""
	}
	addr := netip.MustParseAddrPort("198.51.100.7:443")
	installDestinationChecks(cfg.Rules, false)
	unfiltered := &C.Metadata{NetWork: C.TCP, InName: "in-anytls", Host: "safe.example", DstPort: 443}
	if target := match(t, unfiltered, addr.Addr()); target != "NODE-3" || unfiltered.Host != "safe.example" {
		t.Fatalf("ordinary relay route matched %s with metadata %+v", target, unfiltered)
	}
	installDestinationChecks(cfg.Rules, hasScopedNetworks(st.Filters))
	for _, network := range []C.NetWork{C.TCP, C.UDP} {
		t.Run(network.String(), func(t *testing.T) {
			entry := &C.Metadata{NetWork: network, InName: "in-anytls", InUser: "s1", Host: "safe.example", DstPort: 443}
			if target := match(t, entry.Clone(), mustAddr("203.0.113.7")); target != "REJECT" {
				t.Fatalf("scoped network matched %s", target)
			}
			if target := match(t, entry.Clone(), netip.Addr{}); target != "REJECT" {
				t.Fatalf("unresolved destination matched %s", target)
			}
			if target := match(t, entry, addr.Addr()); target != "NODE-3" || entry.Host != "" || entry.RuleHost() != "safe.example" || entry.AddrPort() != addr {
				t.Fatalf("entry matched %s with metadata %+v", target, entry)
			}
			// The relay has already sniffed the name and Citrus cleared DstIP.
			relay := &C.Metadata{NetWork: network, InName: nodeapi.RelayListener, Host: "safe.example", SniffHost: "safe.example", DstPort: 443}
			if network == C.TCP {
				relay.RawDstAddr = net.TCPAddrFromAddrPort(addr)
			} else {
				relay.RawDstAddr = net.UDPAddrFromAddrPort(addr)
			}
			if target := match(t, relay, mustAddr("203.0.113.7")); target != "NODE-3" || relay.Host != "" || relay.RuleHost() != "safe.example" || relay.AddrPort() != addr {
				t.Fatalf("relay matched %s with metadata %+v", target, relay)
			}
			relay.SniffHost = "deny.example"
			if target := match(t, relay, netip.Addr{}); target != "REJECT" {
				t.Fatalf("global domain block matched %s", target)
			}
			for _, route := range []struct{ host, target string }{{"openai.com", "WARP"}, {"direct.example", "DIRECT"}} {
				m := &C.Metadata{NetWork: network, InUser: "s1", Host: route.host, DstPort: 443}
				if target := match(t, m, addr.Addr()); target != route.target || m.Host != "" || m.RuleHost() != route.host || m.AddrPort() != addr {
					t.Fatalf("wanted %s, matched %s with metadata %+v", route.target, target, m)
				}
			}
		})
	}
}

type udpKeyRecorder struct {
	C.Tunnel
	keys []string
}

func (r *udpKeyRecorder) HandleUDPPacket(p C.UDPPacket, m *C.Metadata) {
	r.keys = append(r.keys, C.NatKey(p, m))
}

type udpSource struct{ C.UDPPacket }

func (udpSource) LocalAddr() net.Addr {
	return net.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.5:1001"))
}

func TestRelayUDPRouteIsBoundToDestination(t *testing.T) {
	inner := &udpKeyRecorder{}
	tun := &Tunnel{inner: inner}
	m := &C.Metadata{InName: nodeapi.RelayListener, Host: "udp.example", DstPort: 443}
	tun.HandleUDPPacket(udpSource{}, m)
	m.DstPort = 8443
	tun.HandleUDPPacket(udpSource{}, m)
	if inner.keys[0] != inner.keys[1] {
		t.Fatal("without scoped filters, destinations on the same UDP socket should share a session")
	}
	inner.keys = nil
	tun.scopedUDP.Store(true)
	m.DstPort = 443
	tun.HandleUDPPacket(udpSource{}, m)
	tun.HandleUDPPacket(udpSource{}, m)
	m.DstPort = 8443
	tun.HandleUDPPacket(udpSource{}, m)
	m.Host = "other.example"
	tun.HandleUDPPacket(udpSource{}, m)
	if k := inner.keys; len(k) != 4 || k[0] != k[1] || k[1] == k[2] || k[2] == k[3] {
		t.Fatalf("UDP session keys = %q", k)
	}
}

// recordingTunnel stands for mihomo's tunnel: it notes what reached it.
type recordingTunnel struct{ tcp atomic.Int32 }

func (r *recordingTunnel) HandleTCPConn(conn net.Conn, _ *C.Metadata) { r.tcp.Add(1); _ = conn.Close() }
func (r *recordingTunnel) HandleUDPPacket(C.UDPPacket, *C.Metadata)   {}
func (r *recordingTunnel) NatTable() C.NatTable                       { return nil }

// The relay's connections reach mihomo without a subscriber; any other unknown user is
// turned away.
func TestRelayBypassesAccounting(t *testing.T) {
	inner := &recordingTunnel{}
	tun := &Tunnel{inner: inner, reg: NewRegistry("e", 0, time.Minute, time.Now, &fakeCore{})}
	a, b := net.Pipe()
	defer b.Close()
	tun.HandleTCPConn(a, &C.Metadata{Type: C.VLESS, InName: nodeapi.RelayListener, InUser: "relay-2", SrcIP: mustAddr("198.51.100.20")})
	c, d := net.Pipe()
	defer d.Close()
	tun.HandleTCPConn(c, &C.Metadata{Type: C.VLESS, InName: "in-vless", InUser: "relay-2", SrcIP: mustAddr("198.51.100.20")})
	if inner.tcp.Load() != 1 {
		t.Fatalf("connections that reached mihomo: %d, want only the relay's", inner.tcp.Load())
	}
}

func mustAddr(s string) netip.Addr { return netip.MustParseAddr(s) }
