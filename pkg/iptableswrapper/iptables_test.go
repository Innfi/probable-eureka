package iptableswrapper

import (
	"fmt"
	"os"
	"strings"
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

// mockIPTables is an in-memory IPTablesIface for unit tests.
type mockIPTables struct {
	rules map[string][]string
}

func newMockIPTables() *mockIPTables {
	return &mockIPTables{rules: make(map[string][]string)}
}

func ruleKey(table, chain string) string {
	return table + "/" + chain
}

func (m *mockIPTables) Append(table, chain string, rulespec ...string) error {
	key := ruleKey(table, chain)
	m.rules[key] = append(m.rules[key], strings.Join(rulespec, " "))
	return nil
}

func (m *mockIPTables) AppendUnique(table, chain string, rulespec ...string) error {
	key := ruleKey(table, chain)
	rule := strings.Join(rulespec, " ")
	for _, r := range m.rules[key] {
		if r == rule {
			return nil
		}
	}
	m.rules[key] = append(m.rules[key], rule)
	return nil
}

func (m *mockIPTables) Exists(table, chain string, rulespec ...string) (bool, error) {
	key := ruleKey(table, chain)
	rule := strings.Join(rulespec, " ")
	for _, r := range m.rules[key] {
		if r == rule {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockIPTables) Delete(table, chain string, rulespec ...string) error {
	key := ruleKey(table, chain)
	rule := strings.Join(rulespec, " ")
	existing := m.rules[key]
	for i, r := range existing {
		if r == rule {
			m.rules[key] = append(existing[:i], existing[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockIPTables) List(table, chain string) ([]string, error) {
	return m.rules[ruleKey(table, chain)], nil
}

func (m *mockIPTables) ListChains(table string) ([]string, error) {
	return nil, nil
}

func (m *mockIPTables) ChainExists(table, chain string) (bool, error) {
	return false, nil
}

// Verify mockIPTables satisfies the interface at compile time.
var _ IPTablesIface = (*mockIPTables)(nil)

func TestMockAppendExistsDelete(t *testing.T) {
	ipt := newMockIPTables()

	rulespec := []string{"-s", "10.0.0.0/24", "!", "-o", "cni0", "-j", "MASQUERADE"}

	err := ipt.Append("nat", "POSTROUTING", rulespec...)
	assert.NoError(t, err)

	exists, err := ipt.Exists("nat", "POSTROUTING", rulespec...)
	assert.NoError(t, err)
	assert.True(t, exists)

	err = ipt.Delete("nat", "POSTROUTING", rulespec...)
	assert.NoError(t, err)

	exists, err = ipt.Exists("nat", "POSTROUTING", rulespec...)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestMockAppendUnique(t *testing.T) {
	ipt := newMockIPTables()

	rulespec := []string{"-j", "MASQUERADE"}

	assert.NoError(t, ipt.AppendUnique("nat", "POSTROUTING", rulespec...))
	assert.NoError(t, ipt.AppendUnique("nat", "POSTROUTING", rulespec...))

	rules, err := ipt.List("nat", "POSTROUTING")
	assert.NoError(t, err)
	assert.Len(t, rules, 1)
}
