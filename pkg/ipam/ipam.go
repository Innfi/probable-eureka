package ipam

import (
	"fmt"
	"net"

	"github.com/innfi/probable-eureka/pkg/config"
	"github.com/innfi/probable-eureka/pkg/logging"

	"github.com/vishvananda/netlink"
)

type Allocation struct {
	IP          string `json:"ip"`
	ContainerID string `json:"container_id"`
}

type AllocationStore struct {
	Allocations []Allocation `json:"allocations"`
}

type IPAM struct {
	config     *config.IPAMConfig
	netlinkAdd func(link netlink.Link, addr *netlink.Addr) error
	store      Store
}

func NewIPAM(cfg *config.IPAMConfig) (IPAM, error) {
	s, err := NewStore(cfg)
	if err != nil {
		return IPAM{}, fmt.Errorf("failed to create IPAM store: %w", err)
	}
	return IPAM{config: cfg, netlinkAdd: netlink.AddrAdd, store: s}, nil
}

func (ipam *IPAM) BindNewAddr(link netlink.Link, containerID string) (*netlink.Addr, error) {
	unlock, err := ipam.store.Lock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer unlock()

	addr, err := ipam.newAddr()
	if err != nil {
		return nil, err
	}

	if err := ipam.netlinkAdd(link, addr); err != nil {
		return nil, err
	}

	store, err := ipam.store.Load()
	if err != nil {
		return nil, err
	}
	store.Allocations = append(store.Allocations, Allocation{
		IP:          addr.IP.String(),
		ContainerID: containerID,
	})
	if err := ipam.store.Save(store); err != nil {
		return nil, fmt.Errorf("failed to save allocation: %w", err)
	}

	logging.Logger.Info("ip_allocated",
		"allocated_ip", addr.IP.String(),
		"container_id", containerID,
	)

	return addr, nil
}

func (ipam *IPAM) parseIPRange() (startIP, endIP net.IP, subnet *net.IPNet, err error) {
	if len(ipam.config.Ranges) == 0 || len(ipam.config.Ranges[0]) == 0 {
		return nil, nil, nil, fmt.Errorf("no IP ranges configured")
	}

	rangeConfig := ipam.config.Ranges[0][0]

	_, subnet, err = net.ParseCIDR(rangeConfig.Subnet)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse subnet %s: %w", rangeConfig.Subnet, err)
	}

	if rangeConfig.RangeStart != "" {
		startIP = net.ParseIP(rangeConfig.RangeStart)
		if startIP == nil {
			return nil, nil, nil, fmt.Errorf("failed to parse rangeStart %s", rangeConfig.RangeStart)
		}
	} else {
		startIP = nextIP(subnet.IP)
	}

	if rangeConfig.RangeEnd != "" {
		endIP = net.ParseIP(rangeConfig.RangeEnd)
		if endIP == nil {
			return nil, nil, nil, fmt.Errorf("failed to parse rangeEnd %s", rangeConfig.RangeEnd)
		}
	} else {
		endIP = lastIP(subnet)
	}

	return startIP, endIP, subnet, nil
}

func (ipam *IPAM) newAddr() (*netlink.Addr, error) {
	startIP, endIP, subnet, err := ipam.parseIPRange()
	if err != nil {
		return nil, err
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
	store, err := ipam.store.Load()
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

func (ipam *IPAM) ReleaseAddr(containerID string) error {
	unlock, err := ipam.store.Lock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer unlock()

	store, err := ipam.store.Load()
	if err != nil {
		return err
	}

	var kept []Allocation
	for _, alloc := range store.Allocations {
		if alloc.ContainerID == containerID {
			logging.Logger.Info("ip_released",
				"ip", alloc.IP,
				"container_id", containerID,
			)
		} else {
			kept = append(kept, alloc)
		}
	}

	if kept == nil {
		kept = []Allocation{}
	}

	return ipam.store.Save(&AllocationStore{Allocations: kept})
}

func (ipam *IPAM) ReleaseStaleAllocations(validContainerIDs map[string]bool) ([]Allocation, error) {
	unlock, err := ipam.store.Lock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer unlock()

	store, err := ipam.store.Load()
	if err != nil {
		return nil, err
	}

	var kept []Allocation
	var released []Allocation
	for _, alloc := range store.Allocations {
		if validContainerIDs[alloc.ContainerID] {
			kept = append(kept, alloc)
		} else {
			released = append(released, alloc)
		}
	}

	if len(released) == 0 {
		return nil, nil
	}

	if err := ipam.store.Save(&AllocationStore{Allocations: kept}); err != nil {
		return nil, err
	}

	return released, nil
}

func (ipam *IPAM) CheckStatus() error {
	startIP, endIP, _, err := ipam.parseIPRange()
	if err != nil {
		return err
	}

	if ipam.findAvailableIP(startIP, endIP) == nil {
		return fmt.Errorf("no available IP addresses in configured range")
	}

	return nil
}
