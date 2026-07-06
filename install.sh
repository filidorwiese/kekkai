#!/bin/sh
# kekkai installer — curl-pipe from the repo (§10):
#   curl -fsSL https://raw.githubusercontent.com/OWNER/kekkai/main/install.sh | sh
# Override the version with KEKKAI_VERSION=vX.Y.Z.
set -eu

REPO="${KEKKAI_REPO:-filidorwiese/kekkai}"
INSTALL_DIR="${KEKKAI_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
    Linux) ;;
    *) echo "kekkai supports Linux only (got $(uname -s))" >&2; exit 1 ;;
esac

case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

VERSION="${KEKKAI_VERSION:-}"
if [ -z "$VERSION" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | head -1 | cut -d'"' -f4)
fi
if [ -z "$VERSION" ]; then
    echo "could not determine the latest release of ${REPO}" >&2
    exit 1
fi

TARBALL="kekkai_${VERSION}_linux_${ARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "downloading kekkai ${VERSION} (linux/${ARCH})"
curl -fsSL "${BASE}/${TARBALL}" -o "${TMP}/${TARBALL}"
curl -fsSL "${BASE}/SHA256SUMS" -o "${TMP}/SHA256SUMS"

cd "$TMP"
grep " ${TARBALL}\$" SHA256SUMS | sha256sum -c - >/dev/null || {
    echo "checksum verification FAILED for ${TARBALL}" >&2
    exit 1
}
tar -xzf "$TARBALL"

mkdir -p "$INSTALL_DIR"
install -m 0755 kekkai "$INSTALL_DIR/kekkai"
echo "installed $("$INSTALL_DIR/kekkai" version) to $INSTALL_DIR/kekkai"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo "note: $INSTALL_DIR is not on your PATH — add it, e.g.:"
       echo "  export PATH=\"\$PATH:$INSTALL_DIR\"" ;;
esac
