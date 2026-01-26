package ipam

import (
	"fmt"
	"net"
	"test-cni-plugin/pkg/config"

	"github.com/vishvananda/netlink"
)

type IPAM struct {
	config *config.IPAMConfig
}

func NewIPAM(config *config.IPAMConfig) IPAM {
	return IPAM{config: config}
}

func (ipam *IPAM) BindNewAddr(link netlink.Link) (*netlink.Addr, error) {
	addr, err := ipam.newAddr()
	if err != nil {
		return nil, err
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		return nil, err
	}

	return addr, nil
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
	// TODO: implement persistence to track allocated IPs using ipam.config.DataDir
	return cloneIP(start)
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
