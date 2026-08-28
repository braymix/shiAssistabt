#!/bin/sh
# shikA installer — downloads the right shikad binary for this machine, drops it
# on PATH, and (optionally) installs a service so it runs in the background.
#
#   curl -fsSL https://raw.githubusercontent.com/braymix/shika/main/packaging/install.sh | sh
#
# Env overrides:
#   SHIKA_REPO      GitHub owner/repo to fetch releases from (default braymix/shika)
#   SHIKA_VERSION   release tag to install (default: latest)
#   SHIKA_BINDIR    install directory (default: $HOME/.local/bin)
#   SHIKA_SERVICE   install+enable a background service: 1 (default) or 0
set -eu

REPO="${SHIKA_REPO:-braymix/shika}"
VERSION="${SHIKA_VERSION:-latest}"
BINDIR="${SHIKA_BINDIR:-$HOME/.local/bin}"
SERVICE="${SHIKA_SERVICE:-1}"

say()  { printf '\033[36mshikA\033[0m %s\n' "$*"; }
die()  { printf '\033[31mshikA error:\033[0m %s\n' "$*" >&2; exit 1; }

# ---- detect platform ---------------------------------------------------------
os=$(uname -s)
arch=$(uname -m)
ext=""
case "$os" in
  Linux)
    # Termux reports Linux but needs the android path handled by shikad itself.
    goos="linux" ;;
  Darwin) goos="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) goos="windows"; ext=".exe" ;;
  *) die "unsupported OS: $os (see packaging/README.md for manual steps)" ;;
esac
case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) die "unsupported CPU: $arch" ;;
esac

asset="shikad-${goos}-${goarch}${ext}"

# ---- resolve download URL ----------------------------------------------------
if [ "$VERSION" = "latest" ]; then
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
fi

say "installing ${asset} (${VERSION}) from ${REPO}"
mkdir -p "$BINDIR"
tmp=$(mktemp)
if command -v curl >/dev/null 2>&1; then
  curl -fSL "$url" -o "$tmp" || die "download failed: $url"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp" "$url" || die "download failed: $url"
else
  die "need curl or wget to download"
fi
chmod +x "$tmp"
mv "$tmp" "$BINDIR/shikad${ext}"
say "installed to $BINDIR/shikad${ext}"

case ":$PATH:" in
  *":$BINDIR:"*) : ;;
  *) say "note: add $BINDIR to your PATH" ;;
esac

# ---- optional service --------------------------------------------------------
if [ "$SERVICE" != "1" ]; then
  say "done (service install skipped)"
  exit 0
fi

case "$goos" in
  linux)
    if command -v systemctl >/dev/null 2>&1 && [ -n "${XDG_RUNTIME_DIR:-}" ]; then
      unitdir="$HOME/.config/systemd/user"
      mkdir -p "$unitdir"
      sed "s|%h/.local/bin/shikad|$BINDIR/shikad|" \
        "$(dirname "$0")/systemd/shikad.service" > "$unitdir/shikad.service" 2>/dev/null || \
        cat > "$unitdir/shikad.service" <<EOF
[Unit]
Description=shikA distributed AI orchestrator
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=$BINDIR/shikad
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
EOF
      systemctl --user daemon-reload
      systemctl --user enable --now shikad.service
      say "systemd user service enabled — dashboard on http://localhost:8977"
    else
      say "systemd --user not available; start manually: $BINDIR/shikad"
    fi ;;
  darwin)
    plist="$HOME/Library/LaunchAgents/com.shika.shikad.plist"
    mkdir -p "$HOME/Library/LaunchAgents"
    cat > "$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.shika.shikad</string>
  <key>ProgramArguments</key><array><string>$BINDIR/shikad</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>$HOME/Library/Logs/shikad.log</string>
  <key>StandardErrorPath</key><string>$HOME/Library/Logs/shikad.err.log</string>
</dict></plist>
EOF
    launchctl unload "$plist" 2>/dev/null || true
    launchctl load "$plist"
    say "launchd agent loaded — dashboard on http://localhost:8977" ;;
  *)
    say "no service integration for $goos; run $BINDIR/shikad${ext} yourself" ;;
esac

say "done."
