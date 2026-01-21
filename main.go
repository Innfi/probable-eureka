package main

import (
	"encoding/json"
	"fmt"
	"net"
	"test-cni-plugin/pkg/config"
	"test-cni-plugin/pkg/ipam"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func cmdAdd(args *skel.CmdArgs) error {
	conf := config.NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	fmt.Println("conf:", conf)
	fmt.Println("ipam: ", conf.IPAM)

	hostVeth := fmt.Sprintf("veth%s", args.ContainerID[:8])
	containerVeth := args.IfName

	fmt.Println("hostVeth: ", hostVeth)

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns: %v", err)
	}
	defer netns.Close()

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostVeth},
		PeerName:  containerVeth,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("failed to create veth pair: %v", err)
	}

	containerIface, err := netlink.LinkByName(containerVeth)
	if err != nil {
		return err
	}
	if err := netlink.LinkSetNsFd(containerIface, int(netns.Fd())); err != nil {
		return err
	}

	ipamInstance := ipam.NewIPAM(conf.IPAM)
	var addr *netlink.Addr

	if err := netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(containerVeth)
		if err != nil {
			return err
		}

		addr, err = ipamInstance.BindNewAddr(link)
		if err != nil {
			return err
		}

		if err := netlink.LinkSetUp(link); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	if addr == nil {
		return fmt.Errorf("addr never made it out of goroutine")
	}

	// set result
	result := &current.Result{
		CNIVersion: conf.CNIVersion,
		Interfaces: []*current.Interface{
			{Name: containerVeth},
		},
		IPs: []*current.IPConfig{
			{
				Address: net.IPNet{
					IP:   addr.IP,
					Mask: net.CIDRMask(24, 32),
				},
			},
		},
	}

	return types.PrintResult(result, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	hostVeth := fmt.Sprintf("veth%s", args.ContainerID[:8])

	link, err := netlink.LinkByName(hostVeth)
	if err != nil {
		return err
	}

	return netlink.LinkDel(link)
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func main() {
	fmt.Println("main")

	// iptableswrapper.TestRunIptables()

	// nswrapper.TestNS()

	// netlinkwrapper.TestNetLink()
	skel.PluginMainFuncs(skel.CNIFuncs{
		Add:   cmdAdd,
		Del:   cmdDel,
		Check: cmdCheck,
	}, version.All, "test-cni v1.0.0")
}
