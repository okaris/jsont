#!/bin/sh
set -e

# jsont installer — downloads the latest precompiled binary from GitHub releases
# Usage: curl -fsSL i.jsont.sh | sh

REPO="okaris/jsont"
INSTALL_DIR="${JSONT_INSTALL_DIR:-/usr/local/bin}"

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

echo "Installing jsont ${LATEST} (${OS}/${ARCH})..."

# Build download URL
EXT=""
if [ "$OS" = "windows" ]; then
  EXT=".exe"
fi
FILENAME="jsont-${OS}-${ARCH}${EXT}"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${FILENAME}"

# Download
TMPFILE=$(mktemp)
if ! curl -fsSL "$URL" -o "$TMPFILE"; then
  echo "Download failed: $URL" >&2
  rm -f "$TMPFILE"
  exit 1
fi

# Install binary as jsont
chmod +x "$TMPFILE"
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPFILE" "${INSTALL_DIR}/jsont${EXT}"
  ln -sf "${INSTALL_DIR}/jsont${EXT}" "${INSTALL_DIR}/jt${EXT}"
else
  echo "Need sudo to install to ${INSTALL_DIR}"
  sudo mv "$TMPFILE" "${INSTALL_DIR}/jsont${EXT}"
  sudo ln -sf "${INSTALL_DIR}/jsont${EXT}" "${INSTALL_DIR}/jt${EXT}"
fi

echo "Installed jsont ${LATEST} to ${INSTALL_DIR}/jsont${EXT}"
echo "Alias: jt -> jsont"
jsont --version 2>/dev/null || true
