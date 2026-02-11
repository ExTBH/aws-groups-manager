#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="aws-groups-manager"
OWNER="ExTBH"
REPO="aws-groups-manager"

if [[ -z "$OWNER" ]]; then
  echo "error: installer is not configured with a GitHub owner" >&2
  echo "hint: update install.sh defaults" >&2
  exit 1
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux|darwin) ;;
  *)
    echo "error: unsupported OS: $OS" >&2
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "error: unsupported ARCH: $ARCH" >&2
    exit 1
    ;;
esac

ASSET="${BINARY_NAME}_${OS}_${ARCH}.tar.gz"
API_URL="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"

if command -v curl >/dev/null 2>&1; then
  RELEASE_JSON="$(curl -fsSL "$API_URL")"
elif command -v wget >/dev/null 2>&1; then
  RELEASE_JSON="$(wget -qO- "$API_URL")"
else
  echo "error: curl or wget is required" >&2
  exit 1
fi

DOWNLOAD_URL="$(printf '%s' "$RELEASE_JSON" | awk -v asset="$ASSET" '
  $0 ~ "\"name\": \""asset"\"" {found=1}
  found && $0 ~ /"browser_download_url":/ {
    gsub(/[",]/, "", $2)
    print $2
    exit
  }
')"

if [[ -z "$DOWNLOAD_URL" ]]; then
  echo "error: no release asset found for ${OS}/${ARCH} (${ASSET})" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

ARCHIVE_PATH="$TMP_DIR/release.tar.gz"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$DOWNLOAD_URL" -o "$ARCHIVE_PATH"
else
  wget -qO "$ARCHIVE_PATH" "$DOWNLOAD_URL"
fi

tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"

if [[ ! -f "$TMP_DIR/$BINARY_NAME" ]]; then
  echo "error: extracted archive does not contain $BINARY_NAME" >&2
  exit 1
fi

chmod +x "$TMP_DIR/$BINARY_NAME"

DEST="/usr/local/bin/$BINARY_NAME"
if [[ -w "/usr/local/bin" ]]; then
  install -m 0755 "$TMP_DIR/$BINARY_NAME" "$DEST"
elif command -v sudo >/dev/null 2>&1; then
  sudo install -m 0755 "$TMP_DIR/$BINARY_NAME" "$DEST"
else
  LOCAL_BIN="$HOME/.local/bin"
  mkdir -p "$LOCAL_BIN"
  install -m 0755 "$TMP_DIR/$BINARY_NAME" "$LOCAL_BIN/$BINARY_NAME"
  DEST="$LOCAL_BIN/$BINARY_NAME"
fi

echo "installed: $DEST"
echo "verification:"
echo "  $BINARY_NAME version"
echo "  $BINARY_NAME"

if [[ "$DEST" == "$HOME/.local/bin/$BINARY_NAME" ]]; then
  case ":${PATH}:" in
    *":$HOME/.local/bin:"*) ;;
    *)
      echo
      echo "hint: add ~/.local/bin to PATH"
      ;;
  esac
fi
