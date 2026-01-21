Here's how CNI operates through the complete pod lifecycle:

## Pod Creation (ADD Operation)

**1. Container Runtime Receives Pod Request**
The kubelet instructs the container runtime (containerd, CRI-O) to create a new pod with specific network requirements.

**2. Runtime Creates Network Namespace**
The runtime creates an isolated network namespace for the pod but doesn't configure any networking yet.

**3. Runtime Reads CNI Configuration**
The runtime reads CNI configuration files from `/etc/cni/net.d/` (e.g., `10-calico.conflist`), determining which plugins to invoke and in what order.

**4. Runtime Invokes CNI Plugin (ADD)**
The runtime executes the CNI plugin binary with:
- Environment variables: `CNI_COMMAND=ADD`, `CNI_CONTAINERID`, `CNI_NETNS`, `CNI_IFNAME`
- Configuration JSON passed via stdin

**5. Main Plugin Invokes IPAM Plugin**
The main CNI plugin delegates to the IPAM plugin by executing it with `ADD` command, passing IPAM-specific configuration.

**6. IPAM Allocates IP Address**
The IPAM plugin:
- Locks the allocation store to prevent race conditions
- Selects an available IP from the configured pool
- Records the allocation (container ID → IP mapping) in persistent storage
- Returns IP, gateway, routes, and DNS information via stdout

**7. Main Plugin Configures Network Interface**
With the allocated IP, the main plugin:
- Creates a veth pair (one end in host namespace, one in container namespace)
- Moves one veth end into the container's network namespace
- Assigns the allocated IP address to the container's interface
- Sets up routing rules
- Configures any necessary NAT or firewall rules

**8. Plugin Returns Result**
The main plugin returns a CNI result object to the runtime via stdout, containing:
- Interface details
- IP addresses assigned
- Routes configured
- DNS settings

**9. Runtime Starts Container**
With networking configured, the runtime proceeds to start the container processes within the configured network namespace.

## Pod Running (CHECK Operation - Optional)

**1. Runtime Performs Health Check**
Periodically, the runtime may invoke CNI plugins with `CNI_COMMAND=CHECK` to verify network configuration is still valid.

**2. Plugin Validates Configuration**
The plugin verifies:
- Network interface still exists
- IP address is still assigned
- Routes are still configured

**3. Plugin Returns Status**
Returns success or error if configuration has drifted.

## Pod Termination (DEL Operation)

**1. Container Runtime Receives Deletion Request**
Kubelet instructs the runtime to terminate the pod.

**2. Runtime Stops Container Processes**
The runtime stops all processes running in the pod.

**3. Runtime Invokes CNI Plugin (DEL)**
The runtime executes the CNI plugin with:
- Environment variables: `CNI_COMMAND=DEL`, `CNI_CONTAINERID`, `CNI_NETNS`
- Same configuration that was used during ADD

**4. Main Plugin Invokes IPAM Plugin (DEL)**
The main plugin delegates to IPAM with `DEL` command.

**5. IPAM Releases IP Address**
The IPAM plugin:
- Locks the allocation store
- Removes the IP allocation record for this container
- Marks the IP as available for future allocation
- Returns success/failure status

**6. Main Plugin Cleans Up Network**
The main plugin:
- Removes the veth pair interfaces
- Cleans up routing rules
- Removes any firewall/NAT rules
- Deletes any host-side network artifacts

**7. Plugin Returns Result**
Returns success to the runtime.

**8. Runtime Destroys Network Namespace**
With networking torn down, the runtime removes the network namespace entirely.

**9. Cleanup Complete**
The pod is fully terminated and all network resources are released.

## Key Considerations

**Idempotency**: ADD can be called multiple times for the same container - plugins must handle this gracefully.

**Cleanup Guarantees**: DEL must succeed even if the container or namespace no longer exists, ensuring no resource leaks.

**Error Handling**: If any step fails during ADD, the runtime typically retries or reports the pod as failed. Failed allocations should be cleaned up.

**State Persistence**: IPAM state must survive runtime restarts to prevent IP conflicts.