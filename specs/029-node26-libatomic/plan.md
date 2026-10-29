# Implementation Plan: Node 26 LTS runs in the sandbox image

**Branch**: `029-node26-libatomic` | **Date**: 2026-10-29 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/029-node26-libatomic/spec.md`

## Summary

Node 26 became the active LTS in late October 2026. Its Linux binaries (x64 and arm64) link `libatomic.so.1`, which `debian:trixie` does not ship, so `nvm install --lts` unpacks a node that cannot start: `nvm current` prints `none`, the default alias and the `$NVM_DIR/current` symlink are never created, and the next layer fails with `npm: command not found` disguised as the claude-install retry message. Fix: add `libatomic1` to the builtin apt package set in `internal/runtime/up.go`. That string enters both the image tag and the `kekkai.config_hash` label, so every cached image is invalidated once and the offline fallback can never pick a pre-fix image. Research: [research.md](research.md).

## Technical Context

**Language/Version**: Go (stdlib only), as today; Dockerfile template unchanged

**Primary Dependencies**: one Debian main package, `libatomic1` (45 KB installed, depends on libc6 + gcc base); available for amd64 and arm64 from the mirrors the build already uses

**Storage**: none

**Testing**: end-to-end per constitution IV — [quickstart.md](quickstart.md): builds with `lts`, `26`, `24`, `22`; startup line; `node -v`/`npm -v` in-sandbox; one-rebuild-then-cached check; `kekkai traffic` shows no new destinations

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (container side identical: Debian trixie image, arm64 node binary has the same dependency)

**Project Type**: CLI shelling out to docker; one list entry, one spec amendment

**Performance Goals**: no change to startup path; build gains one tiny package in the existing apt layer

**Constraints**: builtin set only, no config key (Principle III, FR-004); no new sudo/caps/egress (Principle II, FR-006); bake-time change deliberately invalidates images (constitution constraint satisfied in the intended direction, FR-005); `SPECIFICATION.md` §5.1 amended in the same commit (Principle I, FR-007)

**Scale/Scope**: 2 files. Modified: `internal/runtime/up.go` (one slice entry + why-comment), `SPECIFICATION.md` (§5.1). Unchanged: `embed/Dockerfile.tmpl`, `internal/runtime/identity.go` (hash functions already cover apt packages), `init.go` starter, README.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | Same commit amends §5.1 builtin apt packages with a new "Required — node runtime" bullet naming `libatomic1` and the Node 26 dependency. Contract: [contracts/builtin-packages.md](contracts/builtin-packages.md). | PASS |
| II. Container Is the Security Boundary | A shared library in the image; no capability, sudo grant, mount, or firewall input. §5.4 builtin destinations untouched; apt fetch is build-time on the daemon's host network like every other package. | PASS |
| III. Minimal Surface | One entry in an existing list. Rejected: a user-facing config key, a conditional install keyed on node version, and a base-image switch (research R2). | PASS |
| IV. End-to-End Validation | quickstart builds the four selectors against the real daemon, verifies the startup line and in-sandbox node/npm, the single rebuild, and §9.6 probes still run at startup. macOS deferred to the same tiering as prior features. | PASS |
| Constraint: image hash from bake-time inputs only | The package list is a bake-time input by definition (§6.1); the rebuild is intended and bounded to one per configuration. | PASS |
| Constraint: one config file, strict parse | No schema change. | PASS |
| §11 out-of-scope list | Untouched. | PASS |

**Post-design re-check**: unchanged after Phase 1. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/029-node26-libatomic/
├── plan.md                      # This file
├── research.md                  # Phase 0: R1–R5, verified on host
├── data-model.md                # Phase 1: bake inputs and hash flow
├── quickstart.md                # Phase 1: e2e validation scenarios
├── contracts/
│   └── builtin-packages.md      # package-set contract, hash consequence, spec amendment text
└── tasks.md                     # Phase 2 (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/runtime/up.go   # builtinAptPackages: add "libatomic1" under a "node runtime" comment
SPECIFICATION.md         # §5.1 builtin apt packages
```

**Structure Decision**: existing layout; the package set lives in `builtinAptPackages` and flows into `renderDockerfile` and `ConfigHash` unchanged.

## Design notes (from research)

- **Where the failure actually is (R1)**: not nvm, not the pre-build existence check, not the claude install step. The binary is present but unloadable; nvm's `nvm_ls_current` runs `node --version`, gets nothing, prints `none`. The claude step's error text is misleading only because `npm` is on the broken `current` path.
- **Why a baked package and not a conditional (R2)**: the image must be identical regardless of runtime config (§5.1 rule); the package is 45 KB; a conditional on `node_version` would make the apt layer depend on the selector for no gain.
- **Hash consequences (R3)**: `builtinAptPackages` is joined into both the rendered Dockerfile (→ image tag) and `ConfigHash` (→ fallback label). One rebuild for everyone; pre-fix images are unreachable via the offline fallback. No migration code.
- **Ordering (R4)**: place after the scripting-runtime group with its own comment so the list keeps its by-purpose grouping; position changes the hash string but that is already invalidated.
- **Placement in the Dockerfile (R5)**: the single apt layer runs before the nvm steps, so the library exists before `nvm install` runs `node --version`. No template change.
- **Agent context**: no `update-agent-context` script in `.specify/scripts/bash/`; step skipped (consistent with prior features).

## Complexity Tracking

No violations.
