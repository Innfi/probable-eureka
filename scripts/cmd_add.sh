#!/bin/bash

# Create a test network namespace if it doesn't exist
if ! ip netns list | grep -q "^test-ns2"; then
  sudo ip netns add test-ns2
fi

# Run ADD
export CNI_COMMAND=ADD
export CNI_CONTAINERID=a0000002-tester
export CNI_NETNS=/var/run/netns/test-ns2
export CNI_IFNAME=eth0
export CNI_PATH=./

# Call the plugin with network config on stdin
cat ./scripts/cni.conf | ./test-cni-plugin
