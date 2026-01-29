#!/bin/bash

# Cleanup with DEL
export CNI_COMMAND=DEL
export CNI_NETNS=/var/run/netns/test-ns
export CNI_IFNAME=eth0
export CNI_PATH=./
cat ./scripts/cni.conf | ./test-cni-plugin

# Remove test namespace
sudo ip netns del test-ns
