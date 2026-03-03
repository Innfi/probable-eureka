package network

import (
	"errors"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/innfi/probable-eureka/pkg/config"
	"github.com/innfi/probable-eureka/pkg/ipam"
	"github.com/innfi/probable-eureka/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

func TestMain(m *testing.M) {
	logging.InitStderr()
	os.Exit(m.Run())
}

// ---- mock implementations ----

// mockLink is a fake netlink.Link with no kernel backing.
type mockLink struct {
	attrs netlink.LinkAttrs
}

func (m *mockLink) Attrs() *netlink.LinkAttrs { return &m.attrs }
func (m *mockLink) Type() string              { return "mock" }

// mockNetLink is an in-memory netlinkwrapper.NetLink.
// It stores links in a map, records LinkDel calls, and can be told to fail LinkSetMaster.
type mockNetLink struct {
	links        map[string]*mockLink
	linkDelCalls []string
	setMasterErr error
	nextIdx      int
}

func newMockNetLink() *mockNetLink {
	return &mockNetLink{links: make(map[string]*mockLink)}
}

func (m *mockNetLink) bumpIdx() int {
	m.nextIdx++
	return m.nextIdx
}

func (m *mockNetLink) LinkByName(name string) (netlink.Link, error) {
	if l, ok := m.links[name]; ok {
		return l, nil
	}
	return nil, fmt.Errorf("link not found: %s", name)
}

func (m *mockNetLink) LinkByIndex(_ int) (netlink.Link, error) { return nil, nil }

// LinkAdd adds the link (and its veth peer, if applicable) to the in-memory store.
func (m *mockNetLink) LinkAdd(link netlink.Link) error {
	name := link.Attrs().Name
	m.links[name] = &mockLink{attrs: netlink.LinkAttrs{Name: name, Index: m.bumpIdx()}}
	if veth, ok := link.(*netlink.Veth); ok && veth.PeerName != "" {
		peer := veth.PeerName
		m.links[peer] = &mockLink{attrs: netlink.LinkAttrs{Name: peer, Index: m.bumpIdx()}}
	}
	return nil
}

func (m *mockNetLink) LinkDel(link netlink.Link) error {
	name := link.Attrs().Name
	m.linkDelCalls = append(m.linkDelCalls, name)
	delete(m.links, name)
	return nil
}

func (m *mockNetLink) LinkList() ([]netlink.Link, error) {
	result := make([]netlink.Link, 0, len(m.links))
	for _, l := range m.links {
		result = append(result, l)
	}
	return result, nil
}

func (m *mockNetLink) LinkSetUp(_ netlink.Link) error                            { return nil }
func (m *mockNetLink) LinkSetDown(_ netlink.Link) error                          { return nil }
func (m *mockNetLink) LinkSetMaster(_, _ netlink.Link) error                     { return m.setMasterErr }
func (m *mockNetLink) LinkSetNoMaster(_ netlink.Link) error                      { return nil }
func (m *mockNetLink) LinkSetNsFd(_ netlink.Link, _ int) error                   { return nil }
func (m *mockNetLink) LinkSetNsPid(_ netlink.Link, _ int) error                  { return nil }
func (m *mockNetLink) LinkSetName(_ netlink.Link, _ string) error                { return nil }
func (m *mockNetLink) LinkSetMTU(_ netlink.Link, _ int) error                    { return nil }
func (m *mockNetLink) LinkSetHardwareAddr(_ netlink.Link, _ net.HardwareAddr) error { return nil }

func (m *mockNetLink) ParseAddr(s string) (*netlink.Addr, error) { return netlink.ParseAddr(s) }
func (m *mockNetLink) AddrAdd(_ netlink.Link, _ *netlink.Addr) error             { return nil }
func (m *mockNetLink) AddrDel(_ netlink.Link, _ *netlink.Addr) error             { return nil }
func (m *mockNetLink) AddrList(_ netlink.Link, _ int) ([]netlink.Addr, error)    { return nil, nil }
func (m *mockNetLink) AddrReplace(_ netlink.Link, _ *netlink.Addr) error         { return nil }

func (m *mockNetLink) RouteAdd(_ *netlink.Route) error                              { return nil }
func (m *mockNetLink) RouteDel(_ *netlink.Route) error                              { return nil }
func (m *mockNetLink) RouteReplace(_ *netlink.Route) error                          { return nil }
func (m *mockNetLink) RouteList(_ netlink.Link, _ int) ([]netlink.Route, error)     { return nil, nil }
func (m *mockNetLink) RouteGet(_ net.IP) ([]netlink.Route, error)                   { return nil, nil }

func (m *mockNetLink) NeighAdd(_ *netlink.Neigh) error                              { return nil }
func (m *mockNetLink) NeighDel(_ *netlink.Neigh) error                              { return nil }
func (m *mockNetLink) NeighList(_, _ int) ([]netlink.Neigh, error)                  { return nil, nil }
func (m *mockNetLink) NeighSet(_ *netlink.Neigh) error                              { return nil }

func (m *mockNetLink) RuleAdd(_ *netlink.Rule) error                                { return nil }
func (m *mockNetLink) RuleDel(_ *netlink.Rule) error                                { return nil }
func (m *mockNetLink) RuleList(_ int) ([]netlink.Rule, error)                       { return nil, nil }

// mockNetNS runs Do callbacks in the same goroutine without entering a real netns.
type mockNetNS struct{}

func (m *mockNetNS) Do(toRun func(ns.NetNS) error) error { return toRun(m) }
func (m *mockNetNS) Set() error                          { return nil }
func (m *mockNetNS) Path() string                        { return "/proc/1/ns/net" }
func (m *mockNetNS) Fd() uintptr                         { return 0 }
func (m *mockNetNS) Close() error                        { return nil }

// mockNSWrapper wraps a mockNetNS as a nswrapper.NS.
type mockNSWrapper struct{ netns *mockNetNS }

func (m *mockNSWrapper) WithNetNSPath(_ string, toRun func(ns.NetNS) error) error {
	return toRun(m.netns)
}
func (m *mockNSWrapper) CurrentNS() (ns.NetNS, error)    { return m.netns, nil }
func (m *mockNSWrapper) GetNS(_ string) (ns.NetNS, error) { return m.netns, nil }

// mockIPAM is a preset ipamIface for tests.
type mockIPAM struct {
	bindResult *netlink.Addr
	bindErr    error
	releaseErr error
}

func (m *mockIPAM) BindNewAddr(_ netlink.Link, _ string) (*netlink.Addr, error) {
	return m.bindResult, m.bindErr
}
func (m *mockIPAM) ReleaseAddr(_ string) error { return m.releaseErr }
func (m *mockIPAM) ReleaseStaleAllocations(_ map[string]bool) ([]ipam.Allocation, error) {
	return nil, nil
}
func (m *mockIPAM) CheckStatus() error { return nil }

// Compile-time interface checks.
var _ ipamIface = (*mockIPAM)(nil)

// ---- helpers ----

func makeIPAMConfig(t *testing.T) *config.IPAMConfig {
	t.Helper()
	return &config.IPAMConfig{
		DataDir: t.TempDir(),
		Ranges: [][]config.Range{
			{{
				Subnet:     "10.0.0.0/24",
				RangeStart: "10.0.0.2",
				RangeEnd:   "10.0.0.254",
				Gateway:    "10.0.0.1",
			}},
		},
	}
}

func newTestNetwork(nl *mockNetLink, nsw *mockNSWrapper, makeIPAM func(*config.IPAMConfig) ipamIface) *Network {
	return &Network{
		netlink: nl,
		ns:      nsw,
		ipt:     nil, // no iptables calls; avoids root requirement
		newIPAM: makeIPAM,
	}
}

// ---- tests ----

func TestSetupNetwork_HappyPath(t *testing.T) {
	nl := newMockNetLink()
	nsw := &mockNSWrapper{netns: &mockNetNS{}}

	wantAddr, err := netlink.ParseAddr("10.0.0.2/24")
	require.NoError(t, err)
	mipm := &mockIPAM{bindResult: wantAddr}

	n := newTestNetwork(nl, nsw, func(_ *config.IPAMConfig) ipamIface { return mipm })

	addr, err := n.SetupNetwork("/proc/1/ns/net", "veth-host", "eth0", "ctr1", "cni0", makeIPAMConfig(t))

	require.NoError(t, err)
	require.NotNil(t, addr)
	assert.Equal(t, "10.0.0.2", addr.IP.String())
}

func TestSetupNetwork_RollsBackVethOnBridgeAttachFail(t *testing.T) {
	nl := newMockNetLink()
	nl.setMasterErr = errors.New("attach failed")
	nsw := &mockNSWrapper{netns: &mockNetNS{}}

	wantAddr, _ := netlink.ParseAddr("10.0.0.2/24")
	mipm := &mockIPAM{bindResult: wantAddr}

	n := newTestNetwork(nl, nsw, func(_ *config.IPAMConfig) ipamIface { return mipm })

	addr, err := n.SetupNetwork("/proc/1/ns/net", "veth-host", "eth0", "ctr1", "cni0", makeIPAMConfig(t))

	require.Error(t, err)
	assert.Nil(t, addr)
	assert.Contains(t, nl.linkDelCalls, "veth-host", "host veth should be deleted on rollback")
}

func TestTeardownNetwork_CallsLinkDel(t *testing.T) {
	nl := newMockNetLink()
	nl.links["veth-host"] = &mockLink{attrs: netlink.LinkAttrs{Name: "veth-host", Index: 1}}
	nsw := &mockNSWrapper{netns: &mockNetNS{}}
	mipm := &mockIPAM{}

	n := newTestNetwork(nl, nsw, func(_ *config.IPAMConfig) ipamIface { return mipm })

	err := n.TeardownNetwork("veth-host", "", makeIPAMConfig(t), "ctr1")

	require.NoError(t, err)
	assert.Contains(t, nl.linkDelCalls, "veth-host")
}

func TestCheckNetwork_ErrorWhenHostVethMissing(t *testing.T) {
	nl := newMockNetLink() // empty — "veth-host" does not exist
	nsw := &mockNSWrapper{netns: &mockNetNS{}}

	n := newTestNetwork(nl, nsw, nil) // IPAM not used by CheckNetwork

	err := n.CheckNetwork("/proc/1/ns/net", "veth-host", "eth0", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "veth-host")
}
