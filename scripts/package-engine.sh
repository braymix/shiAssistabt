#!/usr/bin/env bash
# Package a built prima.cpp checkout into shika-engine-<platform>.zip with
# llama-server / llama-cli and their shared libraries at the top level, which is
# the layout internal/engine expects when it extracts the bundle.
#
#   bash scripts/package-engine.sh <prima-dir> <platform>   # e.g. linux-amd64
set -euo pipefail

PRIMA="${1:?prima dir}"
PLATFORM="${2:?platform}"
OUT="$PWD/shika-engine-${PLATFORM}.zip"
stage="$(mktemp -d)"

for bin in llama-server llama-cli; do
  src="$(find "$PRIMA/build" -type f -name "$bin" 2>/dev/null | head -n1 || true)"
  [ -n "$src" ] || { echo "package-engine: could not find $bin under $PRIMA/build" >&2; exit 1; }
  cp "$src" "$stage/$bin"
  chmod +x "$stage/$bin"
  # Bundle any shared libs sitting next to the binary (ggml/llama .so/.dylib).
  bindir="$(dirname "$src")"
  find "$bindir" -maxdepth 1 -type f \( -name '*.so' -o -name '*.so.*' -o -name '*.dylib' \) \
    -exec cp {} "$stage/" \; 2>/dev/null || true
done

( cd "$stage" && zip -r -9 "$OUT" . >/dev/null )
echo "package-engine: wrote $OUT"
