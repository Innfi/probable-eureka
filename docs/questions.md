## IP Address Allocation

### How to guarantee atomicity when allocating an IP address?

**File-based locking approach:**
- Use `flock()` system call on a lock file before modifying the IP allocation state
- The CNI binary acquires an exclusive lock, reads current state, allocates IP, writes state, then releases lock
- Example from host-local IPAM:
```go
l, err := lock.NewFileLock(lockPath)
l.Lock()
defer l.Unlock()
// perform allocation
```

**Etcd-based approach:**
- Use etcd transactions with Compare-And-Set (CAS) operations
- Leverage etcd's MVCC for optimistic concurrency control
- Calico uses this pattern with resource versioning

### Which persistence storage can the CNI binary access?

**Local filesystem (most common):**
- `/var/lib/cni/networks/<network-name>/` - standard location
- host-local IPAM stores flat files per allocated IP
- Example: `/var/lib/cni/networks/mynet/10.244.0.5` contains container ID

**Etcd (for cluster-wide IPAM):**
- Requires network connectivity and credentials
- Used by Calico, Cilium for distributed state
- Path: `/var/lib/etcd` or remote etcd cluster

**Kubernetes API (via CRDs):**
- CNI can create/read IPPool, IPAMBlock custom resources
- Requires kubeconfig and proper RBAC permissions

### Overall flow in terms of actors

```
1. kubelet receives pod creation request
   ↓
2. kubelet calls CNI plugin (ADD command)
   ↓
3. CNI plugin calls IPAM plugin (separate binary or internal)
   CNI_COMMAND=ADD /opt/cni/bin/host-local < network_config.json
   ↓
4. IPAM plugin:
   - Acquires lock on /var/lib/cni/networks/<network>/.lock
   - Reads /var/lib/cni/networks/<network>/last_reserved_ip
   - Finds next available IP
   - Writes allocation: /var/lib/cni/networks/<network>/<ip> → container_id
   - Returns: {"ips": [{"address": "10.244.0.5/24", "gateway": "10.244.0.1"}]}
   ↓
5. CNI plugin receives IPAM response
   ↓
6. CNI plugin configures network interface with allocated IP
   ↓
7. CNI plugin returns result to kubelet
```

## Address Range Management

### How to find out the CIDR blocks allocated to the node?

**Via CNI configuration:**
```json
{
  "ipam": {
    "type": "host-local",
    "ranges": [
      [{"subnet": "10.244.1.0/24"}]
    ]
  }
}
```
The CNI config is passed to the plugin via stdin.

**From Kubernetes Node object:**
```bash
kubectl get node <node-name> -o jsonpath='{.spec.podCIDR}'
# Returns: 10.244.1.0/24
```

**Calico approach:**
- Queries Calico datastore for IPAMBlock resources assigned to the node
- Each node gets blocks like: `10.244.1.0/26`, `10.244.1.64/26`

**AWS VPC CNI:**
- Uses node's ENI secondary IP ranges
- Queries AWS API for available IPs on attached ENIs

**Dynamic discovery flow:**
```
CNI plugin startup
→ Read node name from downward API or hostname
→ Query K8s API: GET /api/v1/nodes/<name>
→ Parse .spec.podCIDR or .spec.podCIDRs
→ Configure IPAM with these ranges
```

## IP Reuse and Reclamation

### Process of stale entry detection

**Garbage collection approaches:**

**1. Reference-based (on DEL command):**
```
Container deletion
→ kubelet calls CNI DEL
→ IPAM receives container_id/netns
→ Removes /var/lib/cni/networks/<network>/<ip>
→ IP returns to available pool
```

**2. Periodic reconciliation:**
```go
// Pseudocode for GC process
func garbageCollect() {
    allocatedIPs := readAllAllocations() // from /var/lib/cni/networks/
    
    for ip, containerID := range allocatedIPs {
        // Check if container exists
        if !containerExists(containerID) {
            // Check if network namespace exists
            if !netnsExists(containerID) {
                releaseIP(ip)
                deleteAllocationFile(ip)
            }
        }
    }
}
```

**3. Kubernetes-based reconciliation (Calico/Cilium):**
```
Periodic reconciliation loop:
→ List all Pod objects in K8s API
→ List all IP allocations in datastore
→ Compare sets
→ Release IPs not associated with running pods
→ Handle pods in terminating state with grace period
```

**Stale entry indicators:**
- Container ID doesn't exist in container runtime
- Network namespace path `/var/run/netns/<id>` doesn't exist
- Pod object deleted from Kubernetes API
- Last allocation timestamp exceeds timeout threshold

**Calico's approach:**
```
1. Watch Kubernetes pod deletions
2. Wait for pod termination confirmation
3. Clean IPAM block entries
4. Compact blocks to recover fragmented IPs
```

## Gateway Configuration

### Required data to configure the gateway

**Minimum required:**
```json
{
  "gateway": "10.244.1.1",      // Gateway IP
  "subnet": "10.244.1.0/24",    // Network CIDR
  "rangeStart": "10.244.1.10",  // Optional: first allocatable IP
  "rangeEnd": "10.244.1.254"    // Optional: last allocatable IP
}
```

**Additional context needed:**
- **Interface name**: Which host interface acts as gateway (e.g., `cni0` bridge)
- **Routes**: What networks are reachable via this gateway
- **MTU**: Maximum transmission unit for the network
- **MAC address**: Gateway's layer-2 address (for ARP)

