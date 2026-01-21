package iptableswrapper

import (
	"fmt"
	"testing"

	"github.com/coreos/go-iptables/iptables"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	instance, err := NewIPTables(iptables.ProtocolIPv4)
	if err != nil {
		fmt.Println(err)
	}
	assert.Equal(t, err == nil, true)

	chains, err := instance.ListChains("filter")
	assert.Equal(t, err == nil, true)

	for _, elem := range chains {
		fmt.Println(elem)
	}
}
