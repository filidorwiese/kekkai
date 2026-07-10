# Quickstart: Pre-Build Node Version Existence Check

**Feature**: 017-prebuild-node-check

E2E validation per constitution IV, against the live nodejs.org index and real docker builds. Contract: [contracts/node-existence-check.md](contracts/node-existence-check.md).

## Prerequisites

- Docker daemon; network to nodejs.org; Go toolchain per `go.mod`.
- Build: `go build -o kekkai-test ./cmd/kekkai`
- Empty test dir; pseudo-TTY trick only needed for live-sandbox scenarios (none here — all scenarios end at build/abort).

## Scenario 1 — nonexistent versions abort pre-build (SC-001)

For each of `"99"`, `"22.99"`, `"22.99.0"` in `.kekkai.yaml` `image.node_version`:

1. Snapshot `docker images -q | wc -l`.
2. `kekkai up` → exits non-zero within ~15s with:
   `image.node_version: "<value>" matches no published Node version — see https://nodejs.org/dist/ for available versions`
3. Image count unchanged; no build output appeared.

## Scenario 2 — valid selectors proceed; lts skips the check (SC-002)

1. `node_version: "22"` → `kekkai up` starts a build ("building image ..." appears). Abort the build early (Ctrl+C equivalent) or let it run — the check passing is proven by the build starting.
2. `node_version: lts` with the index made unreachable for the probe (see Scenario 3 technique) → build still starts instantly: proves `lts` never consults the index.

## Scenario 3 — inconclusive check proceeds silently (SC-003)

Technique: make the probe's HTTPS egress fail without touching the daemon: run kekkai with `https_proxy=http://127.0.0.1:9` (dead proxy) — the probe errors instantly; `docker build` uses the daemon and is unaffected.

1. `https_proxy=http://127.0.0.1:9 kekkai up` with `node_version: "22"` → no check-related output, build starts normally.
2. Added latency ≈ 0 (immediate connection refusal counts as inconclusive).

Note: the same env would break `claude.version: latest` resolution (also via HTTPS) — pin `claude.version` in the test config to isolate the probe.

## Scenario 4 — native error fallback, no wrapper (SC-005)

1. Rendered-recipe check: `grep -n 'ERROR: Node version' embed/Dockerfile.tmpl` → no match.
2. `https_proxy=http://127.0.0.1:9 kekkai up` with `node_version: "99"` (check inconclusive → build runs) → build fails at the nvm install step; output contains nvm's native `Version '99' not found` once and NO `check image.node_version` echo.

## Scenario 5 — cached image makes zero requests (SC-004)

1. Complete one successful build (e.g. finish Scenario 2's `lts` build).
2. Run `kekkai up` again (image cached): sandbox starts with no index request — verifiable via the dead-proxy env: `https_proxy=http://127.0.0.1:9` + pinned claude version → still instant (nothing tries the network before the run).

## Sign-off checklist

- [ ] SC-001 all three nonexistent shapes abort pre-build with contract text (Scenario 1)
- [ ] SC-002 valid + lts proceed; lts provably check-free (Scenario 2)
- [ ] SC-003 dead-proxy probe → silent proceed (Scenario 3)
- [ ] SC-004 cached-image up: zero index requests (Scenario 5)
- [ ] SC-005 no wrapper in template; single native nvm error on fallback (Scenario 4)
