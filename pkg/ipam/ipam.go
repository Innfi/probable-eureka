package ipam

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"test-cni-plugin/pkg/config"
	"test-cni-plugin/pkg/logging"

	"github.com/vishvananda/netlink"
)

const (
	defaultDataDir  = "/var/lib/cni/networks"
	lockFileName    = ".lock"
	allocationsFile = "allocations.json"
)

type Allocation struct {
	IP          string `json:"ip"`
	ContainerID string `json:"container_id"`
}

type AllocationStore struct {
	Allocations []Allocation `json:"allocations"`
}

type IPAM struct {
	config *config.IPAMConfig
}

func NewIPAM(config *config.IPAMConfig) IPAM {
	return IPAM{config: config}
}

func (ipam *IPAM) BindNewAddr(link netlink.Link, containerID string) (*netlink.Addr, error) {
	unlock, err := ipam.acquireLock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer unlock()

	addr, err := ipam.newAddr()
	if err != nil {
		return nil, err
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		return nil, err
	}

	if err := ipam.saveAllocation(addr.IP.String(), containerID); err != nil {
		return nil, fmt.Errorf("failed to save allocation: %w", err)
	}

	logging.Logger.Info("ip_allocated",
		"allocated_ip", addr.IP.String(),
		"container_id", containerID,
	)

	return addr, nil
}

func (ipam *IPAM) dataDir() string {
	if ipam.config.DataDir != "" {
		return ipam.config.DataDir
	}
	return defaultDataDir
}

func (ipam *IPAM) acquireLock() (func(), error) {
	dir := ipam.dataDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	lockPath := filepath.Join(dir, lockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to acquire file lock: %w", err)
	}

	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}

func (ipam *IPAM) loadAllocations() (*AllocationStore, error) {
	allocPath := filepath.Join(ipam.dataDir(), allocationsFile)

	data, err := os.ReadFile(allocPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AllocationStore{Allocations: []Allocation{}}, nil
		}
		return nil, fmt.Errorf("failed to read allocations file: %w", err)
	}

	var store AllocationStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse allocations file: %w", err)
	}

	return &store, nil
}

func (ipam *IPAM) saveAllocation(ip, containerID string) error {
	store, err := ipam.loadAllocations()
	if err != nil {
		return err
	}

	store.Allocations = append(store.Allocations, Allocation{
		IP:          ip,
		ContainerID: containerID,
	})

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal allocations: %w", err)
	}

	allocPath := filepath.Join(ipam.dataDir(), allocationsFile)
	if err := os.WriteFile(allocPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write allocations file: %w", err)
	}

	return nil
}

func (ipam *IPAM) newAddr() (*netlink.Addr, error) {
	if len(ipam.config.Ranges) == 0 || len(ipam.config.Ranges[0]) == 0 {
		return nil, fmt.Errorf("no IP ranges configured")
	}

	rangeConfig := ipam.config.Ranges[0][0]

	_, subnet, err := net.ParseCIDR(rangeConfig.Subnet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subnet %s: %w", rangeConfig.Subnet, err)
	}

	var startIP, endIP net.IP
	if rangeConfig.RangeStart != "" {
		startIP = net.ParseIP(rangeConfig.RangeStart)
		if startIP == nil {
			return nil, fmt.Errorf("failed to parse rangeStart %s", rangeConfig.RangeStart)
		}
	} else {
		startIP = nextIP(subnet.IP)
	}

	if rangeConfig.RangeEnd != "" {
		endIP = net.ParseIP(rangeConfig.RangeEnd)
		if endIP == nil {
			return nil, fmt.Errorf("failed to parse rangeEnd %s", rangeConfig.RangeEnd)
		}
	} else {
		endIP = lastIP(subnet)
	}

	ip := ipam.findAvailableIP(startIP, endIP)
	if ip == nil {
		return nil, fmt.Errorf("no available IP addresses in range")
	}

	maskSize, _ := subnet.Mask.Size()
	addrStr := fmt.Sprintf("%s/%d", ip.String(), maskSize)

	return netlink.ParseAddr(addrStr)
}

func (ipam *IPAM) findAvailableIP(start, end net.IP) net.IP {
	store, err := ipam.loadAllocations()
	if err != nil {
		return nil
	}

	allocatedIPs := make(map[string]bool)
	for _, alloc := range store.Allocations {
		allocatedIPs[alloc.IP] = true
	}

	for ip := cloneIP(start); !ipGreaterThan(ip, end); ip = nextIP(ip) {
		if !allocatedIPs[ip.String()] {
			return ip
		}
	}

	return nil
}

func ipGreaterThan(a, b net.IP) bool {
	a = a.To16()
	b = b.To16()
	for i := range a {
		if a[i] > b[i] {
			return true
		}
		if a[i] < b[i] {
			return false
		}
	}
	return false
}

func nextIP(ip net.IP) net.IP {
	result := cloneIP(ip)
	for i := len(result) - 1; i >= 0; i-- {
		result[i]++
		if result[i] != 0 {
			break
		}
	}
	return result
}

func lastIP(subnet *net.IPNet) net.IP {
	ip := cloneIP(subnet.IP)
	mask := subnet.Mask
	for i := range ip {
		ip[i] |= ^mask[i]
	}
	ip[len(ip)-1]--
	return ip
}

func cloneIP(ip net.IP) net.IP {
	result := make(net.IP, len(ip))
	copy(result, ip)
	return result
}
