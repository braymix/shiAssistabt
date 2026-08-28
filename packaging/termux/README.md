# Running shikA on Android (Termux)

Until the packaged `.apk` lands (Phase 4), the fastest way to turn an Android
phone into a full shikA worker node is [Termux](https://termux.dev).

```sh
pkg update && pkg install curl
# fetch the arm64 build (most modern phones)
curl -fSL https://github.com/braymix/shika/releases/latest/download/shikad-linux-arm64 -o shikad
chmod +x shikad
./shikad -name "$(getprop ro.product.model)"
```

shikA detects Termux (via `$TERMUX_VERSION` / the Termux prefix) and reports the
node's OS as `android`, which changes how prima.cpp is launched.

## Keep it running in the background

Install the Termux services add-on so shikad survives the app being backgrounded:

```sh
pkg install termux-services
mkdir -p $PREFIX/var/service/shikad
cp start-shikad.sh $PREFIX/var/service/shikad/run
chmod +x $PREFIX/var/service/shikad/run
sv-enable shikad
```

Acquire a wakelock (`termux-wake-lock`) so Android does not suspend the CPU while
the mesh is serving a model.

## What's still TODO for Phase 4

A first-class `.apk` that bundles `shikad` and runs it as a foreground service
with a persistent notification (no Termux required). This directory is the
power-user path in the meantime.
