package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/innfi/probable-eureka/pkg/config"
	"github.com/innfi/probable-eureka/pkg/logging"
	"github.com/innfi/probable-eureka/pkg/metrics"
	"github.com/innfi/probable-eureka/pkg/network"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	vethPrefix           = "veth"
	containerIDPrefixLen = 8
)

var (
	globalMetrics *metrics.Metrics
	metricsOnce   sync.Once
)

func initMetrics(addr string) {
	metricsOnce.Do(func() {
		globalMetrics = metrics.NewMetrics(prometheus.DefaultRegisterer)
		if addr != "" {
			metrics.StartMetricsServer(addr, prometheus.DefaultGatherer)
		}
	})
}

func hostVethName(containerID string) string {
	return vethPrefix + containerID[:containerIDPrefixLen]
}

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

	initMetrics(conf.MetricsAddr)

	hostVeth := hostVethName(args.ContainerID)
	containerVeth := args.IfName

	n := network.New()
	addr, err := n.SetupNetwork(args.Netns, hostVeth, containerVeth, args.ContainerID, conf.Bridge, conf.IPAM)

	status := "success"
	if err != nil {
		status = "error"
	}
	globalMetrics.OpsTotal.WithLabelValues("ADD", status).Inc()
	globalMetrics.OpsDuration.WithLabelValues("ADD", status).Observe(time.Since(start).Seconds())

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

	conf := config.NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	initMetrics(conf.MetricsAddr)

	hostVeth := hostVethName(args.ContainerID)

	logging.Logger.Info("cmdDel",
		"hostVeth", hostVeth,
	)

	n := network.New()
	err := n.TeardownNetwork(hostVeth, conf.Bridge, conf.IPAM, args.ContainerID)

	status := "success"
	if err != nil {
		status = "error"
	}
	globalMetrics.OpsTotal.WithLabelValues("DEL", status).Inc()
	globalMetrics.OpsDuration.WithLabelValues("DEL", status).Observe(time.Since(start).Seconds())

	if err != nil {
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

	initMetrics(conf.MetricsAddr)

	if conf.PrevResult == nil {
		return fmt.Errorf("missing prevResult from runtime")
	}

	prevResult, err := current.GetResult(conf.PrevResult)
	if err != nil {
		return fmt.Errorf("failed to parse prevResult: %v", err)
	}

	hostVeth := hostVethName(args.ContainerID)
	n := network.New()

	err = n.CheckNetwork(args.Netns, hostVeth, args.IfName, prevResult.IPs)

	status := "success"
	if err != nil {
		status = "error"
	}
	globalMetrics.OpsTotal.WithLabelValues("CHECK", status).Inc()
	globalMetrics.OpsDuration.WithLabelValues("CHECK", status).Observe(time.Since(start).Seconds())

	if err != nil {
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

func cmdGC(args *skel.CmdArgs) error {
	start := time.Now()

	conf := config.NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	initMetrics(conf.MetricsAddr)

	validContainerIDs := make(map[string]bool)
	for _, attachment := range conf.ValidAttachments {
		validContainerIDs[attachment.ContainerID] = true
	}

	n := network.New()
	err := n.GarbageCollect(conf.IPAM, validContainerIDs)

	status := "success"
	if err != nil {
		status = "error"
	}
	globalMetrics.OpsTotal.WithLabelValues("GC", status).Inc()
	globalMetrics.OpsDuration.WithLabelValues("GC", status).Observe(time.Since(start).Seconds())

	if err != nil {
		logging.Logger.Error("cni_command_failed",
			"operation", "gc",
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return err
	}

	logging.Logger.Info("cni_command_completed",
		"operation", "gc",
		"valid_attachments", len(conf.ValidAttachments),
		"duration_ms", time.Since(start).Milliseconds(),
		"status", "success",
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
		GC:     cmdGC,
	}, version.All, "probable-eureka v1.0.0")
}
