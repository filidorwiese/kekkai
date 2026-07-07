# Contract: Distribution (release artifacts + installer)

## Release artifacts (`.github/workflows/release.yml`)

Matrix becomes explicit os/arch pairs; naming pattern unchanged:

| GOOS | GOARCH | Artifact |
|---|---|---|
| linux | amd64 | `kekkai_${VERSION}_linux_amd64.tar.gz` |
| linux | arm64 | `kekkai_${VERSION}_linux_arm64.tar.gz` |
| darwin | arm64 | `kekkai_${VERSION}_darwin_arm64.tar.gz` (NEW) |

All three listed in the single `SHA256SUMS`. Build: `CGO_ENABLED=0`, cross-compiled from ubuntu-latest, `-trimpath -ldflags "-s -w -X main.version=${TAG}"` (unchanged). No darwin/amd64 artifact (spec clarification Q4). Binaries unsigned/un-notarized (research.md R7).

## Installer (`install.sh`)

| `uname -s` | `uname -m` | Behavior |
|---|---|---|
| Linux | x86_64/amd64 | linux_amd64 (unchanged) |
| Linux | aarch64/arm64 | linux_arm64 (unchanged) |
| Darwin | arm64 | darwin_arm64 (NEW) |
| Darwin | x86_64 | refuse: "kekkai supports Apple silicon Macs only (Intel Macs are unsupported)" — exit 1 |
| other | — | refuse (unchanged wording updated: "Linux and macOS only") |

- Checksum: use `sha256sum` if on PATH, else `shasum -a 256` (macOS default). Verification failure remains fatal.
- Install dir `~/.local/bin`, `KEKKAI_VERSION`/`KEKKAI_REPO`/`KEKKAI_INSTALL_DIR` overrides: unchanged.

## Documentation contract (README digest)

- Prerequisites: "Linux x86_64/aarch64, or macOS on Apple silicon. Docker (on macOS: Docker Desktop, OrbStack, colima, or any Docker-compatible runtime), git, curl."
- Support tiering stated verbatim-equivalent: Docker Desktop is maintainer-validated each release; other runtimes are supported via capability preflight and community-validated.
- Known limitations gains: macOS shared-folder I/O is slower; on macOS the sandbox can reach Mac services including those bound to localhost (unlike Linux); `git.ssh_agent` needs the runtime to forward the agent into its VM (colima: `--ssh-agent`).
