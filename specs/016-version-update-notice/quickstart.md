# Quickstart: Update Notice on `kekkai version`

**Feature**: 016-version-update-notice

E2E validation per constitution IV, against the real GitHub releases API. Contract: [contracts/version-notice.md](contracts/version-notice.md).

## Prerequisites

- Network access to api.github.com; Go toolchain satisfying `go.mod`.
- No docker needed (host-side CLI feature).

## Scenario 1 — stale binary shows the yellow notice (SC-001, SC-004)

1. `go build -ldflags "-X main.version=v0.0.1" -o kekkai-old ./cmd/kekkai`
2. `./kekkai-old version` (in a terminal) →
   - line 1 (stdout): `v0.0.1`
   - line 2 (stderr): `A new version of kekkai is available (v0.0.1 -> v<latest>), run 'kekkai self-update' to upgrade`, in yellow.
3. Byte-identity with the `up` surface: the notice string matches specs/005's pinned text (same `Notice()` producer — verify by reading the string once from both surfaces or asserting against the contract text).

## Scenario 2 — stdout stays parseable (SC-002)

1. `v=$(./kekkai-old version 2>/dev/null); echo "[$v]"` → `[v0.0.1]` exactly.
2. `./kekkai-old version 2>&1 >/dev/null | head -1` → the advisory line (proves stderr routing).
3. `NO_COLOR=1 ./kekkai-old version` in a terminal → advisory present, no ANSI escapes; piped stderr (non-terminal) → also plain.

## Scenario 3 — silent paths (SC-003, SC-005)

1. Dev build: `go build -o kekkai-dev ./cmd/kekkai && ./kekkai-dev version` → exactly `dev`, no stderr, instant (no lookup).
2. Current build: stamp with the actual latest release tag → version only, no advisory.
3. Lookup failure: `KEKKAI_REPO=filidorwiese/does-not-exist ./kekkai-old version` → `v0.0.1` only, exit 0, no error text (404 exercises the same silent-failure branch as offline).
4. Timing: failure and dev cases complete near-instantly; success case within the existing 10s cap.

## Scenario 4 — regression guard on the up surface

1. `kekkai up` with the stale binary in a test dir still prints the same notice once before the interactive handoff (unchanged behavior, helper rename only).

## Sign-off checklist

- [ ] SC-001 stale binary: version + yellow advisory naming latest and self-update (Scenario 1)
- [ ] SC-002 `$(kekkai version)` exact in all cases; advisory on stderr only (Scenario 2)
- [ ] SC-003 offline/404: silent, exit 0, bounded time (Scenario 3)
- [ ] SC-004 notice text byte-identical across `version` and `up` (Scenarios 1, 4)
- [ ] SC-005 dev/current builds byte-identical to pre-feature output (Scenario 3)
