# Implementation Plan: Node Version Config

**Branch**: `004-node-version-config` | **Date**: 2026-07-07 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/004-node-version-config/spec.md`

## Summary

Replace `image.base_image` with `image.node_version` (default `lts`). Kekkai resolves
the value internally to `node:<version>-trixie` (Debian release pinned as a code
constant). Old key becomes a legacy key with a targeted migration error. Before a build
that would pull the base image, a best-effort Docker Hub manifest check confirms the tag
exists — confirmed-missing fails fast naming the configured version; unreachable
registry skips the check. README.md, SPECIFICATION.md, `kekkai init` starter, and docs
example config drop all `base_image` references.

## Technical Context

**Language/Version**: Go 1.24 (static binary, stdlib `flag`)

**Primary Dependencies**: `gopkg.in/yaml.v3` (only external dep); stdlib `net/http` for registry check (same pattern as npm `latest` resolution in `internal/runtime/up.go`)

**Storage**: N/A (single `.kekkai.yaml` config file, strict-parsed)

**Testing**: End-to-end against a real docker daemon per constitution IV; `go build ./...` + `go vet`; existing `_test.go` conventions where present

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64

**Project Type**: CLI tool (single Go module, `./cmd/kekkai`)

**Performance Goals**: Registry pre-check adds ≤1 token + 1 manifest round-trip (10s timeout, same as npm resolve), only on the build path when base image not local

**Constraints**: Image hash derives from bake-time inputs only (§6.1) — resolved base-image string keeps feeding `ConfigHash`/rendered Dockerfile unchanged; runtime config never triggers rebuild; all config violations reported in one pass before docker work

**Scale/Scope**: ~5 Go files touched, 2 docs, 1 embedded starter config, 1 docs example

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment | Status |
|-----------|-----------|--------|
| I. Spec-First | SPECIFICATION.md §4.2, §4.4, §4.5, §6.1 updated in the same commit as code (explicit FR-007) | PASS |
| II. Container Is the Security Boundary | No boundary change. Registry pre-check runs on the HOST (like npm resolve), not in the sandbox; no new sandbox egress | PASS |
| III. Minimal Surface | Replaces one config key with a narrower one; no new deps (stdlib HTTP); Debian release stays a code constant, not config | PASS |
| IV. End-to-End Validation | quickstart.md defines real-docker scenarios: default lts, pinned version, legacy-key error, missing-tag pre-check error | PASS |

**Post-Phase-1 re-check**: PASS — design adds one helper (`ResolvedBaseImage`), one validation rule, one pre-check function; no new subcommands, no new config sections.

## Project Structure

### Documentation (this feature)

```text
specs/004-node-version-config/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── config.md        # image section contract delta
└── tasks.md             # Phase 2 (/speckit-tasks - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/config/
├── config.go            # ImageConfig.NodeVersion, DefaultNodeVersion, debian suffix
│                        #   constant, ResolvedBaseImage(), legacyKeys += base_image,
│                        #   imageKeysSet (explicit-empty detection)
└── validate.go          # node_version format rule replaces base_image rule

internal/runtime/
├── up.go                # ensureImage: use ResolvedBaseImage(); pre-build registry
│                        #   existence check (skip if base image local / registry down)
└── init.go              # starterConfig: node_version: lts

docs/my-project/.kekkai.yaml   # example config: node_version
README.md                       # config example + prose
SPECIFICATION.md                # §4.2 schema, §4.4 validation, §4.5 init, §6.1 bake inputs
```

**Structure Decision**: Existing single-module layout; no new packages. The registry
check lives in `internal/runtime` next to `resolveLatest()` (same host-side HTTP
pattern, same timeout policy).

## Complexity Tracking

No constitution violations — table not needed.
