# Implementation Plan: Yellow Update Notice

**Branch**: `008-yellow-update-notice` | **Date**: 2026-07-08 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/008-yellow-update-notice/spec.md`

## Summary

Render the `kekkai up` update notice in the feature-006 advisory yellow when
stdout is a terminal and `NO_COLOR` is unset. The inline coloring in
`warnNoConfig` is extracted into one tiny helper (`yellow(f, msg)`) used by
both advisories, guaranteeing the identical color and gating (FR-004). Text,
stream (stdout), position, and silence conditions untouched. One code file
(`internal/runtime/up.go`) plus the SPECIFICATION.md §3 update-notice line.

## Technical Context

**Language/Version**: Go (stdlib), existing toolchain

**Primary Dependencies**: none new (TTY check stays `os.File.Stat()` char-device mode)

**Storage**: N/A

**Testing**: End-to-end per constitution IV — ldflags-versioned builds against the real binary (feature 005 technique)

**Target Platform**: linux + darwin/arm64 (unchanged)

**Project Type**: CLI

**Performance Goals**: None — one extra Stat call on the notice path

**Constraints**: Notice text/stream/timing frozen (FR-003); identical yellow + gating for both advisories (FR-004)

**Scale/Scope**: ~10 lines net in internal/runtime/up.go + one SPECIFICATION.md line

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Spec-First**: SPECIFICATION.md §3 update-notice bullet amended in the
  same commit (FR-005). PASS
- **II. Container is the security boundary**: Rendering-only; no boundary
  contact. PASS
- **III. Minimal Surface**: No new dependencies/flags/keys; net code shrinks
  by deduplicating the color logic into one unexported helper. PASS
- **IV. End-to-End Validation**: quickstart runs ldflags-versioned real
  binaries under a TTY and piped. PASS

Post-design re-check: unchanged, PASS.

## Project Structure

### Documentation (this feature)

```text
specs/008-yellow-update-notice/
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── yellow-notice-cli.md
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/runtime/up.go        # new helper yellow(f *os.File, msg string);
                              # warnNoConfig uses it (stderr); notice print
                              # uses it (stdout)
SPECIFICATION.md              # §3 update-notice line gains yellow rendering note
```

**Structure Decision**: Existing layout. The helper lives unexported in
`internal/runtime` (both call sites are there); `selfupdate.Notice` keeps
returning plain text — color is presentation, applied at the print site.

## Complexity Tracking

No constitution violations; table not needed.
