Based on the CNI IPAM requirements, here are the key actors in the implementation:

## Container Runtime
**Purpose**: Orchestrator of the CNI workflow  
**Responsibility**: Invokes CNI plugins at appropriate lifecycle events (container creation/deletion), passes configuration and environment variables, consumes the returned network configuration results

## CNI Plugin (Main Network Plugin)
**Purpose**: Network interface manager  
**Responsibility**: Creates/deletes network interfaces, configures routes and firewall rules, delegates IP allocation to IPAM plugin, applies the IP configuration to the container's network namespace

## IPAM Plugin
**Purpose**: IP address allocator and tracker  
**Responsibility**: Allocates unique IPs from configured pools, maintains allocation state, releases IPs on deletion, ensures no IP conflicts, provides IP/subnet/gateway information back to the main plugin

## Configuration Store
**Purpose**: Source of network policy and parameters  
**Responsibility**: Holds network configuration (subnet ranges, gateway addresses, DNS settings), provides configuration to plugins at runtime, typically filesystem-based JSON files in `/etc/cni/net.d/`

## Allocation State Store
**Purpose**: Persistent IP allocation tracker  
**Responsibility**: Records which IPs are currently allocated to which containers, survives runtime restarts, prevents duplicate allocations, enables IP reclamation. Could be local filesystem (`/var/lib/cni/networks/`), etcd, or other datastores

## Network Namespace
**Purpose**: Container's isolated network environment  
**Responsibility**: Provides network isolation for the container, hosts the network interfaces and routing tables configured by the CNI plugin

## Result Object
**Purpose**: Communication contract between plugins  
**Responsibility**: Carries structured information (IPs, routes, DNS) from IPAM to main plugin and from plugin back to runtime, ensures consistent data format across the plugin chain

These actors interact in a pipeline: Runtime → Config Store → Main Plugin → IPAM Plugin → Allocation Store, with the Result Object flowing back through the chain.