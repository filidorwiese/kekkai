#!/usr/bin/env bash
set -euo pipefail

REPO="filidorwiese/kekkai"
DOCS_URL="https://github.com/${REPO}"
INSTALL_DIR="${HOME}/.local/bin"
BIN="kekkai"

err() { echo "kekkai-install: $*" >&2; exit 1; }

[ "$(uname -s)" = "Linux" ] || err "only linux is supported in v0.1.0 (got $(uname -s))"

arch="$(uname -m)"
case "$arch" in
    x86_64|amd64) goarch=amd64 ;;
    aarch64|arm64) goarch=arm64 ;;
    *) err "unsupported arch $arch (need x86_64 or aarch64)" ;;
esac

command -v curl >/dev/null 2>&1 || err "curl required"
command -v tar  >/dev/null 2>&1 || err "tar required"

version="${KEKKAI_VERSION:-}"
if [ -z "$version" ]; then
    echo "kekkai-install: resolving latest release tag…"
    version="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep -oE '"tag_name"[[:space:]]*:[[:space:]]*"[^"]+"' \
        | head -1 \
        | sed -E 's/.*"([^"]+)"$/\1/')"
    [ -n "$version" ] || err "could not resolve latest release tag from GitHub API"
fi
echo "kekkai-install: installing $version (linux-$goarch)"

tar_name="kekkai-${version}-linux-${goarch}.tar.gz"
tar_url="https://github.com/${REPO}/releases/download/${version}/${tar_name}"
sum_url="https://github.com/${REPO}/releases/download/${version}/SHA256SUMS"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$tar_url" -o "$tmp/$tar_name"
curl -fsSL "$sum_url" -o "$tmp/SHA256SUMS" || echo "kekkai-install: warning: no SHA256SUMS found, skipping checksum verification"

if [ -f "$tmp/SHA256SUMS" ]; then
    (cd "$tmp" && grep " $tar_name\$" SHA256SUMS | sha256sum -c -) \
        || err "checksum verification failed"
fi

tar -xzf "$tmp/$tar_name" -C "$tmp"
[ -f "$tmp/$BIN" ] || err "extracted tarball does not contain '$BIN'"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$BIN" "$INSTALL_DIR/$BIN"

echo "kekkai-install: installed $INSTALL_DIR/$BIN"
echo "kekkai-install: docs: $DOCS_URL"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) cat <<EOF >&2

NOTE: $INSTALL_DIR is not on your PATH.
Add it to your shell rc, e.g.:
  echo 'export PATH="\$HOME/.local/bin:\$PATH"' >> ~/.zshrc
Then open a new shell.
EOF
        ;;
esac
