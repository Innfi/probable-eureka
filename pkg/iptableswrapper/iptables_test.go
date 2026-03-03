package iptableswrapper

import (
	"fmt"
	"os"
	"testing"

	"github.com/coreos/go-iptables/iptables"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root to access iptables")
	}
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
