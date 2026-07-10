# Research: Update Notice on `kekkai version`

**Feature**: 016-version-update-notice | **Date**: 2026-07-10

Sources: `cmd/kekkai/main.go`, `internal/selfupdate/selfupdate.go`, `internal/runtime/up.go`, `SPECIFICATION.md` §3, specs/005 + specs/008 contracts.

## R1. Reuse point for the message and lookup

- **Decision**: Call the existing `selfupdate.Notice(version)` from the `version` case in `main.go`.
- **Rationale**: `Notice()` already implements everything FR-002/FR-003/FR-006/FR-007 require: dev-build fast path (no network), same `latestTag` source as `self-update` (10s HTTP timeout), strict `CompareVersions == -1` gate, empty string on any failure, and the exact message text `A new version of kekkai is available (<from> -> <to>), run 'kekkai self-update' to upgrade`. Zero new lookup or message code.
- **Alternatives considered**: A `version`-specific message ("you have X, latest is Y") — violates FR-003 (one message, two surfaces) and specs/005's pinned string contract.

## R2. Output streams and ordering

- **Decision**: `fmt.Println(version)` first (stdout, as today), then if `Notice()` returns non-empty, print it to **stderr**.
- **Rationale**: FR-001/FR-005: stdout is the parsed surface (`$(kekkai version)`); the advisory is a diagnostic → stderr, same stream the §4.1 missing-config advisory uses. Printing the version before the lookup means the primary answer appears instantly; the process then lingers ≤10s worst-case doing the courtesy check — acceptable per spec assumption, and instant on dev builds (no lookup at all).
- **Alternatives considered**:
  - stdout for the advisory (like `up`'s notice) — breaks `$(kekkai version)`; `up`'s stdout choice predates a machine-readable contract and is pinned by specs/005 for that surface only.
  - Async with early exit — nothing to overlap with on `version`; complexity for no wall-clock gain (the process must wait for the check either way).

## R3. Yellow styling without duplicating the convention

- **Decision**: Export the `yellow` helper in `internal/runtime/up.go` as `Yellow(f *os.File, msg string)`; `main.go` calls `runtime.Yellow(os.Stderr, notice)`. The two existing in-package callers rename.
- **Rationale**: The helper's own doc comment says both advisory lines go through it "so the convention cannot diverge" — a third copy in `main.go` would be exactly that divergence. It already handles the FR-004 gates: ANSI yellow only when the target file is a terminal (`ModeCharDevice`) and `NO_COLOR` is unset. `main.go` already imports `internal/runtime`.
- **Alternatives considered**:
  - New tiny package (`internal/term`) for the helper — a package for one 8-line function; Minimal Surface says no.
  - Duplicate the ANSI logic in `main.go` — the divergence the helper exists to prevent.

## R4. E2E test strategy for "stale binary"

- **Decision**: Build with `-ldflags "-X main.version=v0.0.1"` (any version below the real latest release of filidorwiese/kekkai) and run `kekkai version`; assert stdout == `v0.0.1` and stderr contains the notice. `KEKKAI_REPO` env (specs/003 hook, honored by `repoSlug()`) stays available for fork testing but is not needed against the real repo.
- **Rationale**: Real API, real release data (constitution IV). A v0.0.1 stamp is guaranteed stale against any published release.
- **Alternatives considered**: Mock server — violates the e2e principle; `KEKKAI_REPO` fork — needless indirection when the real repo works read-only.

## R5. Offline behavior verification

- **Decision**: Validate the silent-failure path by pointing `KEKKAI_REPO` at a non-existent repo (immediate 404 → silent) and, for the network-absent case, running with DNS/network unavailable if practical; assert version-only output, exit 0.
- **Rationale**: FR-006/SC-003. A 404 exercises the identical error-swallowing path (`latestTag` error → empty notice) without needing to sever host networking.
- **Alternatives considered**: `unshare -n` network namespace — needs privileges; the 404 path covers the same code branch.
