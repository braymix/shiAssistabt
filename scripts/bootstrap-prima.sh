#!/bin/sh
# Fetch and build prima.cpp (shikA's data plane) into the directory shikA
# expects, then place the llama-server / llama-cli binaries where the supervisor
# launches them (directly under $PRIMA_DIR).
#
#   sh scripts/bootstrap-prima.sh          # into ~/prima.cpp
#   PRIMA_DIR=/opt/prima.cpp sh scripts/bootstrap-prima.sh
#
# Needs: git, cmake (or make), a C/C++ toolchain. This is best-effort across
# platforms — if your prima.cpp needs special flags (CUDA, Metal, etc.), build
# it by hand per its README and just drop the two binaries in $PRIMA_DIR.
set -eu

PRIMA_DIR="${PRIMA_DIR:-$HOME/prima.cpp}"
PRIMA_REPO="${PRIMA_REPO:-https://github.com/OpenCPIL/prima.cpp}"

say() { printf '\033[36mprima\033[0m %s\n' "$*"; }
die() { printf '\033[31mprima error:\033[0m %s\n' "$*" >&2; exit 1; }

command -v git >/dev/null 2>&1 || die "git is required"

# 1. clone or update
if [ -d "$PRIMA_DIR/.git" ]; then
  say "updating $PRIMA_DIR"
  git -C "$PRIMA_DIR" pull --ff-only || say "pull skipped (local changes?)"
else
  say "cloning $PRIMA_REPO -> $PRIMA_DIR"
  git clone --depth 1 "$PRIMA_REPO" "$PRIMA_DIR"
fi

# 2. build (prefer cmake, fall back to make)
cd "$PRIMA_DIR"
if command -v cmake >/dev/null 2>&1; then
  say "building with cmake"
  cmake -B build -DCMAKE_BUILD_TYPE=Release
  cmake --build build --config Release -j "$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)"
elif command -v make >/dev/null 2>&1; then
  say "building with make"
  make -j "$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)"
else
  die "need cmake or make to build prima.cpp"
fi

# 3. surface the binaries at $PRIMA_DIR root, where shikA execs "./llama-server".
linked=0
for bin in llama-server llama-cli; do
  if [ -x "$PRIMA_DIR/$bin" ]; then
    linked=$((linked + 1))
    continue
  fi
  found=$(find "$PRIMA_DIR" -type f -name "$bin" -perm -u+x 2>/dev/null | head -n1 || true)
  if [ -n "$found" ] && [ "$found" != "$PRIMA_DIR/$bin" ]; then
    ln -sf "$found" "$PRIMA_DIR/$bin"
    linked=$((linked + 1))
    say "linked $bin -> $found"
  fi
done

if [ "$linked" -eq 2 ]; then
  say "done — put your GGUF under $PRIMA_DIR/download/ and press Start in shikA."
else
  say "build finished but couldn't find both binaries; check prima.cpp's build output"
  say "and symlink llama-server + llama-cli into $PRIMA_DIR manually."
fi
