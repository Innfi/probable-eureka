package netlinkwrapper

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

type NetLink interface {
	LinkByName(name string) (netlink.Link, error)

	LinkSetNsFd(link netlink.Link, fd int) error

	ParseAddr(s string) (*netlink.Addr, error)

	AddrAdd(link netlink.Link, addr *netlink.Addr) error

	AddrDel(link netlink.Link, addr *netlink.Addr) error

	AddrList(link netlink.Link, family int) ([]netlink.Addr, error)

	LinkSetUp(link netlink.Link) error

	LinkDel(link netlink.Link) error

	LinkList() ([]netlink.Link, error)
}

type netLink struct {
}

func NewNetlink() NetLink {
	return &netLink{}
}

func (*netLink) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

func (*netLink) LinkSetNsFd(link netlink.Link, fd int) error {
	return netlink.LinkSetNsFd(link, fd)
}

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

func (*netLink) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

func (*netLink) LinkDel(link netlink.Link) error {
	return netlink.LinkDel(link)
}

func (*netLink) LinkList() ([]netlink.Link, error) {
	return netlink.LinkList()
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
