package config

import "github.com/containernetworking/cni/pkg/types"

type NetConf struct {
	types.NetConf
	IPAM *IPAMConfig `json:"ipam"`
}

type IPAMConfig struct {
	Type   string `json:"type"`
	Subnet string `json:"subnet"`
}
