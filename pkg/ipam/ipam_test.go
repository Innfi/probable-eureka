package ipam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/innfi/probable-eureka/pkg/config"
	"github.com/innfi/probable-eureka/pkg/logging"
	"github.com/stretchr/testify/require"
)

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