**Example from Calico:**
```yaml
apiVersion: projectcalico.org/v3
kind: IPPool
metadata:
  name: default-pool
spec:
  cidr: 10.244.0.0/16
  natOutgoing: true
  nodeSelector: all()
  # Gateway is implicitly the first IP in each node's block
```

### Overall sequence

**Bridge-based CNI (e.g., bridge, ptp plugins):**

```
1. IPAM allocation returns gateway info
   ← {"address": "10.244.1.5/24", "gateway": "10.244.1.1"}

2. CNI plugin creates/finds bridge interface
   ip link add cni0 type bridge
   ip link set cni0 up

3. Assign gateway IP to bridge (if not already assigned)
   ip addr add 10.244.1.1/24 dev cni0

4. Create veth pair
   ip link add veth0 type veth peer name vethXXX

5. Move one end to container netns
   ip link set vethXXX netns <container-netns>

6. Configure container interface
   (inside netns):
   ip addr add 10.244.1.5/24 dev vethXXX
   ip link set vethXXX up

7. Set default route pointing to gateway
   ip route add default via 10.244.1.1 dev vethXXX

8. Attach host end to bridge
   ip link set veth0 master cni0
   ip link set veth0 up
```

**Overlay-based CNI (e.g., Cilium, Flannel VXLAN):**

```
1. IPAM allocates IP from node's pod CIDR
   ← {"address": "10.244.1.5/24"}

2. Create/configure overlay interface (e.g., cilium_host)
   ip link add cilium_host type veth peer name cilium_net
   ip addr add 10.244.1.1/32 dev cilium_host

3. Container gets veth with overlay gateway
   Gateway = cilium_host IP (10.244.1.1)

4. Routing handled by overlay mesh
   - No bridge needed
   - Gateway routes to overlay tunnel interface
   - Encapsulation happens at host level
```

## Route Information

### How to ensure the route is valid?

**Validation at allocation time:**

```go
// Pseudocode for route validation
func validateRoute(route Route, gateway string, subnet string) error {
    // 1. Gateway must be in the subnet
    if !subnetContains(subnet, gateway) {
        return fmt.Errorf("gateway %s not in subnet %s", gateway, subnet)
    }
    
    // 2. Destination must be valid CIDR
    _, _, err := net.ParseCIDR(route.Dst)
    if err != nil {
        return fmt.Errorf("invalid destination CIDR: %w", err)
    }
    
    // 3. Gateway must be reachable
    if !isInterfaceUp(route.Interface) {
        return fmt.Errorf("interface %s is down", route.Interface)
    }
    
    return nil
}
```

**Runtime validation methods:**

**1. Connectivity test after setup:**
```bash
# From within container netns
ip netns exec <netns> ping -c 1 -W 1 10.244.1.1  # Test gateway
ip netns exec <netns> ping -c 1 -W 1 8.8.8.8      # Test external route
```

**2. Route existence check:**
```bash
# Verify route is installed
ip netns exec <netns> ip route show | grep "default via 10.244.1.1"
```

**3. ARP resolution verification:**
```bash
# Check gateway MAC is resolvable
ip netns exec <netns> ip neigh show 10.244.1.1
# Should show: 10.244.1.1 dev eth0 lladdr xx:xx:xx:xx:xx:xx REACHABLE
```

**Continuous validation (CNI plugin health checks):**

```go
// Health check logic
func validatePodConnectivity(pod *v1.Pod) error {
    // 1. Check interface exists
    link, err := netlink.LinkByName("eth0")
    if err != nil {
        return err
    }
    
    // 2. Check IP is assigned
    addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
    if len(addrs) == 0 {
        return fmt.Errorf("no IP assigned")
    }
    
    // 3. Check default route
    routes, _ := netlink.RouteList(link, netlink.FAMILY_V4)
    hasDefault := false
    for _, r := range routes {
        if r.Dst == nil { // default route
            hasDefault = true
        }
    }
    
    if !hasDefault {
        return fmt.Errorf("no default route")
    }
    
    return nil
}
```

**Common validation patterns:**

1. **Pre-flight checks**: Validate gateway/routes before applying to container
2. **Post-creation tests**: Ping/curl tests after CNI ADD succeeds
3. **Readiness probes**: Kubernetes liveness/readiness checks validate connectivity
4. **CNI CHECK command**: CNI spec defines CHECK command for validation
   ```bash
   CNI_COMMAND=CHECK /opt/cni/bin/bridge < network_config.json
   ```

**Example CHECK implementation:**
```go
func cmdCheck(args *skel.CmdArgs) error {
    // Parse container netns path
    netns, err := ns.GetNS(args.Netns)
    
    // Validate inside netns
    err = netns.Do(func(_ ns.NetNS) error {
        // Check interface exists
        link, err := netlink.LinkByName(args.IfName)
        
        // Check IP matches expected
        addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
        if !containsExpectedIP(addrs, expectedIP) {
            return fmt.Errorf("IP mismatch")
        }
        
        // Check routes
        routes, _ := netlink.RouteList(link, netlink.FAMILY_V4)
        if !hasValidDefaultRoute(routes, expectedGateway) {
            return fmt.Errorf("invalid routes")
        }
        
        return nil
    })
    
    return err
}
```

These validation mechanisms ensure routes are not only syntactically correct but also functionally working for pod connectivity.
