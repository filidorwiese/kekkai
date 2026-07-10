# Quickstart: Single-Shell Sandbox (bash) and Release Build Repair

**Feature**: 015-remove-zsh

E2E validation per constitution IV. Contracts under test: [contracts/shell.md](contracts/shell.md), [contracts/ci-toolchain.md](contracts/ci-toolchain.md).

## Prerequisites

- Docker daemon; Go toolchain satisfying `go.mod` on the host.
- Build: `go build -o kekkai-test ./cmd/kekkai`
- Empty test dir; pseudo-TTY trick for live runs: `script -qec "kekkai-test up" /dev/null </dev/null &`, poll `docker ps` for the container.
- Cleanup per scenario: `docker rm -f <ctr>`, remove only this test dir's `kekkai-history-<hash>` volume.

## Scenario 1 — CI toolchain (SC-001 mechanism)

1. `grep -rn 'go-version' .github/` → exactly one hit: `go-version-file: go.mod` in `release.yml`.
2. For each matrix entry:
   `GOOS=linux GOARCH=amd64`, `GOOS=linux GOARCH=arm64`, `GOOS=darwin GOARCH=arm64`:
   `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=test" -o /dev/null ./cmd/kekkai` → all succeed.
3. `go version` satisfies the `go` directive in `go.mod`.
4. Full SC-001 (3/3 builds + published release) observed at the next real tag push.

## Scenario 2 — bash shell + no zsh (SC-002)

1. `kekkai up` (pseudo-TTY) → image rebuilds (bake inputs changed — expected once).
2. `docker exec <ctr> sh -c 'command -v zsh || echo NO-ZSH'` → `NO-ZSH`.
3. `docker exec <ctr> sh -c 'getent passwd kekkai | cut -d: -f7'` → `/bin/bash`.
4. `docker exec <ctr> sh -c 'echo $SHELL'` → `/bin/bash`.
5. `kekkai shell` (pty) → bash prompt in `/workspace`; `ls /home/kekkai/.zshrc` → absent.

## Scenario 3 — history persists across restarts (SC-003)

1. In `kekkai shell` (pty): run a marker command (e.g. `echo history-marker-42`), exit the shell.
2. `docker exec <ctr> cat /commandhistory/.bash_history` → contains the marker (incremental append: written before shell exit).
3. `kekkai down`, then `kekkai up` again (same dir), `kekkai shell` → `history | grep history-marker-42` → present.
4. Pre-existing-volume case: place a `.zsh_history` in the volume first → new session still works, `.zsh_history` untouched, `.bash_history` created.

## Scenario 4 — p10k var gone (SC-004)

1. `docker exec <ctr> sh -c 'env | grep -c POWERLEVEL || echo ABSENT'` → `ABSENT`.
2. With `env: {POWERLEVEL9K_DISABLE_GITSTATUS: "true"}` in `.kekkai.yaml`: restart → var present (user passthrough still works, not reserved).

## Scenario 5 — regression guard (SC-005, FR-007)

1. Startup output still prints `kekkai sandbox: node v<x.y.z>, claude <x.y.z>` and firewall probes pass (§9.6).
2. `docker exec <ctr> node -v`, `docker exec <ctr> sh -c 'node -v'`, and `kekkai exec node -v` → same version.
3. `docker exec <ctr> sh -c 'command -v claude'` → `/home/kekkai/.nvm/current/bin/claude`.
4. Image size: `docker images` → new image smaller than the previous feature-014 image (SC-006).

## Sign-off checklist

- [ ] SC-001 mechanism: single toolchain source, 3/3 local matrix builds (Scenario 1)
- [ ] SC-002 bash everywhere, zsh absent (Scenario 2)
- [ ] SC-003 history survives restart; old-volume case handled (Scenario 3)
- [ ] SC-004 p10k var absent; user passthrough intact (Scenario 4)
- [ ] SC-005/SC-006 exec paths + startup line + firewall unchanged; image smaller (Scenario 5)
