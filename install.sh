#!/bin/sh
# RegressGuard installer
# Usage: curl -fsSL https://raw.githubusercontent.com/regressguard/regressguard/main/install.sh | sh

set -e

REPO="Bharath-code/regressguard"
BINARY="regressguard"
INSTALL_DIR="${RG_INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture.
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Linux)  OS_NAME="linux" ;;
  Darwin) OS_NAME="darwin" ;;
  *)
    echo "Unsupported OS: $OS"
    echo "Download manually from: https://github.com/$REPO/releases"
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64)  ARCH_NAME="amd64" ;;
  aarch64) ARCH_NAME="arm64" ;;
  arm64)   ARCH_NAME="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    echo "Download manually from: https://github.com/$REPO/releases"
    exit 1
    ;;
esac

# Fetch the latest release version.
LATEST_URL="https://api.github.com/repos/$REPO/releases/latest"
VERSION="$(curl -fsSL "$LATEST_URL" | grep '"tag_name"' | sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/')"

if [ -z "$VERSION" ]; then
  echo "Could not determine latest version."
  echo "Check: https://github.com/$REPO/releases"
  exit 1
fi

ARCHIVE="regressguard_${VERSION}_${OS_NAME}_${ARCH_NAME}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/v${VERSION}/${ARCHIVE}"

echo "Installing RegressGuard"
echo ""

# Download to a temp directory.
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "> Downloading regressguard $VERSION ($OS_NAME/$ARCH_NAME)..."
curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE"

echo "> Extracting..."
tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR"

# Install the binary.
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
  chmod +x "$INSTALL_DIR/$BINARY"
else
  echo "> Requesting sudo to install to $INSTALL_DIR..."
  sudo mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
  sudo chmod +x "$INSTALL_DIR/$BINARY"
fi

INSTALLED="$INSTALL_DIR/$BINARY"
if [ -x "$INSTALLED" ]; then
  echo "OK Installed regressguard $VERSION to $INSTALLED"
else
  echo "Install failed: $INSTALLED is missing or not executable."
  exit 1
fi

# Pre-rename installs left an `rg` binary; it is not removed (it is yours to delete).
OLD="$INSTALL_DIR/rg"
if [ -x "$OLD" ] && "$OLD" version 2>/dev/null | grep -q RegressGuard; then
  echo ""
  echo "Note: $OLD is the old RegressGuard binary name. Delete it when ready: rm $OLD"
fi

if ! command -v regressguard >/dev/null 2>&1; then
  echo ""
  echo "regressguard is not in your PATH. Add this to your shell profile:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
fi

echo ""
echo "Verify:"
echo "  $INSTALLED version"
