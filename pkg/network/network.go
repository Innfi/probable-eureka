package network

import (
	"fmt"
	"net"
	"test-cni-plugin/pkg/config"
	"test-cni-plugin/pkg/ipam"
	"test-cni-plugin/pkg/logging"
	"test-cni-plugin/pkg/netlinkwrapper"
	"test-cni-plugin/pkg/nswrapper"

	current "github.com/containernetworking/cni/pkg/types/100"
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

func (n *Network) CheckNetwork(netnsPath, hostVeth, containerVeth string, expectedIPs []*current.IPConfig) error {
	// Verify host veth exists
	if _, err := n.netlink.LinkByName(hostVeth); err != nil {
		return fmt.Errorf("host veth %s not found: %v", hostVeth, err)
	}

	// Verify container veth and IPs inside netns
	netns, err := n.ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns: %v", err)
	}
	defer netns.Close()

	return netns.Do(func(_ ns.NetNS) error {
		link, err := n.netlink.LinkByName(containerVeth)
		if err != nil {
			return fmt.Errorf("container veth %s not found: %v", containerVeth, err)
		}

		// Verify link is up
		if link.Attrs().OperState != netlink.OperUp && (link.Attrs().Flags&net.FlagUp) == 0 {
			return fmt.Errorf("container veth %s is not up", containerVeth)
		}

		// Verify expected IPs are present
		addrs, err := n.netlink.AddrList(link, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("failed to list addresses: %v", err)
		}

		for _, expected := range expectedIPs {
			found := false
			for _, addr := range addrs {
				if addr.IPNet.IP.Equal(expected.Address.IP) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("expected IP %s not found on %s", expected.Address.IP, containerVeth)
			}
		}

		return nil
	})
}

func (n *Network) TeardownNetwork(hostVeth string) error {
	link, err := n.netlink.LinkByName(hostVeth)
	if err != nil {
		return err
	}

	return n.netlink.LinkDel(link)
}
