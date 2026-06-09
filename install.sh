#!/usr/bin/env sh
set -e
REPO="dheerajnalapat/lane"   # adjust
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH=amd64; [ "$ARCH" = "aarch64" ] && ARCH=arm64
URL="https://github.com/$REPO/releases/latest/download/lane_${OS}_${ARCH}.tar.gz"
TMP=$(mktemp -d)
echo "downloading $URL"
curl -sSL "$URL" | tar -xz -C "$TMP"
install -m 0755 "$TMP/lane" /usr/local/bin/lane
echo "installed lane to /usr/local/bin/lane"
