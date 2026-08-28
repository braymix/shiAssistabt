# Roadmap

prima-mesh is built in phases. Each phase is a usable milestone, not a big-bang.

## Phase 0 — Orchestrator skeleton ✅ (v0.1, this repo)

- [x] Node identity + hardware detection (RAM, cores, GPU, OS incl. Termux→android)
- [x] Zero-config LAN discovery (UDP multicast beacons)
- [x] Seed-based discovery for cross-subnet / Tailscale peers
- [x] Membership registry with liveness timeout
- [x] Deterministic cluster planner (head election, ranks, ring, prima.cpp argv)
- [x] Supervisor with safe dry-run when prima.cpp isn't built
- [x] Control API + embedded device-management dashboard
- [x] Builds and runs as a single dependency-free binary

## Phase 1 — Real distributed launch 🔜

- [ ] Launch & supervise real prima.cpp across 2+ devices end-to-end
- [ ] Correct start ordering (workers up before head) coordinated via the control API
- [ ] Health checks: is the LLM port answering? is each worker connected?
- [ ] Restart/backoff on crash; surface errors on the dashboard
- [ ] Integration test harness (multi-process on one host, then multi-device CI)

## Phase 2 — Assistant experience 🔜

- [ ] Bundle/auto-configure Open WebUI against the head endpoint
- [ ] One-click voice: wire Whisper STT + a local TTS (Kokoro/Edge) in Open WebUI
- [ ] "Push-to-talk" and wake-word notes; latency budget for voice on small clusters
- [ ] Optional: lightweight built-in chat page for devices that can't run Open WebUI

## Phase 3 — Tailscale & remote 🔜

- [ ] Detect Tailscale (`tailscale status --json`) and auto-populate seeds
- [ ] Prefer tailnet IPs for prima.cpp addresses when peers aren't on the same LAN
- [ ] Document firewall/ACL setup; test a Mac + phone-on-cellular cluster

## Phase 4 — Packaging & install 🔜

- [ ] macOS: signed `.app` / launchd agent + menubar status
- [ ] Android: Termux package + Termux:Widget one-tap, or a thin APK wrapper
- [ ] Windows: service + tray icon
- [ ] `curl | sh` installers per platform; auto-fetch/build prima.cpp

## Phase 5 — Model management 🔜

- [ ] Pick, download and verify GGUF models from the dashboard
- [ ] Curated list (including uncensored options) with size/RAM guidance
- [ ] Per-cluster model selection; ensure every node has the same file
- [ ] Show whether the chosen model fits the current mesh's combined memory

## Later / ideas

- [ ] Authentication + TLS for the control API
- [ ] Gossip-reconciled membership (replace last-seen registry)
- [ ] Heterogeneous role hints (pin head, exclude a device, cap RAM per node)
- [ ] Metrics: tokens/sec, per-node load, bottleneck highlighting on the dashboard
- [ ] Multiple concurrent models / routing

Want to shape this? Open an issue describing your device mix and what you'd use it for.
