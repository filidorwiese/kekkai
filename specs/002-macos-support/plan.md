# Implementation Plan: macOS Support

**Branch**: `002-macos-support` | **Date**: 2026-07-07 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/002-macos-support/spec.md`

## Summary

Add darwin/arm64 as a release target and make `kekkai up` work on Apple silicon Macs with any Docker-compatible runtime. Containers already run in a Linux VM on macOS, so the image pipeline, firewall, and caps are unchanged; the delta is host-side: a capability preflight (one throwaway container run probing bind-ability and, when configured, the VM agent socket), a darwin-specific `ssh_agent` mount path, one builtin firewall domain (`host.docker.internal`) for Mac-host parity, installer/release-matrix additions, and docs. Runtime identity (Docker Desktop / OrbStack / colima) is detected only to decorate preflight failures with fix-hints, never to gate.

## Technical Context

**Language/Version**: Go 1.24, stdlib `flag` only (no new deps)

**Primary Dependencies**: docker CLI (shell-out, unchanged), `gopkg.in/yaml.v3` (existing)

**Storage**: N/A (no persisted state beyond existing docker labels/volumes)

**Testing**: End-to-end against real daemons per constitution Principle IV; macOS scenarios in [quickstart.md](quickstart.md). Maintainer-validated on Docker Desktop only; OrbStack/colima community-validated (spec clarification Q5)

**Target Platform**: linux/amd64, linux/arm64, **darwin/arm64 (new)**; container side unchanged (Linux VM on macOS)

**Project Type**: single static CLI binary

**Performance Goals**: preflight adds ≤ ~2s to `kekkai up` on macOS (single short-lived container); zero overhead on Linux (preflight is darwin-gated)

**Constraints**: no new config keys; firewall script stays single-source with no platform branches (host.docker.internal resolves on macOS runtimes, warn+skips on Linux); image-hash inputs change once (script edit) → one-time rebuild on upgrade

**Scale/Scope**: ~5 files touched in Go (~150 LOC), 1 firewall line, installer + release workflow, README/SPECIFICATION/constitution

## Constitution Check

*Constitution v1.0.0 at plan time; amended to v1.1.0 as part of this feature (see below).*

| Gate | Verdict | Notes |
|---|---|---|
| I. Spec-First | PASS | SPECIFICATION.md amended in this planning session (§1, §2, §4.4, §5.2, §5.4, §7.4, §9, §10); code follows in same feature branch |
| II. Container Is the Security Boundary | PASS | No boundary punctures. `host.docker.internal` builtin allowance mirrors the existing builtin bridge-subnet allowance (Linux host parity, spec clarification Q3); firewall verification untouched; no socket, no sudo changes. macOS was removed from §11 via prior discussion (clarify session 2026-07-07) |
| III. Minimal Surface | PASS after amendment | Adds darwin/arm64 target — requires constitution amendment (done, v1.1.0). No new subcommands, config keys, or dependencies; preflight is one new file + one firewall line |
| IV. End-to-End Validation | PASS | quickstart.md defines macOS e2e scenarios; Docker Desktop maintainer-validated per release, firewall probes still run every startup on all platforms |

**Post-design re-check**: no new violations introduced by Phase 1 artifacts. Complexity Tracking stays empty.

## Project Structure

### Documentation (this feature)

```text
specs/002-macos-support/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output (macOS e2e validation guide)
├── contracts/
│   ├── preflight.md     # macOS preflight + error/hint contract
│   └── distribution.md  # release artifacts + installer contract
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
cmd/kekkai/main.go               # unchanged (dispatch)
internal/
├── config/                      # unchanged (validation is platform-neutral)
├── docker/
│   ├── cli.go                   # + Info() (server OS/name for identity hints)
│   └── exec.go                  # unchanged
└── runtime/
    ├── up.go                    # darwin: ssh_agent mount path + preflight call,
    │                            #   move $SSH_AUTH_SOCK check to linux-only
    ├── preflight_darwin.go      # NEW: capability probe + runtime hints (darwin-gated)
    └── ...                      # down/shell/ps/prune/init unchanged
embed/
└── init-firewall.sh             # + host.docker.internal builtin (warn tier)
install.sh                       # + Darwin arm64 case, Intel refusal, shasum fallback
.github/workflows/release.yml    # + darwin/arm64 matrix entry
README.md                        # support tiering + macOS limitations
SPECIFICATION.md                 # amended (this session)
.specify/memory/constitution.md  # amended to v1.1.0 (this session)
```

**Structure Decision**: existing single-binary layout; the only new file is the darwin preflight in `internal/runtime`. Platform gating via `runtime.GOOS` checks (not build tags) where behavior diverges in shared code paths, a build-tagged file for the probe itself.

## Complexity Tracking

No constitution violations to justify.
