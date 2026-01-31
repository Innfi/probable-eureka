#!/bin/bash

# ip netns add ns-innfi

cnitool add test-network /var/run/netns/test-ns < cni.conf
