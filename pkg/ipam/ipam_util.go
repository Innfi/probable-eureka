package ipam

import "net"

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
