#!/bin/bash
set -e

REPO="khemerak/ntm"

echo "Fetching latest release information for $REPO..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
  echo "Error: Could not determine latest release tag."
  exit 1
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then ARCH="arm64"; fi

BINARY_NAME="ntm-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$BINARY_NAME"

echo "Downloading ntm $LATEST_TAG for $OS-$ARCH..."

TMP_BINARY="$(mktemp /tmp/ntm.XXXXXX)"
trap 'rm -f "$TMP_BINARY"' EXIT

if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_BINARY"; then
  echo "Error: Failed to download $DOWNLOAD_URL"
  exit 1
fi

chmod +x "$TMP_BINARY"

echo "Installing to /usr/local/bin (administrator password is required)..."

sudo mv "$TMP_BINARY" /usr/local/bin/ntm

echo "✓ ntm installed successfully!"
echo "Please run: ntm --help to see more"
