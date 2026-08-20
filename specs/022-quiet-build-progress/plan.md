# Implementation Plan: Quiet Interactive Build Progress Without Custom Apt Repos

**Branch**: `022-quiet-build-progress` | **Date**: 2026-08-20 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/022-quiet-build-progress/spec.md`

## Summary

`docker.BuildImage` gains a `capture bool` parameter. `capture=false` (config has no `apt_repos`): docker inherits `os.Stdout`/`os.Stderr` directly — BuildKit sees the real terminal and renders its compact TTY progress, restoring pre-020 behavior; the returned output string is empty. `capture=true` (≥1 repo): the existing `io.MultiWriter` tee stays, feeding the specs/020 signature hint unchanged. `buildImage` in `internal/runtime/up.go` passes `len(aptRepos) > 0`. Two files, no identity inputs touched, zero rebuilds.

## Technical Context

**Language/Version**: Go (stdlib only), single static binary — unchanged

**Primary Dependencies**: none new; no isatty/pty machinery — TTY detection stays delegated to the docker CLI

**Storage**: N/A

**Testing**: end-to-end per constitution IV — real builds observed through a pseudo-TTY (`script -qec`): renderer markers distinguish compact (`[+] Building` header, ANSI control codes) from plain (`#N [internal] load build definition` step lines); 020's S5 signature-hint scenario re-run as the non-regression gate

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 — behavior identical everywhere (docker CLI does the detection)

**Project Type**: CLI tool shelling out to the docker CLI

**Performance Goals**: none; output plumbing only

**Constraints**: no identity input changes (FR-005 — binary-only, zero rebuilds); 020 hint contract byte-for-byte (FR-003); `--verbose` forces plain in both modes (FR-004); no new config surface (FR-006)

**Scale/Scope**: 2 files: `internal/docker/cli.go` (BuildImage signature + conditional writers), `internal/runtime/up.go` (call site passes repo presence)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | SPECIFICATION.md needs no amendment: §6.3's hint clause already conditions on "apt_repos configured" and no spec text describes build-progress rendering or output capture — the observable contract is unchanged (research.md R4 verifies). | PASS |
| II. Container Is the Security Boundary | Output plumbing on the host side only; firewall, sudoers, image contents, destinations untouched. | PASS |
| III. Minimal Surface | No config keys, no flags, one bool parameter on an internal function. | PASS |
| IV. End-to-End Validation | quickstart.md: pty-observed renderer modes, 020 S5 hint re-run, verbose behavior, image-reuse check — all against a real daemon. | PASS |
| Constraint: image hash from bake-time inputs only | Nothing rendered or hashed changes; SC-003 explicitly verifies reuse. | PASS |

**Post-design re-check**: unchanged after Phase 1. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/022-quiet-build-progress/
├── plan.md                    # This file
├── research.md                # Phase 0 output
├── data-model.md              # Phase 1 output
├── quickstart.md              # Phase 1 output
├── contracts/
│   └── build-output.md        # Phase 1 output — BuildImage behavior in both modes
└── tasks.md                   # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/docker/cli.go     # BuildImage(tag, dir, labels, verbose, capture) — writers chosen by capture
internal/runtime/up.go     # buildImage passes len(aptRepos) > 0 as capture
```

**Structure Decision**: existing single-binary layout; a parameter thread through two files. The 020 design docs (contracts/image-render.md "Build output capture" section) remain the hint contract; this feature narrows only *when* capture happens, recorded here and cross-referenced from research.md.

## Complexity Tracking

No constitution violations. Table not needed.
