I'll explain how CNI plugins connect veth pairs to the routing table using low-level networking tools and libraries.

## Core Networking Flow

When a CNI plugin sets up container networking, it follows this sequence:

**1. Create Network Namespace**
```bash
# Create new namespace for the container
ip netns add <container-ns>
```

**2. Create veth Pair**
```bash
# Create veth pair - one end stays in host, other goes to container
ip link add <veth-host> type veth peer name <veth-container>
```

**3. Move Container End to Namespace**
```bash
# Move one end into the container's network namespace
ip link set <veth-container> netns <container-ns>
```

**4. Configure Interfaces**
```bash
# In host namespace
ip link set <veth-host> up

# In container namespace
ip netns exec <container-ns> ip link set <veth-container> up
ip netns exec <container-ns> ip addr add <pod-ip>/32 dev <veth-container>
```

**5. Add Routes**

This is where veth connects to the routing table:

```bash
# Container side - default route via veth
ip netns exec <container-ns> ip route add default dev <veth-container>

# Host side - route to pod IP via veth
ip route add <pod-ip>/32 dev <veth-host>
```

## Using netlink Package in Go

CNI plugins typically use the `vishvananda/netlink` package rather than shelling out to `ip` commands:

```go
import (
    "github.com/vishvananda/netlink"
    "github.com/containernetworking/plugins/pkg/ns"
)

// Create veth pair
veth := &netlink.Veth{
    LinkAttrs: netlink.LinkAttrs{
        Name: "veth-host",
        MTU:  1500,
    },
    PeerName: "veth-container",
}
err := netlink.LinkAdd(veth)

// Get the peer (container end)
peer, _ := netlink.LinkByName("veth-container")

// Move peer to container namespace
netlink.LinkSetNsFd(peer, int(containerNs.Fd()))

// Set link up
netlink.LinkSetUp(veth)

// Inside container namespace
containerNs.Do(func(_ ns.NetNS) error {
    // Get link inside namespace
    link, _ := netlink.LinkByName("veth-container")
    
    // Assign IP address
    addr, _ := netlink.ParseAddr("10.0.0.2/32")
    netlink.AddrAdd(link, addr)
    
    // Set link up
    netlink.LinkSetUp(link)
    
    // Add default route
    route := &netlink.Route{
        LinkIndex: link.Attrs().Index,
        Scope:     netlink.SCOPE_LINK,
        Dst:       nil, // default route
    }
    netlink.RouteAdd(route)
    
    return nil
})

// Add host-side route
hostRoute := &netlink.Route{
    LinkIndex: veth.Attrs().Index,
    Dst:       &net.IPNet{
        IP:   net.ParseIP("10.0.0.2"),
        Mask: net.CIDRMask(32, 32),
    },
    Scope: netlink.SCOPE_LINK,
}
netlink.RouteAdd(hostRoute)
```

## Key Routing Concepts

**Host routing table entry:**
- Creates a `/32` route pointing to the specific pod IP
- Uses the host-side veth as the next-hop interface
- Traffic destined for pod IP gets forwarded to this veth

**Container routing table entry:**
- Default route (`0.0.0.0/0`) uses the container-side veth
- All outbound traffic from pod goes through veth to host
- The `/32` address on the veth creates point-to-point link

## Why This Works

The routing magic happens because:

1. **Point-to-point link**: The `/32` address on container veth creates a direct link without needing an intermediate gateway
2. **Kernel routing**: When host receives packets on veth, kernel routing looks up destination and forwards appropriately
3. **Reverse path**: Return packets use the host's `/32` route to find the correct veth interface
