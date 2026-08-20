# Implementation Plan: Builtin Archive, File-Inspection, and Search Tools

**Branch**: `021-builtin-tools` | **Date**: 2026-08-20 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/021-builtin-tools/spec.md`

## Summary

Append nine packages (`unzip`, `zip`, `xz-utils`, `zstd`, `bzip2`, `file`, `ripgrep`, `fd-find`, `rsync`) to the `builtinAptPackages` constant as a new "general tooling" group. Debian installs fd's binary as `fdfind`, so `embed/Dockerfile.tmpl` gains one `ln -s /usr/bin/fdfind /usr/local/bin/fd` line in the existing root-owned symlink RUN block, satisfying the canonical-name requirement (FR-002). No other code changes: the package list already flows into the rendered Dockerfile (`ImageTag` changes → one-time rebuild, FR-004) and into `ConfigHash` via the existing aptPackages input (offline fallback never reuses pre-upgrade images). SPECIFICATION.md §5.1 (builtin list) and §6.3 (symlink bullet) amended in the same change.

## Technical Context

**Language/Version**: Go (stdlib only), single static binary — unchanged

**Primary Dependencies**: none new. In-image: 9 Debian packages, all in trixie main, verified installable; combined installed size ~25 MB incl. transitive deps (`libmagic-mgc` 10 MB is the largest) — within the SC-005 50 MB bound

**Storage**: N/A

**Testing**: end-to-end per constitution IV — real `kekkai up`, in-sandbox tool checks and archive round-trips via `kekkai exec`; quickstart.md scenarios

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (image is linux either way); all 9 packages exist for amd64 and arm64 in trixie

**Project Type**: CLI tool baking a Docker image from an embedded template

**Performance Goals**: no runtime impact; build time grows by one apt fetch of ~10 MB download

**Constraints**: bake-time inputs only (the builtin list is already one); exactly one rebuild per config on upgrade, then reuse; no new config surface; firewall/sudoers untouched

**Scale/Scope**: 3 files: `internal/runtime/up.go` (constant), `embed/Dockerfile.tmpl` (one symlink line), SPECIFICATION.md (§5.1, §6.3)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | SPECIFICATION.md §5.1 builtin list + §6.3 symlink bullet amended in the same change as the code (FR-006) | PASS — planned |
| II. Container Is the Security Boundary | No new network destinations (§5.4 untouched), no sudoers change, no daemons; packages fetched at build time on the host network like every existing builtin. Not on the §11 out-of-scope list. | PASS |
| III. Minimal Surface | Zero new config keys or subcommands; one constant extended, justified in spec (near-universal agent tools, same rationale as jq/less/nano). Explicit non-goals keep the heavy additions (build toolchain) out. | PASS |
| IV. End-to-End Validation | quickstart.md validates against a real daemon: tool presence, archive round-trips, one-rebuild-then-reuse, firewall probes. | PASS |
| Constraint: image hash from bake-time inputs only | The builtin list is already a bake input: it renders into the Dockerfile (`ImageTag`) and is an existing `ConfigHash` parameter. No identity code changes needed. | PASS |

**Post-design re-check**: unchanged after Phase 1 — design adds nothing beyond the constant, one template line, and docs. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/021-builtin-tools/
├── plan.md                    # This file
├── research.md                # Phase 0 output
├── data-model.md              # Phase 1 output
├── quickstart.md              # Phase 1 output
├── contracts/
│   └── image-tools.md         # Phase 1 output — package set, canonical names, identity contract
└── tasks.md                   # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/runtime/up.go     # builtinAptPackages: new "general tooling" group (9 packages)
embed/Dockerfile.tmpl      # root symlink RUN block: + ln -s /usr/bin/fdfind /usr/local/bin/fd
SPECIFICATION.md           # §5.1 builtin list, §6.3 symlink bullet
```

**Structure Decision**: existing single-binary layout; the feature is a constant extension plus one template line, flowing through the established render → hash pipeline. No new packages, files, or config surface.

## Complexity Tracking

No constitution violations. Table not needed.
