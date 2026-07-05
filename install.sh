#!/bin/sh
# RegressGuard installer
# Usage: curl -fsSL https://raw.githubusercontent.com/regressguard/regressguard/main/install.sh | sh

set -e

REPO="Bharath-code/regressguard"
BINARY="rg"
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

ARCHIVE="rg_${VERSION}_${OS_NAME}_${ARCH_NAME}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/v${VERSION}/${ARCHIVE}"

echo "Installing RegressGuard"
echo ""

# Download to a temp directory.
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "> Downloading rg $VERSION ($OS_NAME/$ARCH_NAME)..."
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

# Verify using the absolute path — `command -v rg` may find ripgrep instead.
INSTALLED="$INSTALL_DIR/$BINARY"
if [ -x "$INSTALLED" ]; then
  echo "OK Installed rg $VERSION to $INSTALLED"
else
  echo "Install failed: $INSTALLED is missing or not executable."
  exit 1
fi

RESOLVED="$(command -v rg 2>/dev/null || true)"
if [ -z "$RESOLVED" ]; then
  echo ""
  echo "rg is not in your PATH. Add this to your shell profile:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
elif [ "$RESOLVED" != "$INSTALLED" ]; then
  echo ""
  echo "WARNING: 'rg' in your PATH resolves to $RESOLVED, not RegressGuard."
  if "$RESOLVED" --version 2>/dev/null | grep -q ripgrep; then
    echo "That is ripgrep. Typing 'rg' will run ripgrep, not RegressGuard."
  fi
  echo "Use the full path instead:"
  echo "  $INSTALLED version"
fi

echo ""
echo "Verify:"
echo "  $INSTALLED version"
