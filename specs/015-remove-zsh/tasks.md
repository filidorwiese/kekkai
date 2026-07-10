# Tasks: Single-Shell Sandbox (bash) and Release Build Repair

**Input**: Design documents from `/specs/015-remove-zsh/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/shell.md, contracts/ci-toolchain.md, quickstart.md

**Tests**: Per constitution IV, validation is end-to-end (real docker daemon; CI fix via local matrix-equivalent builds). Each story phase ends with its quickstart scenario(s).

**Organization**: Three fully independent stories (disjoint concerns: CI file, image/shell, env injection). Only shared file: `internal/runtime/up.go` (US2 and US3 touch different lines — sequential).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 (P1 releases build), US2 (P2 bash-only sandbox), US3 (P3 drop p10k var)

## Path Conventions

Single Go binary project: `cmd/`, `internal/`, `embed/`, `.github/` at repository root (see plan.md).

---

## Phase 1: Setup

**Purpose**: Confirm clean baseline; all changes are edits to existing files.

- [X] T001 Verify baseline: `go build ./...` succeeds, docker daemon responds (`docker version`), and host `go version` satisfies the `go` directive in `go.mod` (needed for US1 validation)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: None — the three stories share no prerequisite code. Proceed directly to story phases.

**Checkpoint**: n/a

---

## Phase 3: User Story 1 - Releases build again (Priority: P1) 🎯 MVP

**Goal**: Release workflow resolves its Go toolchain from `go.mod` (single source of truth); all three matrix builds compile.

**Independent Test**: quickstart.md Scenario 1 — single `go-version-file` hit in `.github/`, 3/3 local matrix-equivalent builds succeed.

- [X] T002 [P] [US1] In `.github/workflows/release.yml` setup-go step: replace `go-version: "1.24"` with `go-version-file: go.mod` (contracts/ci-toolchain.md; research.md R1 — keeps setup-go@v6's `GOTOOLCHAIN=local` default, now always satisfied)
- [X] T003 [US1] Validate per quickstart.md Scenario 1: `grep -rn 'go-version' .github/` → only the `go-version-file: go.mod` line; run all three matrix builds locally (`GOOS=linux GOARCH=amd64`, `GOOS=linux GOARCH=arm64`, `GOOS=darwin GOARCH=arm64` with `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=test" -o /dev/null ./cmd/kekkai`) → all succeed

**Checkpoint**: Release mechanism repaired (full SC-001 observed at next real tag).

---

## Phase 4: User Story 2 - Sandbox runs on a single shell: bash (Priority: P2)

**Goal**: zsh gone from the image; `kekkai shell` opens bash; login shell + `$SHELL` are bash; history persists across restarts on the same volume.

**Independent Test**: quickstart.md Scenarios 2–3 — `command -v zsh` fails in-sandbox, `kekkai shell` lands in bash, marker command survives sandbox restart.

- [X] T004 [P] [US2] In `embed/Dockerfile.tmpl`: `useradd ... -s /bin/zsh` → `-s /bin/bash`; rewrite the history block per contracts/shell.md — `touch /commandhistory/.bash_history`, append to `/home/kekkai/.bashrc`: `export HISTFILE=/commandhistory/.bash_history`, `export HISTSIZE=10000`, `export HISTFILESIZE=10000`, `export PROMPT_COMMAND='history -a'` (research.md R2 — incremental append, FR-005), chown `.bashrc`; `ENV SHELL=/bin/zsh` → `/bin/bash`; update block comments (bash history, §6.3)
- [X] T005 [US2] In `internal/runtime/up.go`: remove `"zsh"` from `builtinAptPackages` (research.md R6; comment for `bash` entry becomes "nvm dependency + kekkai shell")
- [X] T006 [P] [US2] In `internal/runtime/shell.go`: `docker.Interactive("exec", "-it", c.ID, "zsh")` → `"bash"`; update doc comment ("opens bash")
- [X] T007 [P] [US2] In `cmd/kekkai/main.go`: help text `shell       open zsh in the running sandbox for $PWD` → `open bash ...`
- [X] T008 [US2] E2E quickstart.md Scenarios 2–3: rebuild `kekkai-test`, pseudo-TTY `up` (image rebuilds once — bake inputs changed), verify NO-ZSH, passwd shell `/bin/bash`, `$SHELL=/bin/bash`, no `~/.zshrc`; `kekkai shell` marker command → appears in `/commandhistory/.bash_history` before exit and in `history` after `down`+`up`; old-volume case (pre-seed `.zsh_history` → untouched, `.bash_history` created); cleanup container + this test dir's history volume only

