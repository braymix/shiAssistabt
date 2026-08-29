#!/bin/sh
# Cross-compile prima.cpp (the inference engine) for Android arm64 with the NDK
# and drop the two binaries shikA launches — llama-server and llama-cli — into a
# jniLibs directory as libllama-server.so / libllama-cli.so, so the APK can carry
# the engine and phones do real inference instead of dry-run.
#
#   ANDROID_NDK_HOME=/path/to/ndk sh scripts/build-prima-android.sh <jniLibs-abi-dir>
#
# Best-effort: prima.cpp's own Android build may need tweaks; this encodes the
# standard llama.cpp NDK recipe. Exits non-zero if it can't produce both binaries
# so callers can degrade gracefully.
set -eu

OUT_DIR="${1:-android/app/src/main/jniLibs/arm64-v8a}"
PRIMA_REPO="${PRIMA_REPO:-https://github.com/OpenCPIL/prima.cpp}"
ABI="${ANDROID_ABI:-arm64-v8a}"
API="${ANDROID_API:-26}"
WORK="${WORK:-$(mktemp -d)}"

say() { printf '\033[36mprima-android\033[0m %s\n' "$*"; }
die() { printf '\033[31mprima-android error:\033[0m %s\n' "$*" >&2; exit 1; }

[ -n "${ANDROID_NDK_HOME:-}" ] || die "set ANDROID_NDK_HOME to your NDK path"
TOOLCHAIN="$ANDROID_NDK_HOME/build/cmake/android.toolchain.cmake"
[ -f "$TOOLCHAIN" ] || die "NDK toolchain not found at $TOOLCHAIN"
command -v cmake >/dev/null 2>&1 || die "cmake is required"

if [ ! -d "$WORK/prima.cpp/.git" ]; then
  say "cloning $PRIMA_REPO"
  git clone --depth 1 "$PRIMA_REPO" "$WORK/prima.cpp"
fi
cd "$WORK/prima.cpp"

say "configuring (NDK $ABI, android-$API)"
cmake -B build-android \
  -DCMAKE_TOOLCHAIN_FILE="$TOOLCHAIN" \
  -DANDROID_ABI="$ABI" \
  -DANDROID_PLATFORM="android-$API" \
  -DCMAKE_BUILD_TYPE=Release \
  -DGGML_OPENMP=OFF \
  -DLLAMA_CURL=OFF \
  -DGGML_LLAMAFILE=OFF

say "building"
cmake --build build-android --config Release -j "$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)" || \
  say "full build reported errors; will still look for the binaries we need"

mkdir -p "$OUT_DIR"
found=0
for pair in "llama-server:libllama-server.so" "llama-cli:libllama-cli.so"; do
  bin="${pair%%:*}"; lib="${pair##*:}"
  # prima.cpp/llama.cpp names vary across versions (server/main vs llama-*).
  src=$(find build-android -type f \( -name "$bin" -o -name "${bin#llama-}" \) 2>/dev/null | head -n1 || true)
  if [ -n "$src" ]; then
    cp "$src" "$OUT_DIR/$lib"
    chmod +x "$OUT_DIR/$lib"
    say "packaged $lib  (from $src)"
    found=$((found + 1))
  else
    say "could not find a binary for $bin"
  fi
done

[ "$found" -eq 2 ] || die "did not produce both engine binaries (got $found/2)"
say "done — engine libs in $OUT_DIR"
