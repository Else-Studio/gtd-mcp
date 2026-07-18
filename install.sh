#!/bin/sh
set -e

# Configurable defaults
OWNER="Else-Studio"
REPO="gtd-mcp"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
DOWNLOAD_BASE_URL="${GTD_DOWNLOAD_BASE_URL:-https://github.com/$OWNER/$REPO/releases/download}"

# OS and Arch Detection
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  darwin)  OS="darwin" ;;
  linux)   OS="linux" ;;
  *)       echo "Error: Unsupported OS '$OS'"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)            echo "Error: Unsupported Architecture '$ARCH'"; exit 1 ;;
esac

# Resolve Version
if [ -z "$GTD_VERSION" ]; then
  if [ -n "$GTD_DOWNLOAD_BASE_URL" ] && [ "$DOWNLOAD_BASE_URL" != "https://github.com/$OWNER/$REPO/releases/download" ]; then
    VERSION="0.1.0" # Dummy fallback for local testing
  else
    VERSION=$(curl -s "https://api.github.com/repos/$OWNER/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
      echo "Error: Could not resolve latest release version from GitHub."
      exit 1
    fi
  fi
else
  VERSION="$GTD_VERSION"
fi

VERSION_CLEAN=$(echo "$VERSION" | sed 's/^v//')
BINARY_NAME="gtd"
ARCHIVE_NAME="${BINARY_NAME}_${VERSION_CLEAN}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="${DOWNLOAD_BASE_URL}/${VERSION}/${ARCHIVE_NAME}"
CHECKSUM_URL="${DOWNLOAD_BASE_URL}/${VERSION}/checksums.txt"

mkdir -p "$BIN_DIR"
TMP_DIR=$(mktemp -d)
clean_up() { rm -rf "$TMP_DIR"; }
trap clean_up EXIT INT TERM

echo "Downloading ${BINARY_NAME} ${VERSION}..."
curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ARCHIVE_NAME}"

echo "Verifying checksum..."
curl -fsSL "$CHECKSUM_URL" -o "${TMP_DIR}/checksums.txt"
(
  cd "$TMP_DIR"
  grep "$ARCHIVE_NAME" checksums.txt > ours.txt
  if ! sha256sum -c ours.txt >/dev/null 2>&1 && ! shasum -a 256 -c ours.txt >/dev/null 2>&1; then
    echo "Error: Checksum verification failed!"
    exit 1
  fi
)

echo "Installing to ${BIN_DIR}/${BINARY_NAME}..."
tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"
mv "${TMP_DIR}/${BINARY_NAME}" "$BIN_DIR/"
chmod +x "${BIN_DIR}/${BINARY_NAME}"

echo "Successfully installed ${BINARY_NAME} to ${BIN_DIR}!"
case ":$PATH:" in
  *:"$BIN_DIR":*) ;;
  *)
    echo ""
    echo "Warning: '${BIN_DIR}' is not in your PATH."
    echo "To run '${BINARY_NAME}', add it to your shell config file:"
    echo "  export PATH=\"\$PATH:${BIN_DIR}\""
    ;;
esac