**Checkpoint**: Single-shell sandbox fully working.

---

## Phase 5: User Story 3 - Dead configuration is removed (Priority: P3)

**Goal**: kekkai no longer injects `POWERLEVEL9K_DISABLE_GITSTATUS`; user passthrough unaffected.

**Independent Test**: quickstart.md Scenario 4 — `env` in-sandbox has no POWERLEVEL var; explicit user `env:` entry still passes through.

**Note**: Touches `internal/runtime/up.go` like T005 — run after Phase 4 code tasks (no [P]).

- [X] T009 [US3] In `internal/runtime/up.go`: delete the `addEnv("POWERLEVEL9K_DISABLE_GITSTATUS", "true")` line (research.md R5; do NOT add the key to `reservedEnvKeys`)
- [X] T010 [US3] E2E quickstart.md Scenario 4: sandbox env contains no `POWERLEVEL` entry; with `env: {POWERLEVEL9K_DISABLE_GITSTATUS: "true"}` in `.kekkai.yaml` the var appears (passthrough, not reserved)

**Checkpoint**: All three stories independently done.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Spec-first compliance, README digest, full regression sign-off.

- [X] T011 In `SPECIFICATION.md`: §3 CLI help line (`shell` opens bash), §5.1 apt builtins (remove zsh entry; bash serves `kekkai shell` + nvm), §5.3 builtin env list (drop `POWERLEVEL9K_DISABLE_GITSTATUS=true`), §6.3 (user shell bash; history line → `/commandhistory/.bash_history` with incremental append). Constitution I: MUST land in the same commit as the code changes
- [X] T012 [P] In `README.md` line ~63: `kekkai shell       # opens zsh in the running sandbox` → `opens bash` (FR-008)
- [X] T013 E2E quickstart.md Scenario 5: startup line + firewall probes unchanged, node/npm/npx/claude exec paths pass (SC-005), new image smaller than feature-014 image (SC-006); complete the quickstart sign-off checklist

---

## Dependencies

```text
Phase 1 (T001)
  ├─► Phase 3 US1 (T002 → T003)                    🎯 MVP  — .github only
  ├─► Phase 4 US2 (T004 ∥ T006 ∥ T007, T005 → T008) — image/CLI files
  └─► Phase 5 US3 (T009 → T010)                     — after T005 (same file up.go)
Phases 3–5 ─► Phase 6 (T011 ∥ T012 → T013 last: sign-off)
```

- No foundational phase; stories are independent.
- Only ordering constraint: T009 after T005 (both edit `internal/runtime/up.go`).
- T008/T010/T013 (e2e) share the docker daemon — run sequentially.

## Parallel Execution Examples

- After T001: `T002` (release.yml) ∥ `T004` (Dockerfile.tmpl) ∥ `T006` (shell.go) ∥ `T007` (main.go) — four disjoint files.
- Phase 6: `T011` (SPECIFICATION.md) ∥ `T012` (README.md); `T013` strictly last.
- One combined image build serves T008, T010, T013 — batch the e2e passes on the same rebuilt sandbox where possible (US3's env check just needs the new binary, not a new image).

## Implementation Strategy

1. **MVP first**: T002–T003 unblocks releases (P1) — shippable alone.
2. **Single rebuild**: land all image-affecting edits (T004, T005) before running e2e so the expensive image build happens once for Scenarios 2–5.
3. **Spec-first gate**: T011 commits together with the code (constitution I); one commit series ending with T013 sign-off.
4. **Release note**: next `kekkai up` after upgrade rebuilds the image (bake inputs changed); per-project shell history resets once (zsh → bash file).
