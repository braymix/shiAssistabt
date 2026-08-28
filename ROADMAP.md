# Roadmap

shikA is built in phases. Each phase is a usable milestone, not a big-bang.

## Phase 0 — Orchestrator skeleton ✅ (v0.1, this repo)

- [x] Node identity + hardware detection (RAM, cores, GPU, OS incl. Termux→android)
- [x] Zero-config LAN discovery (UDP multicast beacons)
- [x] Seed-based discovery for cross-subnet / Tailscale peers
- [x] Membership registry with liveness timeout
- [x] Deterministic cluster planner (head election, ranks, ring, prima.cpp argv)
- [x] Supervisor with safe dry-run when prima.cpp isn't built
- [x] Control API + embedded device-management dashboard
- [x] Builds and runs as a single dependency-free binary

## Phase 1 — Real distributed launch 🚧

- [x] Correct start ordering (workers up before head) coordinated via the control API
- [x] Health checks: is the LLM port answering? (per-worker "connected" still TODO)
- [x] Restart/backoff on crash; surface errors on the dashboard
- [x] Process-group teardown so killing prima.cpp reaps its forked helpers
- [x] Integration test harness (multi-process on one host via loopback seeds)
- [ ] Launch & supervise **real** prima.cpp across 2+ physical devices end-to-end
      (needs real hardware + a prima.cpp build; harness above stubs the binary)

## Phase 2 — Assistant experience 🚧

- [x] Auto-configure Open WebUI against the head endpoint (dashboard shows a
      ready-to-run `docker run` wired to the live head URL)
- [x] Voice guidance: Whisper STT + local TTS (Kokoro/Edge) setup surfaced in the UI
- [x] Lightweight built-in chat page (`/chat.html`) proxied to the head, for
      devices that can't run Open WebUI
- [ ] One-click voice bundle (ship/pre-configure Whisper+TTS rather than document it)
- [ ] Push-to-talk / wake-word; measured latency budget on small clusters

## Phase 3 — Tailscale & remote 🚧

- [x] Detect Tailscale (`tailscale status --json`) and auto-populate seeds
- [x] Prefer tailnet IPs for prima.cpp addresses when peers aren't on the same LAN
      (`prefer_tailscale_ip` / `-prefer-tailscale-ip`)
- [ ] Document firewall/ACL setup; test a Mac + phone-on-cellular cluster (needs real tailnet)

## Phase 4 — Packaging & install 🔜

**Goal: one downloadable installer per device class**, so a non-technical user
installs shikA the way they install any app. Target artifacts:

- [ ] **Android — `.apk`**: a thin APK that bundles the `shikad` arm64 binary and
      starts it as a foreground service (with a persistent notification), plus a
      Termux path for power users. This is a full worker node.
- [ ] **Windows — `.exe`**: an installer (`.exe`/MSI) that registers `shikad` as a
      Windows service with a tray icon. Full worker node.
- [ ] **macOS (MacBook) — `.app`**: a signed/notarized `.app` + launchd agent with a
      menubar status item. Full worker/head node.
- [ ] **Linux (incl. Linux Mint) — `.deb` / AppImage**: a `.deb` (Mint/Ubuntu) and a
      portable AppImage, installing a systemd user service. Full worker/head node.
- [ ] **iPad — iOS app (client-first)**: an App Store / TestFlight app. iOS forbids the
      long-running background compute a worker needs, so the iPad ships first as a
      **dashboard + chat/voice client** (manage the mesh, talk to the assistant) and
      contributes compute only within what iOS later allows. Set expectations here
      rather than promising a full worker.
- [ ] Shared: `curl | sh` bootstrap per desktop platform; auto-fetch/build prima.cpp;
      one code-signing + release pipeline in CI feeding all of the above.

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
