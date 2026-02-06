package main

import (
	"encoding/json"
	"fmt"
	"test-cni-plugin/pkg/config"
	"test-cni-plugin/pkg/logging"
	"test-cni-plugin/pkg/network"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
)

func cmdAdd(args *skel.CmdArgs) error {
	start := time.Now()

	conf := config.NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); err != nil {
		logging.Logger.Error("cni_command_failed",
			"operation", "add",
			"container_id", args.ContainerID,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to parse config: %v", err)
	}

	hostVeth := fmt.Sprintf("veth%s", args.ContainerID[:8])
	containerVeth := args.IfName

	n := network.New()
	addr, err := n.SetupNetwork(args.Netns, hostVeth, containerVeth, args.ContainerID, conf.IPAM)
	if err != nil {
		logging.Logger.Error("cni_command_failed",
			"operation", "add",
			"container_id", args.ContainerID,
			"netns", args.Netns,
			"ifname", args.IfName,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return err
	}

	logging.Logger.Info("cni_command_completed",
		"operation", "add",
		"container_id", args.ContainerID,
		"netns", args.Netns,
		"ifname", args.IfName,
		"allocated_ip", addr.IPNet.String(),
		"duration_ms", time.Since(start).Milliseconds(),
		"status", "success",
	)

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
	start := time.Now()
	hostVeth := fmt.Sprintf("veth%s", args.ContainerID[:8])

	logging.Logger.Info("cmdDel",
		"hostVeth", hostVeth,
	)

	n := network.New()

	if err := n.TeardownNetwork(hostVeth); err != nil {
		logging.Logger.Error("cni_command_failed",
			"operation", "del",
			"container_id", args.ContainerID,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return err
	}

	logging.Logger.Info("cni_command_completed",
		"operation", "del",
		"container_id", args.ContainerID,
		"duration_ms", time.Since(start).Milliseconds(),
		"status", "success",
	)
	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	start := time.Now()

	conf := config.NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	if conf.PrevResult == nil {
		return fmt.Errorf("missing prevResult from runtime")
	}

	prevResult, err := current.GetResult(conf.PrevResult)
	if err != nil {
		return fmt.Errorf("failed to parse prevResult: %v", err)
	}

	hostVeth := fmt.Sprintf("veth%s", args.ContainerID[:8])
	n := network.New()

	if err := n.CheckNetwork(args.Netns, hostVeth, args.IfName, prevResult.IPs); err != nil {
		logging.Logger.Error("cni_command_failed",
			"operation", "check",
			"container_id", args.ContainerID,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return err
	}

	logging.Logger.Info("cni_command_completed",
		"operation", "check",
		"container_id", args.ContainerID,
		"duration_ms", time.Since(start).Milliseconds(),
		"status", "success",
	)

	return nil
}

func cmdStatus(args *skel.CmdArgs) error {
	conf := config.NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	n := network.New()
	if err := n.CheckPluginStatus(conf.IPAM); err != nil {
		logging.Logger.Error("cni_command_failed",
			"operation", "status",
			"error", err.Error(),
		)
		return err
	}

	logging.Logger.Info("cni_command_completed",
		"operation", "status",
		"status", "ready",
	)

	return nil
}

func main() {
	if err := logging.Init(""); err != nil {
		logging.InitStderr()
	}

	skel.PluginMainFuncs(skel.CNIFuncs{
		Add:    cmdAdd,
		Del:    cmdDel,
		Check:  cmdCheck,
		Status: cmdStatus,
	}, version.All, "test-cni v1.0.0")
}
