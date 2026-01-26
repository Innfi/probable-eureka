package config

import "github.com/containernetworking/cni/pkg/types"

type NetConf struct {
	types.NetConf
	IPAM *IPAMConfig `json:"ipam"`
}

type IPAMConfig struct {
	Type    string    `json:"type"`
	DataDir string    `json:"dataDir"`
	Ranges  [][]Range `json:"ranges"`
	Routes  []Route   `json:"routes"`
}

type Range struct {
	Subnet     string `json:"subnet"`
	RangeStart string `json:"rangeStart,omitempty"`
	RangeEnd   string `json:"rangeEnd,omitempty"`
	Gateway    string `json:"gateway,omitempty"`
}

type Route struct {
	Dst string `json:"dst"`
	Gw  string `json:"gw,omitempty"`
}
