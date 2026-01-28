package main

import (
	"encoding/json"
	"fmt"
	"test-cni-plugin/pkg/config"
	"test-cni-plugin/pkg/network"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
)

func cmdAdd(args *skel.CmdArgs) error {
	conf := config.NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	hostVeth := fmt.Sprintf("veth%s", args.ContainerID[:8])
	containerVeth := args.IfName

	n := network.New()
	addr, err := n.SetupNetwork(args.Netns, hostVeth, containerVeth, args.ContainerID, conf.IPAM)
	if err != nil {
		return err
	}

	// set result
	result := &current.Result{
		CNIVersion: conf.CNIVersion,
		Interfaces: []*current.Interface{
			{Name: containerVeth},
		},
		IPs: []*current.IPConfig{
			{
				Address: *addr.IPNet,
			},
		},
	}

	return types.PrintResult(result, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	hostVeth := fmt.Sprintf("veth%s", args.ContainerID[:8])

	n := network.New()

	return n.TeardownNetwork(hostVeth)
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func main() {
	skel.PluginMainFuncs(skel.CNIFuncs{
		Add:   cmdAdd,
		Del:   cmdDel,
		Check: cmdCheck,
	}, version.All, "test-cni v1.0.0")
}
