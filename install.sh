#!/usr/bin/env sh
set -e
REPO="dheerajnalapat/berth"   # adjust
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH=amd64; [ "$ARCH" = "aarch64" ] && ARCH=arm64
URL="https://github.com/$REPO/releases/latest/download/berth_${OS}_${ARCH}.tar.gz"
TMP=$(mktemp -d)
echo "downloading $URL"
curl -sSL "$URL" | tar -xz -C "$TMP"
install -m 0755 "$TMP/berth" /usr/local/bin/berth
echo "installed berth to /usr/local/bin/berth"
