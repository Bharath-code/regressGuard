#!/bin/sh
# ponytail: uname-based binary picker; add win32 when Windows builds exist
dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
esac
bin="$dir/bin/rg-$os-$arch"
if [ ! -x "$bin" ]; then
  echo "regressguard: unsupported platform $os/$arch" >&2
  exit 2
fi
chmod +x "$bin" 2>/dev/null
exec "$bin" mcp serve
