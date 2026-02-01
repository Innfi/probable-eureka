package network

import (
	"fmt"
	"test-cni-plugin/pkg/config"
	"test-cni-plugin/pkg/ipam"
	"test-cni-plugin/pkg/logging"
	"test-cni-plugin/pkg/netlinkwrapper"
	"test-cni-plugin/pkg/nswrapper"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

type Network struct {
	netlink netlinkwrapper.NetLink
	ns      nswrapper.NS
}

func New() *Network {
	return &Network{
		netlink: netlinkwrapper.NewNetlink(),
		ns:      nswrapper.NewNS(),
	}
}

func (n *Network) SetupNetwork(netnsPath, hostVeth, containerVeth, containerID string, ipamConfig *config.IPAMConfig) (*netlink.Addr, error) {
	logging.Logger.Info("SetupNetwork",
		"host_veth", hostVeth,
		"container_veth", containerVeth,
		"container_id", containerID,
	)

	netns, err := n.ns.GetNS(netnsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open netns: %v", err)
	}
	defer netns.Close()

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostVeth},
		PeerName:  containerVeth,
	}
	if err := n.netlink.LinkAdd(veth); err != nil {
		return nil, fmt.Errorf("failed to create veth pair: %v", err)
	}

	cleanupVeth := func() {
		if link, err := n.netlink.LinkByName(hostVeth); err == nil {
			n.netlink.LinkDel(link)
		}
	}

	containerIface, err := n.netlink.LinkByName(containerVeth)
	if err != nil {
		cleanupVeth()
		return nil, err
	}
	if err := n.netlink.LinkSetNsFd(containerIface, int(netns.Fd())); err != nil {
		cleanupVeth()
		return nil, err
	}

	ipam := ipam.NewIPAM(ipamConfig)
	var addr *netlink.Addr

	if err := netns.Do(func(_ ns.NetNS) error {
		link, err := n.netlink.LinkByName(containerVeth)
		if err != nil {
			return err
		}

		// need testing: BindNewAddr has to be called in the goroutine?
		addr, err = ipam.BindNewAddr(link, containerID)
		if err != nil {
			return err
		}

		if err := n.netlink.LinkSetUp(link); err != nil {
			return err
		}

		return nil
	}); err != nil {
		cleanupVeth()
		return nil, err
	}

	return addr, nil
}

func (n *Network) TeardownNetwork(hostVeth string) error {
	link, err := n.netlink.LinkByName(hostVeth)
	if err != nil {
		return err
	}

	return n.netlink.LinkDel(link)
}
