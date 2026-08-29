# Packaging & install

shikA ships as a single dependency-free `shikad` binary per device. This
directory holds the install tooling around it.

## One-liners

| Platform | Command |
|----------|---------|
| Linux / macOS | `curl -fsSL https://raw.githubusercontent.com/braymix/shika/main/packaging/install.sh \| sh` |
| Windows (PowerShell) | `irm https://raw.githubusercontent.com/braymix/shika/main/packaging/windows/install.ps1 \| iex` |
| Android | see [`termux/README.md`](termux/README.md) |

`install.sh` detects your OS/CPU, downloads the matching release asset, drops
`shikad` on your PATH, and enables a background service:

- **Linux** — a systemd **user** service ([`systemd/shikad.service`](systemd/shikad.service)).
- **macOS** — a launchd agent ([`launchd/com.shika.shikad.plist`](launchd/com.shika.shikad.plist)).
- **Windows** — a per-user logon scheduled task (no admin needed).

Override behaviour with `SHIKA_REPO`, `SHIKA_VERSION`, `SHIKA_BINDIR`, or
`SHIKA_SERVICE=0` to skip the service.

## Releases

Tagging `vX.Y.Z` triggers [`.github/workflows/release.yml`](../.github/workflows/release.yml),
which cross-compiles every target, writes `SHA256SUMS`, and publishes a GitHub
Release. `install.sh` pulls from `releases/latest` by default.

Build the same artifacts locally with `make dist`.

## Android

Two ways to make a phone a full worker node:

- **Native APK** — [`android/`](../android/) bundles `shikad` (built NDK-free via
  `make android`) and runs it as a foreground service. Build with
  `cd android && ./gradlew assembleRelease`, then sideload. This is the
  no-Termux path.
- **Termux** — [`termux/`](termux/) for power users who prefer a shell.

## prima.cpp (data plane)

`sh scripts/bootstrap-prima.sh` (or `make prima`) clones and builds prima.cpp
into `~/prima.cpp` and links the `llama-server` / `llama-cli` binaries where
shikA launches them.

## Native installers — still TODO (Phase 4)

The scripts above cover every desktop today, and the Android APK builds from
`android/`. The roadmap's signed, one-click packages are not automated yet:

- **Android `.apk`** — buildable now; a **signed** CI release is still TODO.
- **Windows `.exe`/MSI** — a real installer registering a system service + tray icon.
- **macOS `.app`** — signed/notarized bundle + menubar item.
- **Linux `.deb` / AppImage** — for Mint/Ubuntu and portable use.
- **iPad** — client-first iOS app (dashboard + chat/voice; iOS limits background compute).

These need per-platform toolchains and code-signing identities, so they live
outside this scaffold until a signing pipeline exists.
