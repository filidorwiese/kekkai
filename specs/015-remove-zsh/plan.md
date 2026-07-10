# Implementation Plan: Single-Shell Sandbox (bash) and Release Build Repair

**Branch**: `015-remove-zsh` | **Date**: 2026-07-10 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/015-remove-zsh/spec.md`

## Summary

Two independent repairs bundled: (1) the release workflow pins `go-version: "1.24"` while `go.mod` requires `go 1.25.0` and setup-go@v6 defaults `GOTOOLCHAIN=local` — switch to `go-version-file: go.mod` so the module file is the single toolchain source; (2) remove zsh from the sandbox: bash becomes the interactive shell (`kekkai shell`), login shell, and `$SHELL`, with history persistence rewired from `.zshrc`/`INC_APPEND_HISTORY` to `.bashrc`/`PROMPT_COMMAND='history -a'` on the same per-project volume; drop the dead `POWERLEVEL9K_DISABLE_GITSTATUS` env injection.

## Technical Context

**Language/Version**: Go (single static binary; `go.mod` requires 1.25.0 — the CI mismatch is the P1 bug)

**Primary Dependencies**: docker CLI (shelled out), GitHub Actions `setup-go@v6` (release pipeline); in-image: bash (already baked since feature 014)

**Storage**: Per-project docker volume `kekkai-history-<hash>` mounted at `/commandhistory` (unchanged identity; file inside changes to `.bash_history`)

**Testing**: End-to-end per constitution IV (quickstart scenarios against real docker); CI fix verified by toolchain resolution + local matrix-equivalent builds

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (release matrix); linux containers (sandbox)

**Project Type**: CLI (single Go binary)

**Performance Goals**: No new docker calls; image shrinks (zsh + zsh-common removed, ~20MB)

**Constraints**: Image hash from bake-time inputs only (rebuild expected on upgrade — Dockerfile changes); firewall/sudoers/CMD behavior byte-identical apart from the shell swap; history volume identity unchanged

**Scale/Scope**: 6 files touched (`.github/workflows/release.yml`, `embed/Dockerfile.tmpl`, `internal/runtime/up.go`, `internal/runtime/shell.go`, `cmd/kekkai/main.go`, `SPECIFICATION.md`) + `README.md`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
|---|---|---|
| I. Spec-First | `SPECIFICATION.md` §3 (CLI help), §5.1, §5.3, §6.3 updated in the same commit as the code; README digest follows | PASS (planned) |
| II. Container Is the Security Boundary | No boundary change: sudoers block, firewall script, CMD firewall gating untouched; only the interactive shell and one env var change | PASS |
| III. Minimal Surface | Net removal: one apt package, one env injection, one shell story. No new config keys, deps, or subcommands | PASS |
| IV. End-to-End Validation | Quickstart scenarios: shell/history e2e against real docker; firewall probes unaffected; CI fix validated via toolchain resolution behavior | PASS |
| Constraint: image hash = bake-time inputs | Dockerfile change → new hash → rebuild on next `up` (correct, same as every image change); history volume is runtime, identity untouched | PASS |

**Post-Phase-1 re-check**: no violations; Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/015-remove-zsh/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   ├── shell.md         # in-sandbox shell contract (bash, history, $SHELL)
│   └── ci-toolchain.md  # release workflow toolchain contract
└── tasks.md             # Phase 2 output (/speckit-tasks)
```

### Source Code (repository root)

```text
.github/workflows/
└── release.yml            # go-version: "1.24" → go-version-file: go.mod

embed/
└── Dockerfile.tmpl        # useradd -s /bin/bash; history block → .bashrc
                           #   (.bash_history, PROMPT_COMMAND='history -a');
                           #   ENV SHELL=/bin/bash

internal/runtime/
├── up.go                  # drop "zsh" from builtinAptPackages;
│                          #   drop POWERLEVEL9K_DISABLE_GITSTATUS injection
└── shell.go               # docker exec ... zsh → bash (comment + call)

cmd/kekkai/
└── main.go                # help text: "open zsh" → "open bash"

SPECIFICATION.md           # §3 help line, §5.1 apt set, §5.3 env list,
                           #   §6.3 user shell + history lines
README.md                  # "opens zsh" → "opens bash"
```

**Structure Decision**: Existing layout; edits only, no new files.

## Design Decisions (from research)

1. **CI**: `go-version-file: go.mod` replaces the pinned `go-version: "1.24"` — the module file becomes the only toolchain declaration (FR-001, no future drift). `GOTOOLCHAIN=local` default stays: correct once the installed toolchain matches.
2. **History**: `.bashrc` appended block — `HISTFILE=/commandhistory/.bash_history`, `HISTSIZE=10000`, `HISTFILESIZE=10000`, `PROMPT_COMMAND='history -a'` (bash's incremental-append equivalent of zsh `INC_APPEND_HISTORY`, FR-005). Old volumes: bash creates the missing `.bash_history` on first write (volume dir is kekkai-owned); orphaned `.zsh_history` stays untouched.
3. **Shell surfaces**: `useradd -s /bin/bash`, `ENV SHELL=/bin/bash`, `kekkai shell` → `docker exec -it <ctr> bash`. Debian skel `.bashrc` already exists for the created user; kekkai's block appends after it (same pattern as the old `.zshrc` append).
4. **p10k var**: injection removed from `up.go`; not made a reserved key (users may still set it via `env`).

## Complexity Tracking

> No constitution violations — table intentionally empty.
