#!/bin/sh
set -e

# jt installer — downloads the latest precompiled binary from GitHub releases
# Usage: curl -fsSL https://raw.githubusercontent.com/okaris/jt/main/install.sh | sh

REPO="okaris/jt"
INSTALL_DIR="${JT_INSTALL_DIR:-/usr/local/bin}"
BINARY="jt"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  armv7l)         ARCH="arm" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Get latest release tag
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$LATEST" ]; then
  echo "Failed to fetch latest release" >&2
  exit 1
fi

echo "Installing jt ${LATEST} (${OS}/${ARCH})..."

# Build download URL
EXT=""
if [ "$OS" = "windows" ]; then
  EXT=".exe"
fi
FILENAME="jt-${OS}-${ARCH}${EXT}"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${FILENAME}"

# Download
TMPFILE=$(mktemp)
if ! curl -fsSL "$URL" -o "$TMPFILE"; then
  echo "Download failed: $URL" >&2
  rm -f "$TMPFILE"
  exit 1
fi

# Install
chmod +x "$TMPFILE"
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}${EXT}"
else
  echo "Need sudo to install to ${INSTALL_DIR}"
  sudo mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}${EXT}"
fi

echo "Installed jt ${LATEST} to ${INSTALL_DIR}/${BINARY}${EXT}"
jt --version 2>/dev/null || true
