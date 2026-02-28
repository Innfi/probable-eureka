## CNI ADD Command: Key Success Criteria & Isolated Testing Plan

---

### Part 1: Observable Results of a Successful ADD

A successful ADD produces state in three distinct layers:

#### Kernel Networking State (transient, namespace-scoped)
- A veth pair exists: `veth<8-char-container-id>` in the host netns, and `<ifname>` (e.g. `eth0`) in the container netns
- The host-side veth is a member of the configured bridge
- Both ends of the pair are UP
- The container-side veth has the allocated IP/prefix assigned
- The CNI result JSON is written to stdout with: interface name, MAC, and IP

#### IPAM Allocation State (persistent, file-based)
- `allocations.json` contains a new entry: `{ "ip": "<allocated>", "container_id": "<id>" }`
- The `.lock` file exists in the data directory (created on first use)
- The allocated IP is not assigned to any other container_id

#### Process/Runtime Result
- Exit code `0`
- Valid CNI result JSON on stdout conforming to `cniVersion`
- Structured JSON log entry (`cni_command_completed`, `status: success`) written to log file

---

### Part 2: Testing in an Isolated Environment

The core tension here is:

- **Idempotency**: CNI ADD is *not* defined as idempotent — calling ADD twice for the same container is undefined behavior. The plugin should be designed to detect and handle duplicate calls gracefully (the CHECK command handles re-verification).
- **Immutability**: The isolation environment itself should be torn down and rebuilt between test runs; never mutate shared test state.

#### Strategy: Per-Test Network Namespace Isolation

Each test case gets its own transient network namespace and container ID. This enforces immutability at the environment level.

```
Test Case N:
  1. Create fresh netns: ip netns add test-netns-<uuid>
  2. Generate unique container ID: <uuid>
  3. Invoke CNI ADD with that netns path and container ID
  4. Assert all three state layers (kernel, IPAM, stdout)
  5. Tear down: ip netns del test-netns-<uuid>, invoke CNI DEL
  6. Assert cleanup (veth gone, allocation removed)
```

#### Test Matrix

| Scenario | Netns | Container ID | Expected Outcome |
|---|---|---|---|
| First ADD, clean state | fresh | unique | all three state layers satisfied |
| Second ADD, same container ID | same | same | should fail or be a no-op; no duplicate allocation |
| Second ADD, same netns, different ID | same | different | should allocate new IP; detect veth collision |
| ADD after partial failure (veth created, IP alloc fails) | fresh | unique | cleanup rollback, no dangling veth, no allocation record |
| ADD when IP pool is exhausted | fresh | unique | non-zero exit, error on stdout, no state written |

#### Idempotency Specifically

The plugin's current ADD does **not** guard against duplicate calls — if called twice with the same container ID:
- A second veth creation will fail (name collision on host veth)
- A second IP may be allocated to the same container ID (the IPAM `BindNewAddr` does not check for pre-existing allocations)

This means the test plan must verify these failure modes rather than assume correct idempotency. The CHECK command is the correct tool for "did setup succeed?" re-verification, not re-invoking ADD.

#### IPAM Immutability

Because `allocations.json` is file-based and shared, the test harness must:
- Use a unique `DataDir` per test run (not `/var/lib/cni/networks/` globally)
- Start each test with a known-good (or empty) `allocations.json`
- Not share the data directory across parallel test runs

The `IPAMConfig.DataDir` field makes this straightforward — inject a temp directory per test.

#### Validation Commands (per test assertion)

```
# Kernel state
ip link show veth<id>                        → present, UP
ip netns exec <netns> ip link show <ifname>  → present, UP
ip netns exec <netns> ip addr show <ifname>  → expected CIDR assigned
bridge link show                             → host veth is bridge member

# IPAM state
cat <DataDir>/allocations.json               → entry for container_id present, IP matches CNI stdout

# CNI stdout
echo $CNI_RESULT | jq '.ips[0].address'     → matches IPAM record
```

#### Environment Immutability Checklist

- [ ] Unique `DataDir` per test (temp dir, cleaned after)
- [ ] Unique container ID per test (UUID)
- [ ] Fresh network namespace per test (created before, deleted after)
- [ ] Bridge either pre-created and shared (risky) or per-test (safer)
- [ ] No reliance on global `/var/log/cni/` state for assertions
- [ ] Tests run sequentially if sharing any host-namespace resources (bridge, veth names)

---

### Summary

A successful ADD is verified by cross-checking three independent state sources: kernel netlink state, the IPAM JSON file, and the CNI result on stdout — all three must agree on the same IP and interface. Testing is isolated by making `DataDir`, container ID, and network namespace unique per test case. Idempotency is a property of ADD-then-CHECK-then-DEL as a sequence, not ADD-then-ADD; the test plan should explicitly cover the duplicate-ADD failure path rather than assuming it works correctly.
