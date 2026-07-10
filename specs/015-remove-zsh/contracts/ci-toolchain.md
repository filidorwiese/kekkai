# Contract: Release workflow Go toolchain

**Feature**: 015-remove-zsh

## Single source of truth

- The Go toolchain requirement is declared in exactly one place: `go.mod` (`go <version>` directive).
- The release workflow (`.github/workflows/release.yml`) MUST resolve its toolchain from that file (`setup-go` with `go-version-file: go.mod`) and MUST NOT contain any literal Go version.
- `GOTOOLCHAIN=local` (setup-go@v6 default) is kept: builds use exactly the installed toolchain, which by construction satisfies the module.

## Behavior

| Situation | Outcome |
|---|---|
| Workflow runs at current `go.mod` (1.25.0) | all 3 matrix builds (linux/amd64, linux/arm64, darwin/arm64) compile |
| Future `go.mod` bump (e.g. 1.26) | CI follows automatically, zero workflow edits |
| Someone re-adds a pinned `go-version` | contract violation — review MUST reject |

## Validation (pre-release, no tag push)

- Local equivalent of each matrix build succeeds under a toolchain satisfying `go.mod`:
  `GOOS=<os> GOARCH=<arch> CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=test" -o /dev/null ./cmd/kekkai`
- `grep -r 'go-version' .github/` returns only the `go-version-file: go.mod` line.
- Full SC-001 (published release with tarballs + checksums) confirmed at the next real tag.
