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

## Phase 4 — Packaging & install 🚧

**Goal: one downloadable installer per device class**, so a non-technical user
installs shikA the way they install any app.

Shipped so far (see [`packaging/`](packaging/)): a `curl | sh` / PowerShell
bootstrap that installs `shikad` and a background service (systemd user unit,
launchd agent, or Windows logon task), a Termux path for Android, and a
tag-triggered GitHub Release pipeline (`make dist` locally). The signed native
packages below are still TODO — they need per-platform toolchains and signing
identities:

- [x] **Android — `.apk`**: a thin APK (in [`android/`](android/)) that bundles the
      `shikad` arm64 binary (built NDK-free via `GOOS=android`, `make android`) and
      runs it as a foreground service with a persistent notification, CPU wakelock,
      and multicast lock; a WebView shows the dashboard. Full worker node. Termux
      path also documented. A CI workflow builds a **signed, installable** APK on
      every tag with zero setup (it auto-generates a throwaway key if you don't
      provide keystore secrets) and attaches it to the Release.
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
- [x] Shared: `curl | sh` / PowerShell bootstrap per desktop platform, a
      tag-triggered CI release pipeline, and `make prima` to auto-fetch/build
      prima.cpp. (Code-signing still TODO.)

## Phase 5 — Model management 🚧

- [x] Pick, download and verify (SHA256) GGUF models from the dashboard
- [x] Curated list (including an uncensored option) with size/RAM guidance
- [x] Show whether a model fits the current mesh's combined memory
- [x] Per-cluster model selection: the head advertises the chosen model and
      every node builds its command for that file and auto-downloads it, so the
      whole mesh converges on one model
- [ ] Pin real SHA256s for every catalog entry (some ship unverified for now)

## Later / ideas

- [ ] Authentication + TLS for the control API
- [ ] Gossip-reconciled membership (replace last-seen registry)
- [ ] Heterogeneous role hints (pin head, exclude a device, cap RAM per node)
- [ ] Metrics: tokens/sec, per-node load, bottleneck highlighting on the dashboard
- [ ] Multiple concurrent models / routing

Want to shape this? Open an issue describing your device mix and what you'd use it for.
