Here are the approaches to test your CNI implementation from the CRI (Container Runtime Interface) perspective:

## 1. Manual CNI Plugin Testing (Direct Invocation)

**Create a test network namespace and invoke CNI directly:**

```bash
# Create a network namespace
ip netns add test-ns

# Set required environment variables
export CNI_COMMAND=ADD
export CNI_CONTAINERID=test-container-123
export CNI_NETNS=/var/run/netns/test-ns
export CNI_IFNAME=eth0
export CNI_PATH=/opt/cni/bin

# Create CNI configuration
cat > /tmp/test-cni.conf <<EOF
{
  "cniVersion": "0.4.0",
  "name": "test-network",
  "type": "your-cni-plugin",
  "ipam": {
    "type": "your-ipam-plugin",
    "subnet": "10.244.0.0/16"
  }
}
EOF

# Invoke ADD operation
cat /tmp/test-cni.conf | /opt/cni/bin/your-cni-plugin

# Verify the result
ip netns exec test-ns ip addr show
ip netns exec test-ns ip route show

# Test DEL operation
export CNI_COMMAND=DEL
cat /tmp/test-cni.conf | /opt/cni/bin/your-cni-plugin

# Cleanup
ip netns del test-ns
```

## 2. Use CNI Plugin Test Tools

**cnitool** - Official CNI testing utility:

```bash
# Install cnitool
go install github.com/containernetworking/cni/cnitool@latest

# Add network configuration
cnitool add test-network /var/run/netns/test-ns < /tmp/test-cni.conf

# Check network configuration
cnitool check test-network /var/run/netns/test-ns < /tmp/test-cni.conf

# Delete network configuration
cnitool del test-network /var/run/netns/test-ns < /tmp/test-cni.conf
```

## 3. Test with containerd (Real CRI)

**Configure containerd to use your CNI plugin:**

```bash
# Edit containerd config
sudo vi /etc/containerd/config.toml
```

```toml
[plugins."io.containerd.grpc.v1.cri".cni]
  bin_dir = "/opt/cni/bin"
  conf_dir = "/etc/cni/net.d"
```

**Create pod using crictl:**

```bash
# Install crictl
VERSION="v1.28.0"
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin

# Configure crictl
cat > /etc/crictl.yaml <<EOF
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
EOF

# Create pod config
cat > pod-config.json <<EOF
{
  "metadata": {
    "name": "test-pod",
    "namespace": "default",
    "uid": "test-uid-123"
  },
  "log_directory": "/tmp",
  "linux": {}
}
EOF

# Create container config
cat > container-config.json <<EOF
{
  "metadata": {
    "name": "test-container"
  },
  "image": {
    "image": "busybox:latest"
  },
  "command": [
    "sleep",
    "3600"
  ]
}
EOF

# Pull image
crictl pull busybox:latest

# Create pod sandbox (this triggers CNI ADD)
POD_ID=$(crictl runp pod-config.json)

# Inspect the pod's network
crictl inspectp $POD_ID | jq '.info.runtimeSpec.linux.namespaces'

# Check network namespace
ip netns list
nsenter --net=/var/run/netns/cni-<id> ip addr

# Create container in pod
CONTAINER_ID=$(crictl create $POD_ID container-config.json pod-config.json)
crictl start $CONTAINER_ID

# Exec into container to test networking
crictl exec $CONTAINER_ID ip addr
crictl exec $CONTAINER_ID ping -c 3 8.8.8.8

# Cleanup (triggers CNI DEL)
crictl stopp $POD_ID
crictl rmp $POD_ID
```

## 4. Integration Testing with Kubernetes

**Deploy to test cluster:**

```bash
# Copy plugin to all nodes
for node in $(kubectl get nodes -o name); do
  kubectl debug $node -it --image=alpine -- \
    sh -c "cp /host/path/to/your-plugin /opt/cni/bin/"
done

# Create CNI configuration ConfigMap
kubectl create configmap cni-config \
  --from-file=/etc/cni/net.d/10-your-cni.conflist \
  -n kube-system

# Deploy test pod
cat > test-pod.yaml <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: cni-test-pod
spec:
  containers:
  - name: test
    image: busybox
    command: ["sleep", "3600"]
EOF

kubectl apply -f test-pod.yaml

# Verify pod networking
kubectl exec cni-test-pod -- ip addr
kubectl exec cni-test-pod -- ip route
kubectl exec cni-test-pod -- ping -c 3 8.8.8.8

# Check CNI logs on the node
kubectl get pod cni-test-pod -o wide  # Get node name
ssh node-name "journalctl -u containerd | grep CNI"
```

## 5. Automated Testing Framework

**Create test script:**

```bash
#!/bin/bash

test_cni_add() {
  local container_id=$1
  local netns=$2
  
  ip netns add $netns
  
  result=$(CNI_COMMAND=ADD \
    CNI_CONTAINERID=$container_id \
    CNI_NETNS=/var/run/netns/$netns \
    CNI_IFNAME=eth0 \
    CNI_PATH=/opt/cni/bin \
    /opt/cni/bin/your-plugin < config.json)
  
  # Validate result structure
  echo "$result" | jq -e '.ips[0].address' > /dev/null || return 1
  
  # Verify interface exists
  ip netns exec $netns ip link show eth0 > /dev/null || return 1
  
  # Verify IP is assigned
  ip netns exec $netns ip addr show eth0 | grep -q "inet " || return 1
  
  return 0
}

test_cni_del() {
  local container_id=$1
  local netns=$2
  
  CNI_COMMAND=DEL \
    CNI_CONTAINERID=$container_id \
    CNI_NETNS=/var/run/netns/$netns \
    CNI_IFNAME=eth0 \
    CNI_PATH=/opt/cni/bin \
    /opt/cni/bin/your-plugin < config.json
  
  # Verify cleanup
  ip netns exec $netns ip link show eth0 2>&1 | grep -q "does not exist" || return 1
  ip netns del $netns
  
  return 0
}

# Run tests
test_cni_add "test-123" "test-ns-1" && echo "✓ ADD test passed"
test_cni_del "test-123" "test-ns-1" && echo "✓ DEL test passed"
```

## 6. Debugging and Validation

**Check CNI plugin logs:**

```bash
# Enable CNI debug logging in containerd
sudo vi /etc/containerd/config.toml
# Set log_level = "debug"

# Watch logs
journalctl -u containerd -f | grep CNI

# Or check kubelet logs in Kubernetes
journalctl -u kubelet -f | grep CNI
```

**Validate IPAM state:**

```bash
# Check IPAM allocations
ls -la /var/lib/cni/networks/your-network/

# Verify no IP conflicts
cat /var/lib/cni/networks/your-network/* | sort | uniq -d
```

**Network connectivity tests:**

```bash
# From within test container
ping <gateway-ip>
ping <another-pod-ip>
ping 8.8.8.8
nslookup kubernetes.default.svc.cluster.local
```

The most realistic approach is **#3 (crictl with containerd)** as it exactly mimics how Kubernetes invokes CNI plugins through the CRI, without requiring a full cluster setup.
