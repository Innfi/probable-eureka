package nswrapper

import (
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
)

type NS interface {
	WithNetNSPath(nsPath string, toRun func(ns.NetNS) error) error

	CurrentNS() (ns.NetNS, error)

	GetNS(nspath string) (ns.NetNS, error)
}

type nsType struct{}

func NewNS() NS {
	return &nsType{}
}

func (*nsType) WithNetNSPath(nsPath string, toRun func(ns.NetNS) error) error {
	return ns.WithNetNSPath(nsPath, toRun)
}

func (*nsType) CurrentNS() (ns.NetNS, error) {
	return ns.GetCurrentNS()
}

func (*nsType) GetNS(nspath string) (ns.NetNS, error) {
	return ns.GetNS(nspath)
}

func TestNS() {
	ns := NewNS()

	currentNS, err := ns.CurrentNS()
	if err != nil {
		fmt.Println("err: ", err)
		return
	}

	fmt.Println("currentNS: ", currentNS.Path())
}
