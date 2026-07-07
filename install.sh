#!/bin/sh
# kekkai installer — curl-pipe from the repo (§10):
#   curl -fsSL https://raw.githubusercontent.com/OWNER/kekkai/main/install.sh | sh
# Override the version with KEKKAI_VERSION=vX.Y.Z.
set -eu

REPO="${KEKKAI_REPO:-filidorwiese/kekkai}"
INSTALL_DIR="${KEKKAI_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
    Linux)
        OS=linux
        case "$(uname -m)" in
            x86_64|amd64) ARCH=amd64 ;;
            aarch64|arm64) ARCH=arm64 ;;
            *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
        esac
        ;;
    Darwin)
        OS=darwin
        case "$(uname -m)" in
            arm64) ARCH=arm64 ;;
            *) echo "kekkai supports Apple silicon Macs only (Intel Macs are unsupported)" >&2; exit 1 ;;
        esac
        ;;
    *) echo "kekkai supports Linux and macOS only (got $(uname -s))" >&2; exit 1 ;;
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

TARBALL="kekkai_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "downloading kekkai ${VERSION} (${OS}/${ARCH})"
curl -fsSL "${BASE}/${TARBALL}" -o "${TMP}/${TARBALL}"
curl -fsSL "${BASE}/SHA256SUMS" -o "${TMP}/SHA256SUMS"

# macOS ships shasum, not sha256sum
if command -v sha256sum >/dev/null 2>&1; then
    SHA256="sha256sum"
else
    SHA256="shasum -a 256"
fi

cd "$TMP"
grep " ${TARBALL}\$" SHA256SUMS | $SHA256 -c - >/dev/null || {
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
