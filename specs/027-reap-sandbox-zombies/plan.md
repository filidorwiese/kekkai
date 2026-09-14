# Implementation Plan: Reap orphaned processes inside the sandbox

**Branch**: `027-reap-sandbox-zombies` | **Date**: 2026-09-09 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/027-reap-sandbox-zombies/spec.md`

## Summary

`kekkai up` runs the sandbox without an init process and the image CMD ends in `exec claude`, so claude (node) is PID 1. Orphaned grandchildren of statusline/hook helpers re-parent to it and are never reaped, accumulating as `<defunct>` for the whole session. Fix: add `--init` to the fixed `docker run` prefix in `buildRunArgs` so Docker's bundled `docker-init` (tini) is PID 1. Verified on this host: orphans reaped, exit codes and signal-kill codes propagate, tini hands the tty foreground group to claude so Ctrl-C and SIGWINCH reach claude directly, `docker stop` is forwarded. No image, CMD, config or host-side signal-handling change. Research: [research.md](research.md).

## Technical Context

**Language/Version**: Go (stdlib only), as today

**Primary Dependencies**: none new; docker CLI `run --init` (Docker ≥ 1.13; host has 29.x; snap and Docker Desktop ship `docker-init`)

**Storage**: none

**Testing**: end-to-end per constitution IV — [quickstart.md](quickstart.md), 14 scenarios incl. a control run proving the repro; `go build` + `go vet`

**Target Platform**: linux/amd64, linux/arm64 (incl. snap Docker), darwin/arm64 Docker Desktop; container unchanged

**Project Type**: CLI shelling out to docker; one run-arg addition, spec amendment

**Performance Goals**: no change to startup path (tini adds one tiny process)

**Constraints**: no bake-time input change → image hash unchanged (constitution constraint, FR-005); no new config key/flag (Principle III); no new caps/sudo (Principle II); §9.6 probes untouched; `SPECIFICATION.md` §7.2/§7.3 amended in the same commit (Principle I)

**Scale/Scope**: 2 files. Modified: `internal/runtime/up.go` (one arg + comment), `SPECIFICATION.md` (§7.2, §7.3). New: this feature's docs. Unchanged: `internal/docker/exec.go`, `embed/Dockerfile.tmpl`, `preflight_darwin.go`, README.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | Same commit amends §7.2 (init process, signal/exit semantics) and §7.3 (`--rm -it --init` prefix). Contract: [contracts/lifecycle.md](contracts/lifecycle.md). | PASS |
| II. Container Is the Security Boundary | tini is inside the container, runs as the same user, adds no capability, mount, socket or sudo grant. Firewall verification unchanged. | PASS |
| III. Minimal Surface | One fixed argument; no config, flag, dependency or package. Rejected apt-baked tini and a shell-wrapper PID 1 (research R1). | PASS |
| IV. End-to-End Validation | quickstart: real-daemon scenarios for PID 1, zombie absence (sandbox and host view), a control run reproducing the defect, Ctrl-C, down, resize, exit codes, startup-failure path, mpr supervision, no rebuild, §9.6 probes. macOS deferred, same tiering as 026. | PASS |
| Constraint: image hash from bake-time inputs only | `--init` is a run arg; template untouched; quickstart 12 checks no rebuild. | PASS |
| Constraint: one config file, strict parse | No schema change. | PASS |
| §11 out-of-scope list | Untouched. | PASS |

**Post-design re-check**: unchanged after Phase 1. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/027-reap-sandbox-zombies/
├── plan.md              # This file
├── research.md          # Phase 0: R1–R6, all verified on host
├── data-model.md        # Phase 1: process tree before/after, event table
├── quickstart.md        # Phase 1: e2e validation scenarios
├── contracts/
│   └── lifecycle.md     # run-arg prefix, observable guarantees, spec amendments
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/runtime/up.go   # buildRunArgs: "run","--rm","-it","--init",… + why-comment
SPECIFICATION.md         # §7.2 lifecycle, §7.3 run args assembly
```

**Structure Decision**: existing layout; the fixed run prefix stays in `buildRunArgs` where `--rm -it` already live.

## Design notes (from research)

- **Why `--init` and not a baked reaper (R1)**: identical effect, zero image change, zero packages; the defect only manifests with node as PID 1, so the control run in quickstart must use the old binary or a node PID 1.
- **Signals (R3)**: tini `setpgid` + `tcsetpgrp` make claude the tty foreground group → Ctrl-C and SIGWINCH bypass tini entirely; SIGTERM from `docker stop` is forwarded. Host-side `docker.Interactive` untouched.
- **Exit codes (R2)**: tini returns child status / 128+signal; docker CLI already surfaces it.
- **CMD interaction (R4)**: `exec claude` keeps the foreground pgrp; mpr supervisor remains claude's child; pre-claude failures still exit non-zero through tini.
- **Missing docker-init (R5)**: daemon refuses creation with a message naming `docker-init`; satisfies FR-007 without a probe. Preflight not extended.
- **Agent context**: no `update-agent-context` script in `.specify/scripts/bash/`; step skipped (consistent with prior features).

## Complexity Tracking

No constitution violations; table not needed.
