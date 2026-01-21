CNI plugins with IPAM (IP Address Management) have several key requirements:

## Core CNI Requirements

**Plugin Interface**: Must implement the CNI specification with standard operations (ADD, DEL, CHECK, VERSION) and communicate via stdin/stdout using JSON.

**Execution Model**: Must be executable binaries placed in the CNI bin directory (typically `/opt/cni/bin`), invoked by the container runtime with specific environment variables.

**Network Configuration**: Must read configuration from either stdin or CNI configuration files (typically in `/etc/cni/net.d/`), which specify the plugin chain and parameters.

## IPAM-Specific Requirements

**IP Allocation**: Must allocate unique IP addresses to containers from a defined pool, ensuring no conflicts. The IPAM plugin needs to maintain state about which IPs are in use.

**Address Release**: Must properly release and reclaim IP addresses when containers are deleted, preventing IP exhaustion.

**Subnet Management**: Must understand and respect subnet boundaries, CIDR ranges, and gateway configurations specified in the network configuration.

**Configuration Parameters**: Must accept and process IPAM configuration including:
- IP ranges or subnets to allocate from
- Gateway addresses
- Routes to install
- DNS settings

## Integration Requirements

**Result Format**: Must return standardized result information including assigned IPs, routes, and DNS configuration in the CNI result structure.

**Idempotency**: ADD operations should be idempotent - calling ADD multiple times for the same container should succeed without errors.

**Cleanup**: DEL operations must properly clean up all allocated resources even if the container no longer exists.

**Storage Backend**: IPAM plugins typically need persistent storage (filesystem, etcd, or other datastore) to track IP allocations across runtime restarts.

Common IPAM plugins like `host-local`, `dhcp`, and `static` each implement these requirements differently based on their allocation strategy.