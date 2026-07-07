# Quickstart Validation: Node Version Config

End-to-end validation per constitution IV (real docker daemon, no mocks).

## Prerequisites

- Docker daemon running, network access to Docker Hub
- Built binary: `go build -o kekkai ./cmd/kekkai`
- Throwaway project dir: `mkdir -p /tmp/kekkai-qs && cd /tmp/kekkai-qs && git init -q`

## Scenario 1 — default lts (US1)

```sh
../kekkai init          # or path to built binary
grep node_version .kekkai.yaml          # expect: node_version: lts ; no base_image anywhere
../kekkai up
```

Inside sandbox: `node --version` → current LTS major. Exit; `kekkai down`.

## Scenario 2 — pinned version + version change rebuild (US1, FR-008)

```sh
sed -i 's/node_version: lts/node_version: 22/' .kekkai.yaml
../kekkai up            # builds new image, node --version → v22.x
```

Exit. Re-run `../kekkai up` unchanged → no rebuild (same `kekkai:<hash>`). Change to
`node_version: 24` → rebuild with new hash. Verify via `docker images 'kekkai:*'`.

## Scenario 3 — legacy key migration error (US2)

```sh
printf 'image:\n  base_image: node:24-trixie\n' > .kekkai.yaml
../kekkai up
```

Expect: exit 1, one-pass validation report containing
`"image.base_image" is now image.node_version`. No docker activity
(`docker ps -a` unchanged).

## Scenario 4 — malformed selector caught pre-docker (US3, SC-004)

```sh
printf 'image:\n  node_version: "node:24"\n' > .kekkai.yaml
../kekkai up            # error names the offending value; no docker calls
printf 'image:\n  node_version: ""\n' > .kekkai.yaml
../kekkai up            # explicit-empty error, not silent default
```

## Scenario 5 — pre-check: confirmed-missing tag (US3, SC-005)

```sh
printf 'image:\n  node_version: "99.99"\n' > .kekkai.yaml
docker image rm node:99.99-trixie 2>/dev/null || true
../kekkai up
```

Expect: fast failure BEFORE any `docker build`/pull, error naming `99.99` and
`node:99.99-trixie` (contract table in [contracts/config.md](contracts/config.md)).

## Scenario 6 — offline with local base image (edge case)

```sh
printf 'image:\n  node_version: "22"\n' > .kekkai.yaml
docker pull node:22-trixie              # ensure local
# simulate registry outage, e.g. temporarily point registry-1.docker.io to 127.0.0.1 in /etc/hosts
../kekkai up                            # must start: check skipped, local base used
```

Restore `/etc/hosts` afterwards.

## Scenario 7 — docs swept clean (FR-007)

```sh
grep -rn base_image README.md SPECIFICATION.md docs/ internal/runtime/init.go
```

Expect: no hits describing it as a live key (only the `legacyKeys` migration entry in
`internal/config/config.go` remains, plus this feature's spec artifacts).

## Cleanup

`../kekkai down; ../kekkai prune` (if available); `rm -rf /tmp/kekkai-qs`.
