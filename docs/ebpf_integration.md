## eBPF Integration with CNI for IP Allocation

Here's a conceptual overview of how eBPF can be integrated into CNI for IP address allocation and management:

---

### Architecture Overview

```
Pod Creation
    │
    ▼
kubelet → CNI Plugin → IP Allocation Logic
                              │
                    ┌─────────┴─────────┐
                    │   eBPF Programs    │
                    │  (kernel space)    │
                    └─────────┬─────────┘
                              │
                    ┌─────────▼─────────┐
                    │    eBPF Maps       │
                    │ (shared state)     │
                    └───────────────────┘
```

---

### Key Components

#### 1. eBPF Maps for IP State Tracking

eBPF maps act as the shared data store between kernel-space programs and userspace CNI logic:

```c
// BPF map: track allocated IPs per namespace
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32);          // IP address (network byte order)
    __type(value, struct ip_entry);
    __uint(max_entries, 65536);
} ip_allocations SEC(".maps");

struct ip_entry {
    __u64 pod_id;
    __u32 netns_inode;
    __u8  allocated;
    __u8  reserved[3];
};
```

#### 2. CNI Plugin Userspace Side (Go)

The CNI plugin interacts with eBPF maps for allocation:

```go
// Load and interact with eBPF maps at CNI ADD time
func allocateIP(coll *ebpf.Collection, subnet *net.IPNet) (net.IP, error) {
    ipMap := coll.Maps["ip_allocations"]

    // Iterate subnet, find first unallocated IP
    for ip := range iterateSubnet(subnet) {
        key := ipToUint32(ip)
        var entry IPEntry

        err := ipMap.Lookup(key, &entry)
        if errors.Is(err, ebpf.ErrKeyNotExist) {
            // IP is free — allocate it
            entry = IPEntry{PodID: currentPodID(), Allocated: 1}
            if err := ipMap.Put(key, entry); err != nil {
                return nil, err
            }
            return ip, nil
        }
    }
    return nil, errors.New("no IPs available in subnet")
}
```

#### 3. eBPF TC/XDP Hook for Fast-Path Enforcement

Once an IP is allocated, eBPF enforces it in the data plane:

```c
// TC hook: drop traffic from pods using wrong source IP
SEC("tc/egress")
int enforce_ip_policy(struct __sk_buff *skb) {
    struct iphdr *ip = get_ip_header(skb);
    struct ip_entry entry;

    __u32 src = ip->saddr;
    if (bpf_map_lookup_elem(&ip_allocations, &src, &entry) < 0) {
        return TC_ACT_SHOT;  // IP not allocated — drop
    }
    if (!entry.allocated) {
        return TC_ACT_SHOT;
    }
    return TC_ACT_OK;
}
```

---

### IP Allocation Flow

```
CNI ADD called
    │
    ├── Load eBPF objects (maps + programs)
    │
    ├── Scan ip_allocations map for free IP
    │         (atomic CAS via BPF_MAP_TYPE_HASH)
    │
    ├── Write allocation entry to map
    │         { ip → pod_id, netns, timestamp }
    │
    ├── Attach TC/XDP program to veth pair
    │         (enforces source IP in data plane)
    │
    └── Return CNI result with allocated IP


CNI DEL called
    │
    ├── Delete entry from ip_allocations map
    │
    └── Detach eBPF programs from interface
```

---

### Atomicity / Race Condition Handling

Since multiple CNI invocations can race, use a **spinlock map** or `BPF_ATOMIC` ops:

```c
// Use BPF spinlock for atomic allocation
struct ip_entry {
    struct bpf_spin_lock lock;
    __u8  allocated;
    __u64 pod_id;
};

// In BPF program or via userspace with BPF_F_LOCK flag
bpf_spin_lock(&entry->lock);
if (!entry->allocated) {
    entry->allocated = 1;
    entry->pod_id = pod_id;
}
bpf_spin_unlock(&entry->lock);
```

---

### Real-World Reference Implementations

| Project | Approach |
|---|---|
| **Cilium** | Replaces kube-proxy with eBPF; uses BPF maps for endpoint/IP tracking |
| **Calico eBPF** | Uses XDP + TC hooks, BPF maps for policy + routing |
| **Antrea** | OVS + eBPF for pod network, IP tracked in OVS flow tables |

---

### Key Takeaways

1. **eBPF maps** replace etcd/in-memory stores for IP state — kernel-accessible, low-latency
2. **TC/XDP hooks** enforce IP allocation in the fast path (before routing)
3. **Userspace CNI** reads/writes maps at pod lifecycle events (ADD/DEL/CHECK)
4. **Atomicity** requires careful use of spinlocks or compare-and-swap patterns
5. **Cilium's approach** is the most mature reference — worth studying their `pkg/ipam` + BPF map integration
