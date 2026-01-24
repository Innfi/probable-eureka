package netlinkwrapper

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

type NetLink interface {
	// Link operations
	LinkByName(name string) (netlink.Link, error)
	LinkByIndex(index int) (netlink.Link, error)
	LinkAdd(link netlink.Link) error
	LinkDel(link netlink.Link) error
	LinkList() ([]netlink.Link, error)
	LinkSetUp(link netlink.Link) error
	LinkSetDown(link netlink.Link) error
	LinkSetMaster(link netlink.Link, master netlink.Link) error
	LinkSetNoMaster(link netlink.Link) error
	LinkSetNsFd(link netlink.Link, fd int) error
	LinkSetNsPid(link netlink.Link, nspid int) error
	LinkSetName(link netlink.Link, name string) error
	LinkSetMTU(link netlink.Link, mtu int) error
	LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error

	// Address operations
	ParseAddr(s string) (*netlink.Addr, error)
	AddrAdd(link netlink.Link, addr *netlink.Addr) error
	AddrDel(link netlink.Link, addr *netlink.Addr) error
	AddrList(link netlink.Link, family int) ([]netlink.Addr, error)
	AddrReplace(link netlink.Link, addr *netlink.Addr) error

	// Route operations
	RouteAdd(route *netlink.Route) error
	RouteDel(route *netlink.Route) error
	RouteReplace(route *netlink.Route) error
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	RouteGet(destination net.IP) ([]netlink.Route, error)

	// Neighbor (ARP) operations
	NeighAdd(neigh *netlink.Neigh) error
	NeighDel(neigh *netlink.Neigh) error
	NeighList(linkIndex, family int) ([]netlink.Neigh, error)
	NeighSet(neigh *netlink.Neigh) error

	// Rule operations
	RuleAdd(rule *netlink.Rule) error
	RuleDel(rule *netlink.Rule) error
	RuleList(family int) ([]netlink.Rule, error)
}

type netLink struct {
}

func NewNetlink() NetLink {
	return &netLink{}
}

// Link operations

func (*netLink) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

func (*netLink) LinkByIndex(index int) (netlink.Link, error) {
	return netlink.LinkByIndex(index)
}

func (*netLink) LinkAdd(link netlink.Link) error {
	return netlink.LinkAdd(link)
}

func (*netLink) LinkDel(link netlink.Link) error {
	return netlink.LinkDel(link)
}

func (*netLink) LinkList() ([]netlink.Link, error) {
	return netlink.LinkList()
}

func (*netLink) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

func (*netLink) LinkSetDown(link netlink.Link) error {
	return netlink.LinkSetDown(link)
}

func (*netLink) LinkSetMaster(link netlink.Link, master netlink.Link) error {
	return netlink.LinkSetMaster(link, master)
}

func (*netLink) LinkSetNoMaster(link netlink.Link) error {
	return netlink.LinkSetNoMaster(link)
}

func (*netLink) LinkSetNsFd(link netlink.Link, fd int) error {
	return netlink.LinkSetNsFd(link, fd)
}

func (*netLink) LinkSetNsPid(link netlink.Link, nspid int) error {
	return netlink.LinkSetNsPid(link, nspid)
}

func (*netLink) LinkSetName(link netlink.Link, name string) error {
	return netlink.LinkSetName(link, name)
}

func (*netLink) LinkSetMTU(link netlink.Link, mtu int) error {
	return netlink.LinkSetMTU(link, mtu)
}

func (*netLink) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetHardwareAddr(link, hwaddr)
}

// Address operations

func (*netLink) ParseAddr(s string) (*netlink.Addr, error) {
	return netlink.ParseAddr(s)
}

func (*netLink) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrAdd(link, addr)
}

func (*netLink) AddrDel(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrDel(link, addr)
}

func (*netLink) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	return netlink.AddrList(link, family)
}

func (*netLink) AddrReplace(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrReplace(link, addr)
}

// Route operations

func (*netLink) RouteAdd(route *netlink.Route) error {
	return netlink.RouteAdd(route)
}

func (*netLink) RouteDel(route *netlink.Route) error {
	return netlink.RouteDel(route)
}

func (*netLink) RouteReplace(route *netlink.Route) error {
	return netlink.RouteReplace(route)
}

func (*netLink) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	return netlink.RouteList(link, family)
}

func (*netLink) RouteGet(destination net.IP) ([]netlink.Route, error) {
	return netlink.RouteGet(destination)
}

// Neighbor (ARP) operations

func (*netLink) NeighAdd(neigh *netlink.Neigh) error {
	return netlink.NeighAdd(neigh)
}

func (*netLink) NeighDel(neigh *netlink.Neigh) error {
	return netlink.NeighDel(neigh)
}

func (*netLink) NeighList(linkIndex, family int) ([]netlink.Neigh, error) {
	return netlink.NeighList(linkIndex, family)
}

func (*netLink) NeighSet(neigh *netlink.Neigh) error {
	return netlink.NeighSet(neigh)
}

// Rule operations

func (*netLink) RuleAdd(rule *netlink.Rule) error {
	return netlink.RuleAdd(rule)
}

func (*netLink) RuleDel(rule *netlink.Rule) error {
	return netlink.RuleDel(rule)
}

func (*netLink) RuleList(family int) ([]netlink.Rule, error) {
	return netlink.RuleList(family)
}

func TestNetLink() {
	netlink := NewNetlink()

	links, err := netlink.LinkList()
	if err != nil {
		fmt.Println("err: ", err)
		return
	}

	for _, link := range links {
		fmt.Printf("link: %v\n", link)
	}
}
