#!/bin/sh
set -e

REPO="codeofmario/wiremap"
INSTALL_DIR="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

LATEST=$(curl -sI "https://github.com/$REPO/releases/latest" | grep -i location | sed 's/.*tag\///' | tr -d '\r')
if [ -z "$LATEST" ]; then
  echo "Error: failed to fetch latest release version"; exit 1
fi
if ! echo "$LATEST" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+'; then
  echo "Error: invalid version format: $LATEST"; exit 1
fi

TARBALL="wiremap_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/${LATEST}/${TARBALL}"
CHECKSUM_URL="https://github.com/$REPO/releases/download/${LATEST}/checksums.txt"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading wiremap $LATEST for ${OS}/${ARCH}..."
curl -sL "$URL" -o "$TMPDIR/$TARBALL"
curl -sL "$CHECKSUM_URL" -o "$TMPDIR/checksums.txt" 2>/dev/null || true

if [ -f "$TMPDIR/checksums.txt" ] && grep -q "$TARBALL" "$TMPDIR/checksums.txt"; then
  echo "Verifying checksum..."
  EXPECTED=$(grep "$TARBALL" "$TMPDIR/checksums.txt" | awk '{print $1}')
  ACTUAL=$(sha256sum "$TMPDIR/$TARBALL" | awk '{print $1}')
  if [ "$EXPECTED" != "$ACTUAL" ]; then
    echo "Error: checksum mismatch (expected $EXPECTED, got $ACTUAL)"; exit 1
  fi
  echo "Checksum verified."
else
  echo "Warning: checksum file not available, skipping verification."
fi

tar xz -C "$TMPDIR" -f "$TMPDIR/$TARBALL" wiremap
sudo install "$TMPDIR/wiremap" "$INSTALL_DIR/wiremap"

echo "wiremap installed to $INSTALL_DIR/wiremap"
