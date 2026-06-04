#!/usr/bin/env bash
set -euo pipefail

GITCODE_REPO="${MSCLI_GITCODE_REPO:-mindspore/mscli}"
GITCODE_BASE_URL="https://gitcode.com/api/v5/repos/${GITCODE_REPO}/releases"
INSTALL_DIR="$HOME/.mscli/bin"
BINARY_NAME="mscli"
REQUESTED_VERSION="${MSCLI_VERSION:-}"
CONNECT_TIMEOUT="${MSCLI_INSTALL_CONNECT_TIMEOUT:-5}"

# Detect OS.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "Error: unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# Detect architecture.
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

echo "Detected: ${OS}/${ARCH}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: required command not found: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd perl

fetch_json() {
  local url="$1"
  curl -fsSL \
    --connect-timeout "$CONNECT_TIMEOUT" \
    --retry 2 \
    --retry-delay 1 \
    -H "Accept: application/json" \
    "$url" </dev/null
}

json_release_tag() {
  perl -MJSON::PP -e '
    use strict;
    use warnings;
    local $/;
    my $payload = <STDIN>;
    my $json = decode_json($payload);
    print($json->{tag_name} // q());
  '
}

normalize_tag() {
  local version="$1"
  case "$version" in
    v*) printf '%s\n' "$version" ;;
    *) printf 'v%s\n' "$version" ;;
  esac
}

latest_from_gitcode() {
  local tag

  tag="$(fetch_json "${GITCODE_BASE_URL}/latest" | json_release_tag)"
  if [ -z "$tag" ]; then
    return 1
  fi
  normalize_tag "$tag"
}

resolve_latest() {
  local latest=""

  echo "Resolving latest release from GitCode..." >&2
  latest="$(latest_from_gitcode 2>/dev/null || true)"
  if [ -z "$latest" ]; then
    return 1
  fi
  printf '%s\n' "$latest"
}

if [ -n "$REQUESTED_VERSION" ]; then
  LATEST="$(normalize_tag "$REQUESTED_VERSION")"
  echo "Using requested release: ${LATEST}"
else
  if ! LATEST="$(resolve_latest)"; then
    echo "Error: could not determine latest release" >&2
    exit 1
  fi
fi

echo "Latest release: ${LATEST}"

ASSET="mscli-${OS}-${ARCH}"
URL="https://gitcode.com/${GITCODE_REPO}/releases/download/${LATEST}/${ASSET}"

# Download binary.
echo "Downloading ${ASSET} from GitCode..."
mkdir -p "$INSTALL_DIR"
curl -fSL -o "${INSTALL_DIR}/${BINARY_NAME}" "$URL" </dev/null
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo ""
echo "Installed mscli ${LATEST} to ${INSTALL_DIR}/${BINARY_NAME}"

# Auto-add to PATH.
PATH_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""

# Detect shell profile.
CURRENT_SHELL="$(basename "${SHELL:-bash}")"
case "$CURRENT_SHELL" in
  zsh)  PROFILE="$HOME/.zshrc" ;;
  bash)
    if [ -f "$HOME/.bash_profile" ]; then
      PROFILE="$HOME/.bash_profile"
    else
      PROFILE="$HOME/.bashrc"
    fi
    ;;
  *)    PROFILE="$HOME/.profile" ;;
esac

# Add to profile if not already there.
if [ -f "$PROFILE" ] && grep -qF "$INSTALL_DIR" "$PROFILE" 2>/dev/null; then
  echo ""
  echo "PATH already configured in ${PROFILE}"
else
  echo "$PATH_LINE" >> "$PROFILE"
  echo ""
  echo "Added mscli to PATH in ${PROFILE}"
fi
echo ""
echo "Run: source ${PROFILE} && mscli"
