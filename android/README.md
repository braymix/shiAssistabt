# shikA for Android (native APK)

A real installable app: it bundles the `shikad` orchestrator as a native binary
and runs it as a **foreground service**, so an Android phone becomes a full
worker node in your AI mesh — no Termux required.

## How it works

`shikad` is pure Go, so it cross-compiles straight to an Android-native
executable (`GOOS=android GOARCH=arm64`) with no NDK. We ship it as
`libshikad.so` inside the APK's `jniLibs/arm64-v8a/`, which Android unpacks into
the read-only, **executable** `nativeLibraryDir`. The foreground service execs it
there, holds a CPU wakelock and a Wi-Fi **multicast lock** (so LAN
auto-discovery works), and a WebView shows the dashboard from `localhost:8977`.

## Build

You need the Go toolchain and the Android SDK (Android Studio, or command-line
tools with a JDK 17).

```bash
# 1. build the orchestrator binary into jniLibs (from the repo root)
make android

# 2. build the APK
cd android
gradle wrapper        # first time only, if you don't use Android Studio
./gradlew assembleRelease
# -> app/build/outputs/apk/release/app-release-unsigned.apk
```

Open the `android/` folder in Android Studio to build/run/sign with a click
instead.

## Install

Sideload the APK (`adb install app-release.apk`, or copy it to the phone and
tap it with "install unknown apps" allowed). Launch **shikA**: it starts the
service, shows a persistent notification, and the phone appears automatically in
the dashboard of every other shikA device on the same Wi-Fi.

For remote phones (mobile data), run shikA on your other devices with Tailscale
and add the phone's tailnet address as a seed — see the repo README.

## Signing & Play Store

`assembleRelease` produces an **unsigned** APK. Sign it with your own keystore
(`apksigner`) for sideloading, or wire a signing config for a Play Store build.
A signed release pipeline is Phase 4 roadmap work.

## Not arm64?

Almost every modern phone is arm64. For an older 32-bit device, add the ABI to
`ndk.abiFilters` in `app/build.gradle` and cross-build that binary into the
matching `jniLibs/<abi>/libshikad.so`.
