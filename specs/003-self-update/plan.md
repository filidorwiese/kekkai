# Implementation Plan: Self-Update Command

**Branch**: `003-self-update` | **Date**: 2026-07-07 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/003-self-update/spec.md`

## Summary

Add `kekkai self-update`: query the latest GitHub release (same API endpoint install.sh uses), compare to the running version, and when newer, download the platform tarball, verify it against SHA256SUMS, and atomically replace the running executable. Reports "Updated kekkai vX -> vY" on success, "You're on the latest version (vX)" when current. Pure stdlib, host-side only, no sandbox or firewall impact.

## Technical Context

**Language/Version**: Go 1.24 (existing toolchain)

**Primary Dependencies**: stdlib only: net/http, encoding/json, crypto/sha256, archive/tar, compress/gzip, os, path/filepath. No new modules (yaml.v3 stays the only dep).

**Storage**: N/A (temp dir during download, deleted after)

**Testing**: End-to-end per constitution Principle IV: real GitHub releases (or a fork via `KEKKAI_REPO`), quickstart.md scenarios

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (identical code path; `runtime.GOOS`/`GOARCH` select the artifact)

**Project Type**: CLI subcommand

**Performance Goals**: up-to-date check < 5s (one API call); full update < 60s (SC-001/SC-003)

**Constraints**: atomic replace (rename within same directory); zero writes when up to date; checksum verification mandatory; no privilege escalation

**Scale/Scope**: ~200 LOC in one new file + ~10 LOC dispatch + spec/README amendments

## Constitution Check

*GATE: constitution v1.1.0*

| Gate | Verdict | Notes |
|---|---|---|
| I. Spec-First | PASS (conditional) | SPECIFICATION.md amendment required in the same commit as code: §3 add `self-update`, §10 add self-update bullet, §11 remove the `kekkai update` line. Exact delta below. |
| II. Container Is the Security Boundary | PASS | Host-side only; no sandbox, firewall, or mount changes. §11 listed the self-updater as out of scope: this feature request + spec 003 is the prior discussion §11 demands. Note: §11 entry was "deferred", not "rejected by threat model" (unlike docker socket), so removal is permitted. |
| III. Minimal Surface | PASS | New subcommand justified by spec 003 (FR-001). Zero new dependencies, zero new config keys. `KEKKAI_REPO` env override mirrors install.sh precedent (not config). |
| IV. End-to-End Validation | PASS | quickstart.md validates against real releases on a real host; no unit-test suite added. |

Post-design re-check: unchanged, all PASS.

### SPECIFICATION.md amendment delta (land with the code, not before)

- §3 CLI block: add `kekkai self-update # update the binary to the latest GitHub release` and bullet for output/refusal semantics.
- §10 Distribution: add bullet: self-update downloads the same release artifacts install.sh consumes, verifies SHA256SUMS, atomically replaces the running binary; dev builds refuse; `KEKKAI_REPO` honored.
- §11: delete the "`kekkai update` self-updater." line.
- README digest: add self-update to command list (task phase).

## Project Structure

### Documentation (this feature)

```text
specs/003-self-update/
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── self-update-cli.md
└── tasks.md             # /speckit-tasks output
```

### Source Code (repository root)

```text
cmd/kekkai/
└── main.go                       # +case "self-update", usage text

internal/selfupdate/
└── selfupdate.go                 # NEW: release lookup, semver compare,
                                  # download, verify, atomic replace

SPECIFICATION.md                  # §3, §10, §11 amendment (same commit)
README.md                         # digest update
```

**Structure Decision**: new `internal/selfupdate` package rather than `internal/runtime`: runtime is the docker/sandbox layer and self-update has zero docker involvement; keeps the boundary clean and the package independently readable (~200 LOC, one file).

## Design decisions (details in research.md)

- Release discovery: `GET https://api.github.com/repos/${KEKKAI_REPO:-filidorwiese/kekkai}/releases/latest`, parse `tag_name` (R1)
- Version compare: numeric semver on `v`-tags; equal or ahead → no-op success (R2)
- Artifact: `kekkai_${TAG}_${GOOS}_${GOARCH}.tar.gz` + `SHA256SUMS`, sha256 verified before extraction (R3)
- Atomic replace: temp file next to `os.Executable()` (resolved through symlinks), chmod 0755, `os.Rename` over the old binary; writability pre-checked before any download (R4)
- Dev guard: `version == "dev"` or non-`v` prefix refuses (R5)
- Exact user-facing strings pinned in contracts/self-update-cli.md

## Complexity Tracking

No violations. No new dependencies, no new config keys, one new package with one file.
