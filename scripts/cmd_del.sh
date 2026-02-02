#!/bin/bash

# Cleanup with DEL
export CNI_COMMAND=DEL
export CNI_CONTAINERID=a0000002-tester
export CNI_NETNS=/var/run/netns/test-ns2
export CNI_IFNAME=eth1
export CNI_PATH=./
cat ./scripts/cni.conf | ./test-cni-plugin

# Remove test namespace
# sudo ip netns del test-ns
