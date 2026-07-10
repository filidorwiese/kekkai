# Implementation Plan: Host UID/GID Match

**Branch**: `018-host-uid-gid` | **Date**: 2026-07-10 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/018-host-uid-gid/spec.md`

## Summary

The sandbox user is baked with hardcoded UID/GID 1000 (`embed/Dockerfile.tmpl:20-21`), so hosts whose primary gid differs (e.g. uid=1000, gid=1001) get workspace files with a foreign group. Fix: resolve the host uid/gid at `kekkai up` time, render them into the Dockerfile template, and let the existing rendered-Dockerfile image hash trigger per-identity rebuilds. Identities outside the safe range (root, system ids, macOS 501/20) fall back to the historical 1000/1000. `ConfigHash` gains the identity so the §6.2 offline fallback never reuses an image baked for a different identity.

## Technical Context

**Language/Version**: Go (stdlib only: `os.Getuid`/`os.Getgid`, `text/template`)

**Primary Dependencies**: Docker CLI (build/run), Debian trixie base image, existing template pipeline in `internal/runtime/up.go`

**Storage**: N/A (identity is derived per invocation, baked into image; no new persisted state)

**Testing**: End-to-end per constitution IV: real `kekkai up` on Linux, `stat` files on host, firewall probes. `go vet` / build. No unit-test suite of consequence.

**Target Platform**: linux/amd64, linux/arm64 primary; darwin/arm64 must be behaviorally unchanged (falls back to 1000/1000 since macOS uid 501/gid 20 are below the safe range)

**Project Type**: Single Go CLI (`./cmd/kekkai`) + embedded Dockerfile template

**Performance Goals**: No measurable impact on `kekkai up`; identity resolution is two syscalls

**Constraints**: Image hash derives from bake-time inputs only (§6.1) — identity is a bake-time input, so this holds; runtime config never triggers rebuild (unchanged)

**Scale/Scope**: ~4 files touched (`embed/Dockerfile.tmpl`, `internal/runtime/up.go`, `internal/runtime/identity.go`, `SPECIFICATION.md` §6.3), plus one new helper

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | `SPECIFICATION.md` §6.3 (currently states "UID/GID 1000") updated in the same commit as the code | PASS — plan includes the spec edit |
| II. Container Is the Security Boundary | No new sudo grants, no firewall change, no new network destinations; sudoers stays username-based (`kekkai`) and unchanged | PASS |
| III. Minimal Surface | No new subcommand, no new config key (identity is derived, never configured), no new dependency; one small helper function | PASS |
| IV. End-to-End Validation | Validation is a real `up` + host-side `stat` + firewall probes; macOS renders the fallback identity (semantically unchanged; one-time rebuild on upgrade) | PASS |

**Post-design re-check**: PASS — design adds two template fields, one helper, one signature change. No boundary or surface impact.

## Project Structure

### Documentation (this feature)

```text
specs/018-host-uid-gid/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── sandbox-identity.md
└── tasks.md             # Phase 2 output (/speckit-tasks - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
embed/
└── Dockerfile.tmpl          # groupadd/useradd lines take {{.Uid}}/{{.Gid}}; numeric chown

internal/runtime/
├── up.go                    # renderDockerfile signature + template data; ConfigHash call site
├── identity.go              # ConfigHash gains uid/gid inputs; new sandboxIdentity() helper
└── (no other runtime files) # exec/shell paths run as the image default user — untouched

SPECIFICATION.md             # §6.3 sandbox-user paragraph rewritten (spec-first, same commit)
```

**Structure Decision**: Existing single-binary layout; all changes live in the two files that already own image identity and rendering, plus the embedded template and the spec.

## Complexity Tracking

No constitution violations. Table not needed.
