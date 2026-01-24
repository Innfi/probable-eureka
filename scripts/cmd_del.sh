#!/bin/bash

# Cleanup with DEL
export CNI_COMMAND=DEL
cat ./scripts/cni.conf | ./test-cni-plugin

# Remove test namespace
sudo ip netns del test-ns
