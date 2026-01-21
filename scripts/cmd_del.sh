#!/bin/bash

# Cleanup with DEL
export CNI_COMMAND=DEL
echo '{"cniVersion":"1.0.0","name":"mynet","type":"bridge"}' | sudo -E ./test-cni-plugin

# Remove test namespace
# sudo ip netns del test-ns
