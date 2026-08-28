# Architecture

shikA has two planes that stay strictly separated.

- **Control plane** — `shikad`, the Go daemon in this repo. It decides *what should run where*. It never does inference.
- **Data plane** — [prima.cpp](https://github.com/OpenCPIL/prima.cpp). It does the actual distributed inference. shikA treats it as a managed child process.

Keeping these apart means shikA can evolve (better discovery, a nicer dashboard, Tailscale, model management) without touching the C++ inference engine, and can track upstream prima.cpp without merge pain.

## Design principles

1. **No central coordinator.** Every node runs the same planning function over the same membership set and reaches the *same* plan. There is no "server" whose failure takes down the mesh. The head is elected deterministically, not assigned.
2. **Zero external Go dependencies (for now).** The orchestrator uses only the standard library, so it cross-compiles to macOS, Linux, Windows and Android/Termux with `go build` and installs with no toolchain drama. Dependencies get added only when they clearly earn their place.
3. **Safe by default.** Nothing launches inference unless the operator opts in (`-autostart` or the dashboard button). If prima.cpp isn't built yet, the supervisor stays in *dry-run* and simply reports the command it would run.
4. **The engine is not reinvented.** All model math, quantization and layer-splitting is prima.cpp's job.

## Packages

```
cmd/shikad            entrypoint: wires everything together, signal handling
internal/config         JSON config + defaults
internal/node           this device's identity + hardware detection (RAM, cores, GPU, OS)
internal/discovery      UDP-multicast LAN discovery, HTTP seed polling, membership registry
internal/cluster        deterministic plan: head election, ranks, ring, prima.cpp argv
internal/supervisor     launch/stop/watch this node's prima.cpp process
internal/api            control API (JSON) + serves the dashboard
web/                    embedded dashboard (single self-contained HTML page)
```

## How a plan is formed

1. **Detect self.** `node.Detect` reads RAM (`/proc/meminfo` on Linux/Android, `sysctl hw.memsize` on macOS), logical cores, a best-effort GPU hint, and OS (Linux under Termux is reported as `android`, because it changes how prima.cpp launches).
2. **Advertise.** Every `BeaconEvery` interval each node multicasts its `node.Info` as JSON to `239.42.42.42:9977`. Peers on the same subnet receive it and record it with a timestamp.
3. **Seeds (Tailscale / cross-subnet).** Multicast doesn't cross most VPNs or subnets, so `seeds` in the config lists peer *control* addresses (`host:8977`) that are polled over HTTP (`GET /api/self`) and folded into the same membership registry.
4. **Membership.** `discovery.Registry` keeps peers seen within `PeerTimeout`. `Alive()` returns self + live peers.
5. **Plan.** `cluster.Build` sorts members by `Capability()` (RAM-dominant, cores and GPU as tie-breakers; ID as a final deterministic tie-break), elects `rank 0` = head, assigns ranks, and builds the ring so `member[i].next = member[(i+1) % n]`. It then emits the exact prima.cpp argv per node — `llama-server` for the head, `llama-cli` for workers.
6. **Supervise.** `supervisor.Apply` finds this node's member, and if the command changed, restarts prima.cpp in `PrimaDir`. Identical membership → identical plan on every device → each device independently runs the right command with no negotiation.

## Why deterministic election beats a leader protocol (for now)

A full consensus protocol (Raft/gossip-with-leader) is the "correct" answer for a mutable cluster, and may come later. But for a handful of home devices, a **pure function of the sorted member set** is simpler, has no split-brain window, needs no persistent state, and is trivially debuggable: given the same devices, everyone can predict the head. The cost is that membership churn re-plans the whole cluster (prima.cpp can't hot-add a node anyway), which is acceptable at this scale.

## Control API

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/self` | this node's `Info` (also used by seed discovery) |
| GET | `/api/peers` | all live members (self first) |
| GET | `/api/plan` | current deterministic plan + per-node commands |
| GET | `/api/state` | supervisor state + autostart flag + node info |
| POST | `/api/cluster/start` | enable autostart and apply the plan now |
| POST | `/api/cluster/stop` | stop prima.cpp and disable autostart |
| GET | `/` | the device-management dashboard |

## Data plane handoff

The head's plan yields an OpenAI-compatible base URL (`http://<head-ip>:<llm_port>/v1`). [Open WebUI](https://openwebui.com) points at it for chat and voice; any OpenAI-compatible client can too. shikA's job ends at producing and supervising that endpoint.

## Known limitations of the current scaffold

- Membership is last-seen-based, not gossip-reconciled; a peer behind asymmetric routing may not be seen by all nodes. Seeds are the escape hatch.
- The supervisor runs one child and restarts on command change; it does not yet back off, health-check the LLM port, or coordinate start ordering (workers-before-head) across devices — that's Phase 1.
- Security: the control API is unauthenticated and meant for trusted home networks / Tailscale tailnets. Auth is a roadmap item before any untrusted exposure.
