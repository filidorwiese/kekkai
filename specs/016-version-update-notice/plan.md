# Implementation Plan: Update Notice on `kekkai version`

**Branch**: `016-version-update-notice` | **Date**: 2026-07-10 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/016-version-update-notice/spec.md`

## Summary

Give the existing update notice a second surface: `kekkai version` prints the version on stdout (first, unconditionally), then — when `selfupdate.Notice()` returns a message — one yellow advisory line on stderr. The message definition, release lookup, silent-failure policy, and 10s bound are all the existing specs/005 machinery; the only new code is the call site in `main.go` plus exporting the shared `yellow` styling helper so the convention cannot diverge.

## Technical Context

**Language/Version**: Go (single static binary, stdlib only)

**Primary Dependencies**: existing `internal/selfupdate.Notice()` (GitHub latest-release lookup, 10s timeout, silent on all failures); existing `yellow()` advisory helper in `internal/runtime/up.go`

**Storage**: N/A

**Testing**: E2E per constitution IV: run stamped binaries against the real GitHub API (`KEKKAI_REPO` override available as the specs/003 testing hook); capture-stdout and NO_COLOR checks

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (host CLI)

**Project Type**: CLI

**Performance Goals**: `kekkai version` unchanged for dev builds (no lookup); stamped builds bounded by the existing 10s lookup cap, version line printed before the lookup starts

**Constraints**: stdout contract frozen (version string only); advisory on stderr; byte-identical notice text to `up`; no new lookup mechanism, no caching

**Scale/Scope**: 3 files: `cmd/kekkai/main.go`, `internal/runtime/up.go` (export helper), `SPECIFICATION.md`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
|---|---|---|
| I. Spec-First | `SPECIFICATION.md` §3 (`version` line + notice paragraph) updated in the same commit | PASS (planned) |
| II. Container Is the Security Boundary | Host-side CLI only; no container/firewall involvement | PASS |
| III. Minimal Surface | No new subcommand/config/dependency; reuses `Notice()` and the yellow helper (one export rename). Net ~6 lines | PASS |
| IV. End-to-End Validation | Quickstart: stamped binary vs real releases API, stdout-capture check, offline check | PASS |
| Constraints (config/image rules) | Untouched | PASS |

**Post-Phase-1 re-check**: no violations; Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/016-version-update-notice/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── version-notice.md  # CLI output contract for `kekkai version`
└── tasks.md             # Phase 2 output (/speckit-tasks)
```

### Source Code (repository root)

```text
cmd/kekkai/
└── main.go                # "version" case: print version (stdout), then
                           #   selfupdate.Notice → runtime.Yellow on stderr

internal/runtime/
└── up.go                  # yellow() → exported Yellow() (single advisory
                           #   styling point; both up-callers renamed)

SPECIFICATION.md           # §3: version command line + notice second surface
```

**Structure Decision**: Existing layout; edits only. README needs no change (it does not document notice behavior).

## Design Decisions (from research)

1. **Print order**: version line first (stdout, unbuffered), then the blocking `Notice()` lookup — the primary answer is never delayed by the courtesy check (FR-001, edge case "slow lookup").
2. **Stream split**: advisory → stderr via the shared helper (`runtime.Yellow(os.Stderr, msg)`); matches the missing-config advisory convention and keeps `$(kekkai version)` clean (FR-005). Deliberate difference from `up` (which prints its notice on stdout mid-interactive-flow): on `version`, stdout is the machine-readable surface.
3. **Helper export, not duplication**: `yellow()` in `up.go` is documented as the single point of the advisory convention — export it (`Yellow`) rather than copying the ANSI/`NO_COLOR`/terminal logic into `main.go` (FR-003/FR-004 "cannot diverge").
4. **No async**: `up` races the check against docker work; `version` has nothing to overlap with — synchronous call, silent empty-string fast paths (dev build returns before any network).

## Complexity Tracking

> No constitution violations — table intentionally empty.
