# Implementation Plan: Optional Configuration File

**Branch**: `006-optional-config` | **Date**: 2026-07-08 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/006-optional-config/spec.md`

## Summary

Make `.kekkai.yaml` optional for `kekkai up`: absence means all built-in
defaults plus one yellow stderr warning naming `kekkai init`; a present but
empty or comments-only file also means all defaults (fixing today's
`parse ...: EOF` abort) with no warning. The `kekkai init` starter template
becomes fully commented out. README.md and SPECIFICATION.md drop every
"config required" statement. Three small code sites: `internal/config`
(empty-document handling + exported defaults constructor), `internal/runtime/up.go`
(missing-file fallback + warning), `internal/runtime/init.go` (template).

## Technical Context

**Language/Version**: Go (stdlib), existing toolchain

**Primary Dependencies**: `gopkg.in/yaml.v3` (already present); no new dependencies

**Storage**: N/A (no persistent state; kekkai stays stateless)

**Testing**: End-to-end per constitution IV — quickstart scenarios against the real binary

**Target Platform**: linux + darwin/arm64 (unchanged)

**Project Type**: CLI

**Performance Goals**: No measurable startup change; the fallback is a stat miss

**Constraints**: Warning on stderr only, yellow only when stderr is a terminal; exit status unchanged; no new config surface

**Scale/Scope**: ~40 lines of code across 3 files + template rewrite + 2 docs

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Spec-First**: SPECIFICATION.md §3, §4.1, §4.5 amended in the same commit
  as the code (FR-008). PASS
- **II. Container is the security boundary**: Defaults are the most restrictive
  posture (firewall on, nothing granted); allowing unconfigured runs grants no
  new capability. The builtin destination set is untouched. PASS
- **III. Minimal Surface**: No new dependencies (TTY check via
  `os.Stderr.Stat()` char-device mode, stdlib); no new flags or config keys;
  one new exported function in `internal/config`. PASS
- **IV. End-to-End Validation**: quickstart.md scenarios run the real binary
  against a real docker daemon. PASS

Post-design re-check: unchanged, PASS.

## Project Structure

### Documentation (this feature)

```text
specs/006-optional-config/
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── optional-config-cli.md
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/config/config.go     # Load: empty-document = defaults; Defaults() constructor;
                              # ErrNoConfig comment updated (no longer a user-facing error)
internal/runtime/up.go        # Up: ErrNoConfig -> warning + Defaults() instead of abort
internal/runtime/init.go      # starterConfig: every key commented out
README.md                     # config file now optional; warning documented
SPECIFICATION.md              # §3 (init line), §4.1 (Required -> optional + warning), §4.5 (starter)
```

**Structure Decision**: Existing layout; no new files or packages. The
missing-file decision stays at the `up` boundary (`internal/runtime/up.go`)
so `config.Load`/`Discover` semantics stay reusable and side-effect free.

## Complexity Tracking

No constitution violations; table not needed.
