#!/bin/sh
# Install the latest (or a pinned) eigenmemory release from GitHub.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/DarkestAbed/eigenmemory/main/install.sh | sh
#   EIGENMEMORY_VERSION=v0.1.0 sh install.sh
#
# Environment overrides:
#   EIGENMEMORY_VERSION   release tag (default: latest)
#   INSTALL_DIR           install target (default: ~/.local/bin)
set -eu

REPO="DarkestAbed/eigenmemory"
VERSION="${EIGENMEMORY_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
    Linux*) os=linux ;;
    Darwin*) os=darwin ;;
    *) echo "error: unsupported OS '$(uname -s)'; download a binary manually from" >&2
       echo "https://github.com/${REPO}/releases" >&2; exit 1 ;;
esac

case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) echo "error: unsupported architecture '$(uname -m)'" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')"
    [ -n "$VERSION" ] || { echo "error: could not resolve latest release" >&2; exit 1; }
fi

asset="eigenmemory_${VERSION#v}_${os}_${arch}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/${VERSION}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading ${asset}..."
curl -fsSL -o "${tmp}/${asset}" "${base_url}/${asset}"
curl -fsSL -o "${tmp}/checksums.txt" "${base_url}/checksums.txt"

echo "Verifying checksum..."
expected="$(grep "${asset}" "${tmp}/checksums.txt" | awk '{print $1}')"
[ -n "$expected" ] || { echo "error: no checksum found for ${asset}" >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
    echo "${expected}  ${tmp}/${asset}" | sha256sum -c - >/dev/null
else
    echo "${expected}  ${tmp}/${asset}" | shasum -a 256 -c - >/dev/null
fi

mkdir -p "${INSTALL_DIR}"
tar -xzf "${tmp}/${asset}" -C "${tmp}" eigenmemory
mv "${tmp}/eigenmemory" "${INSTALL_DIR}/eigenmemory"
chmod +x "${INSTALL_DIR}/eigenmemory"

echo "Installed ${INSTALL_DIR}/eigenmemory (${VERSION})"
case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *) echo "note: ${INSTALL_DIR} is not on your PATH; add it to your shell profile" >&2 ;;
esac
echo "Next: cd into your project and run 'eigenmemory init'"