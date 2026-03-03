# probable-eureka — agent instructions

## What to do each session

1. Read `progress.txt` for gotchas from prior sessions.
2. Read `prd.json` and pick the **first** story whose `"status"` is `"incomplete"`.
3. Implement exactly that one story — nothing more, nothing less.
4. Run the quality gate:
   ```
   make vet && make test && make build
   ```
5. If the gate passes:
   - Commit: `git add -p && git commit -m "<story-id>: <title>"`
   - In `prd.json`, set `"status": "complete"` for that story.
   - Append any new gotchas or patterns to `progress.txt`.
6. If the gate fails: fix the errors, re-run, then commit.
7. Stop. Do not start the next story.

## Project overview

CNI plugin for Kubernetes/containerd implementing ADD, DEL, CHECK, STATUS, GC.

```
main.go                  — CNI entry point (skel.PluginMainFuncs)
pkg/config/config.go     — NetConf, IPAMConfig, Range, Route structs
pkg/network/network.go   — SetupNetwork, TeardownNetwork, CheckNetwork, GarbageCollect
pkg/ipam/ipam.go         — BindNewAddr, ReleaseStaleAllocations, CheckStatus
pkg/ipam/ipam_util.go    — IP arithmetic helpers (nextIP, lastIP, cloneIP, ipGreaterThan)
pkg/logging/logging.go   — structured logger (slog)
pkg/netlinkwrapper/      — NetLink interface wrapping vishvananda/netlink
pkg/nswrapper/           — NS interface wrapping containernetworking/plugins/pkg/ns
pkg/iptableswrapper/     — IPTablesIface wrapping coreos/go-iptables
pkg/vethwrapper/         — (currently unused in network.go; veth created via netlinkwrapper)
pkg/ipwrapper/           — (currently unused)
```

## Key dependencies

| Package | Purpose |
|---|---|
| `github.com/containernetworking/cni` | CNI spec types + skel |
| `github.com/containernetworking/plugins` | ns helper |
| `github.com/vishvananda/netlink` | netlink (links, routes, addrs) |
| `github.com/coreos/go-iptables` | iptables |
| `github.com/stretchr/testify` | test assertions |

## Quality gate

```bash
make vet    # go vet ./...
make test   # go test ./...
make build  # go build -o probable-eureka .
```

All three must exit 0 before committing.

## Coding conventions

- Use the `logging.Logger` (slog) for structured log lines; key pattern: `"event_name", "key", value`.
- Prefer the wrapper interfaces (NetLink, NS) over calling netlink/ns directly so code stays testable.
- IPAM operations that touch allocations.json must hold the file lock (call `ipam.acquireLock()`).
- Table-driven tests with `t.Run`; use `t.TempDir()` for any filesystem state.
