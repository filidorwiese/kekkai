# Implementation Plan: OpenSSH Client in Sandbox Image

**Branch**: `019-openssh-client` | **Date**: 2026-08-18 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/019-openssh-client/spec.md`

## Summary

Add `openssh-client` to the builtin apt package set baked into every sandbox image, so SSH commit signing (`gpg.format ssh` + forwarded agent) and SSH git remotes work when `git.ssh_agent: true`. Unconditional bake — never keyed on runtime config — per constitution (image hash from bake-time inputs only) and existing precedent (`jq`/`aggregate` baked though only `allow_github` uses them). One-line code change in `builtinAptPackages` plus the mandatory SPECIFICATION.md §5.1 amendment in the same commit.

## Technical Context

**Language/Version**: Go (stdlib only), single static binary

**Primary Dependencies**: none new; Debian trixie apt (`openssh-client` from the base distro repo)

**Storage**: N/A

**Testing**: end-to-end per constitution IV — real `kekkai up`, real docker daemon; no unit suite

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (image is linux either way)

**Project Type**: CLI tool baking a Docker image from an embedded template

**Performance Goals**: no runtime impact; image grows ~5–6 MB installed (in line with existing convenience packages)

**Constraints**: image hash derives from bake-time inputs only; runtime config (`git.ssh_agent`) must never influence image content or trigger rebuilds; duplicate user `apt_packages` entry must stay harmless

**Scale/Scope**: one list entry in `internal/runtime/up.go`, one sentence in SPECIFICATION.md §5.1; no config schema, firewall, or mount changes

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | SPECIFICATION.md §5.1 builtin package list amended in same commit as code | PASS — planned as part of the change (FR-006) |
| II. Container Is the Security Boundary | No new network destinations, no sudo grants, no firewall change. `openssh-client` is a client binary; outbound SSH is still governed by the existing firewall (allowed hosts only). Not on the spec's out-of-scope list. | PASS |
| III. Minimal Surface | No new subcommand, config key, or dependency. One package added to an existing list; justification: `ssh_agent: true` already promises agent auth + signing, which is dead without it. | PASS |
| IV. End-to-End Validation | Validation via real `kekkai up` + signed commit inside sandbox (quickstart.md). Firewall probes untouched. | PASS |
| Constraint: image hash from bake-time inputs only | Package added to `builtinAptPackages` (a bake-time input). `ssh_agent` toggle never consulted at bake. Hash + `kekkai.config_hash` change once on upgrade — correct and intended. | PASS |

**Post-design re-check**: unchanged — design adds no scope beyond the package-list entry and spec amendment. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/019-openssh-client/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── package-set.md   # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit-tasks - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/runtime/up.go   # builtinAptPackages (line ~26): add "openssh-client"
SPECIFICATION.md         # §5.1 apt packages: add openssh-client with rationale
embed/Dockerfile.tmpl    # unchanged — already renders {{range .AptPackages}}
internal/config/*        # unchanged — apt_packages append semantics untouched
```

**Structure Decision**: existing single-binary layout; change confined to the builtin package list constant and the spec document. No new files or packages.

## Complexity Tracking

No constitution violations. Table not needed.
