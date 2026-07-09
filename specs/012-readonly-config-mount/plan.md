# Implementation Plan: Read-Only Config Mount

**Branch**: `012-readonly-config-mount` | **Date**: 2026-07-09 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/012-readonly-config-mount/spec.md`

## Summary

Close the config self-escalation path: expose `.kekkai.yaml` read-only at `/workspace/.kekkai.yaml` inside the sandbox (bind over the rw workspace mount), or a comment-only placeholder when no config exists, so the agent can read but never create/modify the policy that governs its own sandbox. Simultaneously drop `.kekkai.yml` as an accepted config name: `kekkai up` detects it, warns about the typo, and exits before any docker work; `kekkai init` refuses while it exists.

Approach: all changes live in `internal/config` (discovery) and `internal/runtime` (up mount assembly, init message). No new dependencies, no new config keys, no image changes.

## Technical Context

**Language/Version**: Go 1.25 (static binary, stdlib `flag`)

**Primary Dependencies**: `gopkg.in/yaml.v3` (existing); docker CLI invoked via `internal/docker`

**Storage**: files — `.kekkai.yaml` at workspace root; temp placeholder file under `os.MkdirTemp` for the no-config case

**Testing**: end-to-end against a real docker daemon (Constitution IV); `go vet` / `go build`; validation scenarios in quickstart.md

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64

**Project Type**: single CLI binary (`./cmd/kekkai`)

**Performance Goals**: no measurable `up` startup overhead (one extra `os.Stat`/`os.Lstat` + one temp-file write worst case)

**Constraints**: typo check must fire before any docker work; placeholder must not create anything in the workspace nor persist after exit; mount must not trigger image rebuild (runtime-only, §6.1 unaffected)

**Scale/Scope**: ~4 files touched (`config.go`, `up.go`, `init.go`, `SPECIFICATION.md`) + README; est. <100 LOC delta

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment | Status |
|-----------|------------|--------|
| I. Spec-First | SPECIFICATION.md §4.1, §4.5, §5.2 amended in the same commit as code; README "Configure" digest updated | PASS (planned, tasked) |
| II. Container Is the Security Boundary | Feature strengthens the boundary (agent cannot rewrite its own policy). No new holes: ro bind + no SYS_ADMIN means no remount. No firewall/destination changes | PASS |
| III. Minimal Surface | No new subcommand, config key, or dependency. One config file rule actually gets stricter (.yml dropped). Placeholder content is a code constant | PASS |
| IV. End-to-End Validation | quickstart.md defines e2e scenarios against real docker: read/write attempts in container, typo exit, host state checks | PASS |

**Post-Phase-1 re-check**: PASS — design introduces no violations; no Complexity Tracking entries needed.

## Project Structure

### Documentation (this feature)

```text
specs/012-readonly-config-mount/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── config-mount.md  # Phase 1 output: messages, mount specs, placeholder content
└── tasks.md             # Phase 2 output (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/
├── config/
│   └── config.go        # Discover(): drop .yml acceptance, add typo error
└── runtime/
    ├── up.go            # buildRunArgs(): config ro mount / placeholder mount; cleanup
    └── init.go          # Init(): split existing-config vs typo-file messages

SPECIFICATION.md          # §4.1 (single name + typo error), §4.5 (init), §5.2 (mounts)
README.md                 # Configure section: read-only in sandbox, .yaml only
```

**Structure Decision**: existing single-project layout; changes confined to `internal/config`, `internal/runtime`, and docs. No new packages.

## Design Decisions (from research.md)

1. **Typo detection lives in `config.Discover`** — returns a dedicated typo error whenever a `.kekkai.yml` directory entry exists (via `os.Lstat`, any file type), regardless of whether `.kekkai.yaml` also exists. All config consumers inherit the rule (FR-001); `up` gets it via `Load` before any docker work (FR-002/003). The old "both exist, remove one" error is deleted.
2. **Read-only config mount is a builtin mount** (§5.2/§7.3): appended immediately after the `$PWD:/workspace` bind in `buildRunArgs`. File-over-directory bind: `-v $PWD/.kekkai.yaml:/workspace/.kekkai.yaml:ro`. Without SYS_ADMIN the agent cannot remount or delete the mount point (FR-006).
3. **Placeholder = temp file on host, bind-mounted ro**: written to `os.MkdirTemp("", "kekkai-config-")` with the fixed comment line, mounted at the same target, removed after `docker.Interactive` returns (it waits; no exec-replace). Nothing touches the workspace; nothing persists (amended FR-005).
4. **`kekkai init`**: `.kekkai.yaml` present → existing "already exists" refusal; `.kekkai.yml` present → typo refusal telling the user to rename/remove (FR-008).
5. **Ordering interaction**: secrets shadows come later in §7.3 order, so `secrets.hide: [.kekkai.yaml]` still wins (later mount on same target). Documented, not blocked — hiding your own config is a legal, if odd, user choice.

## Complexity Tracking

No constitution violations; table intentionally empty.
