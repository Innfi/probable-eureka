package vethwrapper

import (
	"net"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
)

type Veth interface {
	Setup(contVethName string, mtu int, contVethMac string, hostNS ns.NetNS) (net.Interface, net.Interface, error)
}

type veth struct{}

// why not just NewVeth?
func NewVeth() Veth {
	return &veth{}
}

func (*veth) Setup(contVethName string, mtu int, contVethMac string, hostNS ns.NetNS) (net.Interface, net.Interface, error) {
	return ip.SetupVeth(contVethName, mtu, contVethMac, hostNS)
}
