# Implementation Plan: Mirror the host project path inside the sandbox

**Branch**: `026-mirror-host-path` | **Date**: 2026-09-09 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/026-mirror-host-path/spec.md`

## Summary

Replace the fixed `/workspace` bind with a mirror of the host project path: `P = EvalSymlinks(Getwd())` becomes the bind source *and* destination, the `-w` of `docker run`, `docker exec -w` for `shell`/`exec`, and the base for the config, `.git` and secret-shadow binds. Claude Code then keys per-project state by the same path on host and in sandbox, ending cross-project bleed of memory, sessions, trust and grants. The path is a runtime input only: the Dockerfile loses `/workspace` and gets a neutral `WORKDIR /home/kekkai`, so one image still serves every project. Host-side validation rejects `/`, ancestors of kekkai's own in-container dirs, and paths with `:`/control chars; macOS preflight probes the mirrored destination. Old `-workspace` state is left orphaned. Research: [research.md](research.md).

## Technical Context

**Language/Version**: Go (stdlib only), as today

**Primary Dependencies**: none new; docker CLI `run -w`, `exec -w` (API ≥ 1.35; host has 29.x)

**Storage**: none new. Existing `~/.claude` bind, per-project history volume (name hash now of the resolved path)

**Testing**: end-to-end per constitution IV — [quickstart.md](quickstart.md), 22 scenarios; `go build` + `go vet`; `grep` for leftover `/workspace`

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 hosts; container unchanged

**Project Type**: CLI shelling out to docker; mount/arg assembly change, template edit, spec + docs amendments

**Performance Goals**: no change to startup path; validation is pure string work, zero docker calls

**Constraints**: project path never enters the image (constitution constraint, FR-006); no new config key or flag (Principle III); no new sudo/caps (Principle II); §9.6 probes untouched; `SPECIFICATION.md` amended in the same commit (Principle I)

**Scale/Scope**: ~9 files. Modified: `internal/runtime/identity.go` (ProjectDir helper + validation), `internal/runtime/up.go` (binds, shadows, `-w`), `internal/runtime/shell.go`, `internal/runtime/exec.go` (`-w P`), `internal/runtime/down.go`, `traffic.go`, `mpr.go`, `init.go` (use helper; init comment wording), `internal/runtime/preflight_darwin.go` (probe dst), `internal/runtime/sandboxprompt.go` (two lines), `embed/Dockerfile.tmpl` (mkdir/chown/WORKDIR), `SPECIFICATION.md` (§5.2, §5.3 prompt note, §6.3, §7.1, §7.3, §7.4, §8), `README.md` (upgrade note). New: `contracts/sandbox-layout.md`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | Same commit amends `SPECIFICATION.md`: §5.2 (bind table with `P`), §6.3 (no `/workspace`, `WORKDIR /home/kekkai`), §7.1 (identity from resolved path), §7.3 (`-w P`), §7.4 (probe `P:P:ro`), §8 (shadow paths under `P`), §4.4 (path validation). README gets the upgrade note. | PASS |
| II. Container Is the Security Boundary | No new caps, sudo, sockets or destinations. Read-only config and `.git` binds and secret shadows keep identical semantics under the new base path (verified by quickstart 7–10). Validation prevents the bind from overlaying kekkai's own tooling (`/usr/local/bin`, `/home/kekkai`). Parent dirs docker creates are root-owned; the sandbox user cannot write outside `P`. | PASS |
| III. Minimal Surface | No config key, no flag, no dependency. One helper replaces seven `os.Getwd()` call sites; protected-path list and rejected charset are code constants. | PASS |
| IV. End-to-End Validation | quickstart.md: real-daemon scenarios for isolation, host sharing, symlink, every workspace-relative bind, subcommands, history, image reuse, unusual paths, rejections, macOS unshared folder. §9.6 probes unchanged. | PASS |
| Constraint: image hash from bake-time inputs only | `P` is never rendered; template edit is a one-time rebuild like any other. Two projects share one tag (quickstart 15). | PASS |
| Constraint: one config file, strict parse, one-pass validation | No schema change; the new path checks join the one-pass report in `up`. | PASS |
| §11 out-of-scope list | Untouched (no multiple sandboxes per folder, no per-project Dockerfile). | PASS |

**Post-design re-check**: unchanged after Phase 1. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/026-mirror-host-path/
├── plan.md              # This file
├── research.md          # Phase 0: R1–R9 decisions
├── data-model.md        # Phase 1: ProjectDir, validation, bind table, state ownership
├── quickstart.md        # Phase 1: e2e validation scenarios
├── contracts/
│   └── sandbox-layout.md  # run/exec args, validation errors, preflight, image, prompt lines, upgrade note
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/kekkai/main.go                 # unchanged (dispatch)
internal/runtime/
├── identity.go                    # + ProjectDir(), + ValidateProjectPath(), protected list, charset
├── up.go                          # binds/shadows/-w under P; validation call after config validation
├── shell.go, exec.go              # ProjectDir(); docker exec -w P
├── down.go, traffic.go, mpr.go    # ProjectDir() for label lookup
├── init.go                        # ProjectDir(); template comment "project root"
├── preflight_darwin.go            # probe dst P
└── sandboxprompt.go               # two wording lines
embed/Dockerfile.tmpl              # drop /workspace mkdir+chown; WORKDIR /home/kekkai
SPECIFICATION.md, README.md        # amendments per contract
```

**Structure Decision**: existing layout; identity concerns stay in `identity.go` per §7.1 ("any change updates every consumer").

## Design notes (from research)

- **Single path helper (R2)**: `ProjectDir()` resolves symlinks once; label, names, binds, `-w` and all lookups use it. Symlinked invocations get a new container/volume name once; documented.
- **Validation (R3, R4)**: runs in `up` after config validation, before the existing-container check; errors join the one-pass report. Rejected: `/`, equal-or-ancestor of `/home/kekkai`, `/usr/local/bin`, `/commandhistory`; any `:` or byte < 0x20. Everything else, including spaces and unicode, passes.
- **Mount syntax (R3)**: keep `-v`; the colon rule is what makes `-v` unambiguous.
- **Image (R5)**: `WORKDIR /home/kekkai` is a fallback for exec paths without `-w`; the real cwd is always passed at runtime.
- **Preflight (R6)**: only the project bind's destination changes; failure format and hint table reused.
- **Prompt (R7)**: two lines reworded; contract pins the new text and supersedes the 011 lines.
- **Host sharing nuance (R1)**: memory, sessions and `settings.local.json` are host-shared; the `.claude.json` projects map (trust, MCP, prompt history) is per-project but sandbox-side unless the host sets `CLAUDE_CONFIG_DIR=~/.claude`. Spec story 2 amended accordingly.

## Complexity Tracking

No constitution violations; table not needed.
