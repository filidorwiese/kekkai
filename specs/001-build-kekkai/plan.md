# Implementation Plan: Build Kekkai

**Branch**: `001-build-kekkai` | **Date**: 2026-07-06 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-build-kekkai/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Build kekkai end-to-end per `SPECIFICATION.md`: one static Go binary that runs Claude Code
autonomously inside a per-project Docker sandbox — strict-validated `.kekkai.yaml` config,
content-hashed image builds, label-keyed ephemeral containers, deny-by-default egress firewall
with mandatory self-verification, secrets shadowing, git/ssh/env/limits controls, lifecycle
subcommands, and tag-driven release distribution. Technical approach: stdlib-first Go shelling
out to the docker CLI, with the Dockerfile template and firewall script embedded in the binary.

## Technical Context

**Language/Version**: Go 1.24 (static binary, `CGO_ENABLED=0`)

**Primary Dependencies**: `gopkg.in/yaml.v3` (strict config parsing) — only external Go module.
Runtime: docker CLI on host; inside image: iptables/ipset/dig/curl/jq/aggregate (§5.1).

**Storage**: none — state lives in docker (container labels `kekkai.cwd` etc., named history
volumes, `kekkai:<hash>` images)

**Testing**: end-to-end against a real docker daemon (constitution IV); `go build` + `go vet`
as gates; no unit-test suite of consequence. Validation scenarios in [quickstart.md](quickstart.md).

**Target Platform**: linux/amd64 + linux/arm64 hosts with Docker, git, curl

**Project Type**: single CLI tool

**Performance Goals**: SC-001 — warm `up` to Claude exec < 15 s; first build < 2 min

**Constraints**: image hash from bake-time inputs only (§6.1); firewall config via env, never
bind mounts (§9); no docker socket in sandbox, no extra sudoers grants (§2, §6.3); one sandbox
per directory (§7)

**Scale/Scope**: single developer machine, handful of concurrent project sandboxes; 8
subcommands; ~10 config keys; 2 embedded assets

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Spec-First | All design artifacts derive from SPECIFICATION.md; any divergence discovered during implementation lands in SPECIFICATION.md in the same commit | PASS — plan/research/contracts cite spec sections; no divergence introduced |
| II | Container Is the Security Boundary | No docker socket in sandbox; single sudoers grant (firewall script only); probes never skipped; new destinations only via user config; §11 exclusions untouched | PASS — design keeps firewall script embedded + env-driven; no boundary-crossing features planned |
| III | Minimal Surface | Stdlib `flag`; exactly one external module (yaml.v3); no new subcommands/config keys beyond §3/§4.2; code constants for defaults | PASS — structure below adds no surface beyond spec |
| IV | End-to-End Validation | Validation = real docker daemon runs per quickstart.md; firewall probes part of every startup | PASS — quickstart.md defines the e2e matrix; no mock-docker layer planned |

**Post-Phase-1 re-check**: PASS — contracts and data model introduce no new dependencies,
subcommands, or boundary exceptions.

## Project Structure

### Documentation (this feature)

```text
specs/001-build-kekkai/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── cli.md           # Subcommand/flag/exit-code contract
│   ├── config.md        # .kekkai.yaml schema + validation contract
│   └── sandbox.md       # Container/image/firewall runtime contract
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/kekkai/
└── main.go              # flag parsing + dispatch only; version via -ldflags

internal/config/
├── config.go            # schema structs, load (yml/yaml discovery), expansion (§4.3)
└── validate.go          # strict + semantic validation, multi-error report (§4.4)

internal/runtime/
├── identity.go          # container name/label/volume derivation (§7.1) — single source
├── init.go              # kekkai init (§4.5)
├── up.go                # validate → resolve version → build-if-missing → run args → exec (§6, §7)
├── down.go              # stop/remove by kekkai.cwd label
├── shell.go             # exec zsh in running container
├── ps.go                # list by kekkai labels
└── prune.go             # orphans + unused images (+ --volumes), confirm unless --yes

internal/docker/
├── cli.go               # docker CLI invocation helpers (inspect/build/run/ps/rm)
└── exec.go              # interactive exec with SIGINT/SIGTERM forwarding (§7.2)

embed/
├── Dockerfile.tmpl      # rendered with base image, apt packages, claude version (§6)
└── init-firewall.sh     # deny-by-default egress + verification probes (§9)

install.sh               # curl-pipe installer (§10)
.github/workflows/release.yml  # v* tag → matrix build → tarballs + SHA256SUMS → release
```

**Structure Decision**: single Go module at repo root, exactly the layout SPECIFICATION.md §3
prescribes (`cmd/kekkai` dispatch, logic per subcommand in `internal/runtime/<name>.go`).
`internal/config` and `internal/docker` keep parsing and daemon plumbing out of the runtime
logic; `embed/` assets compile into the binary via `go:embed` so the distributed artifact stays
a single static file.

## Complexity Tracking

> No constitution violations — table intentionally empty.
