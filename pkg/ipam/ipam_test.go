package ipam

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/innfi/probable-eureka/pkg/config"
	"github.com/innfi/probable-eureka/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

// mockLink satisfies netlink.Link without any syscalls.
type mockLink struct{}

func (m *mockLink) Attrs() *netlink.LinkAttrs { return &netlink.LinkAttrs{} }
func (m *mockLink) Type() string              { return "mock" }

func noopAddrAdd(_ netlink.Link, _ *netlink.Addr) error { return nil }

func TestMain(m *testing.M) {
	logging.InitStderr()
	os.Exit(m.Run())
}

func makeIPAM(t *testing.T) *IPAM {
	t.Helper()
	cfg := &config.IPAMConfig{
		DataDir: t.TempDir(),
		Ranges: [][]config.Range{
			{
				{
					Subnet:     "10.0.0.0/24",
					RangeStart: "10.0.0.2",
					RangeEnd:   "10.0.0.10",
				},
			},
		},
	}
	i := NewIPAM(cfg)
	i.netlinkAdd = noopAddrAdd
	return &i
}

func writeAllocations(t *testing.T, dir string, allocs []Allocation) {
	t.Helper()
	store := AllocationStore{Allocations: allocs}
	data, err := json.Marshal(store)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, allocationsFile), data, 0644))
}

func TestReleaseAddr_RemovesAllocation(t *testing.T) {
	i := makeIPAM(t)
	writeAllocations(t, i.dataDir(), []Allocation{
		{IP: "10.0.0.2", ContainerID: "container-1"},
	})

	require.NoError(t, i.ReleaseAddr("container-1"))

	store, err := i.loadAllocations()
	require.NoError(t, err)
	require.Empty(t, store.Allocations)
}

func TestReleaseAddr_UnknownContainerIDIsNoop(t *testing.T) {
	i := makeIPAM(t)
	writeAllocations(t, i.dataDir(), []Allocation{
		{IP: "10.0.0.2", ContainerID: "container-1"},
	})

	require.NoError(t, i.ReleaseAddr("container-unknown"))

	store, err := i.loadAllocations()
	require.NoError(t, err)
	require.Len(t, store.Allocations, 1)
	require.Equal(t, "container-1", store.Allocations[0].ContainerID)
}

func TestReleaseAddr_IPReuse(t *testing.T) {
	i := makeIPAM(t)
	writeAllocations(t, i.dataDir(), []Allocation{
		{IP: "10.0.0.2", ContainerID: "container-1"},
	})

	require.NoError(t, i.ReleaseAddr("container-1"))

	// After release, findAvailableIP should return 10.0.0.2 (first in range) again.
	start, end, _, err := i.parseIPRange()
	require.NoError(t, err)
	ip := i.findAvailableIP(start, end)
	require.NotNil(t, ip)
	require.Equal(t, "10.0.0.2", ip.String())
}

func TestBindNewAddr(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "allocates IP in configured range",
			run: func(t *testing.T) {
				i := makeIPAM(t)
				addr, err := i.BindNewAddr(&mockLink{}, "ctr1")
				require.NoError(t, err)
				_, subnet, _ := net.ParseCIDR("10.0.0.0/24")
				assert.True(t, subnet.Contains(addr.IP), "allocated IP %s not in subnet", addr.IP)
				assert.Equal(t, "10.0.0.2", addr.IP.String())
			},
		},
		{
			name: "two allocations get different IPs",
			run: func(t *testing.T) {
				i := makeIPAM(t)
				addr1, err := i.BindNewAddr(&mockLink{}, "ctr1")
				require.NoError(t, err)
				addr2, err := i.BindNewAddr(&mockLink{}, "ctr2")
				require.NoError(t, err)
				assert.NotEqual(t, addr1.IP.String(), addr2.IP.String())
			},
		},
		{
			name: "ReleaseAddr then BindNewAddr reuses the freed IP",
			run: func(t *testing.T) {
				i := makeIPAM(t)
				addr1, err := i.BindNewAddr(&mockLink{}, "ctr1")
				require.NoError(t, err)

				_, err = i.BindNewAddr(&mockLink{}, "ctr2")
				require.NoError(t, err)

				require.NoError(t, i.ReleaseAddr("ctr1"))

				addr3, err := i.BindNewAddr(&mockLink{}, "ctr3")
				require.NoError(t, err)
				assert.Equal(t, addr1.IP.String(), addr3.IP.String(), "ctr3 should reuse ctr1's IP")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.run)
	}
}

func TestReleaseStaleAllocations(t *testing.T) {
	tests := []struct {
		name         string
		initial      []Allocation
		validIDs     map[string]bool
		wantReleased []string
		wantKept     []string
	}{
		{
			name: "removes only invalid container IDs",
			initial: []Allocation{
				{IP: "10.0.0.2", ContainerID: "ctr1"},
				{IP: "10.0.0.3", ContainerID: "ctr2"},
				{IP: "10.0.0.4", ContainerID: "ctr3"},
			},
			validIDs:     map[string]bool{"ctr1": true, "ctr3": true},
			wantReleased: []string{"ctr2"},
			wantKept:     []string{"ctr1", "ctr3"},
		},
		{
			name: "all valid returns nil released",
			initial: []Allocation{
				{IP: "10.0.0.2", ContainerID: "ctr1"},
			},
			validIDs:     map[string]bool{"ctr1": true},
			wantReleased: nil,
			wantKept:     []string{"ctr1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := makeIPAM(t)
			writeAllocations(t, i.dataDir(), tc.initial)

			released, err := i.ReleaseStaleAllocations(tc.validIDs)
			require.NoError(t, err)

			var releasedIDs []string
			for _, a := range released {
				releasedIDs = append(releasedIDs, a.ContainerID)
			}
			assert.Equal(t, tc.wantReleased, releasedIDs)

			store, err := i.loadAllocations()
			require.NoError(t, err)
			var keptIDs []string
			for _, a := range store.Allocations {
				keptIDs = append(keptIDs, a.ContainerID)
			}
			assert.ElementsMatch(t, tc.wantKept, keptIDs)
		})
	}
}

func TestCheckStatus(t *testing.T) {
	tests := []struct {
		name    string
		ranges  [][]config.Range
		wantErr bool
	}{
		{
			name:    "no ranges configured returns error",
			ranges:  nil,
			wantErr: true,
		},
		{
			name: "valid range with available IPs returns nil",
			ranges: [][]config.Range{
				{{Subnet: "10.0.0.0/24", RangeStart: "10.0.0.2", RangeEnd: "10.0.0.10"}},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := NewIPAM(&config.IPAMConfig{
				DataDir: t.TempDir(),
				Ranges:  tc.ranges,
			})
			i.netlinkAdd = noopAddrAdd
			err := i.CheckStatus()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
