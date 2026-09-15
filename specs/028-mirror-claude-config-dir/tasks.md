# Tasks: Mirror Claude Config Dir at Host Path

**Input**: Design documents from `specs/028-mirror-claude-config-dir/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/sandbox-layout.md, quickstart.md

**Tests**: No automated tests (constitution IV: end-to-end against a real daemon). Each story ends with a manual quickstart step.

**Organization**: Tasks grouped by user story. US1 alone is the MVP.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1, US2, US3 per spec.md

## Path Conventions

Single Go binary: `internal/runtime/`, `internal/config/`, `embed/`, docs at repo root.

---

## Phase 1: Setup

No project setup needed; existing layout, no new packages or dependencies.

- [X] T001 Build baseline binary with `task build` and record the current `docker images kekkai --format '{{.Tag}}'` output for the SC-005 comparison in quickstart step 6

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: single source for the config dir path, reused by every story.

- [X] T002 Add `ClaudeConfigDir() (string, error)` to `internal/runtime/identity.go`: `$CLAUDE_CONFIG_DIR` non-empty → `filepath.Abs(filepath.Clean(v))`, else `filepath.Join(os.UserHomeDir(), ".claude")`; no `EvalSymlinks` (research R1); doc comment explains why unresolved
- [X] T003 Generalize `ValidateProjectPath` in `internal/runtime/identity.go` into `validateMirrorPath(kind, p string) []error` with `kind` in the messages ("project path" / "claude config dir"); keep `ValidateProjectPath(p)` as a wrapper and add `ValidateClaudeConfigDir(p)` per contracts/sandbox-layout.md message table
- [X] T004 In `Up` in `internal/runtime/up.go`, after `ValidateProjectPath(pwd)`: resolve `ClaudeConfigDir()`, append `ValidateClaudeConfigDir(dir)` errors to the one-pass report; pass `dir` down to `preflight` and `buildRunArgs` (extend their signatures)

**Checkpoint**: `go build ./...` passes; behavior unchanged until Phase 3 rewires the mount.

---

## Phase 3: User Story 1 - Host plugins work in the sandbox (Priority: P1) 🎯 MVP

**Goal**: config dir mounted and addressed at its host path so stored absolute paths resolve.

**Independent Test**: quickstart steps 1 and 2 (plugin commands appear under `/`, `claude plugin list` matches host, absolute-path hook runs).

- [X] T005 [US1] In `buildRunArgs` in `internal/runtime/up.go`: replace `claudeDir := filepath.Join(home, ".claude")` with the passed `dir`; keep `os.MkdirAll(dir, 0o700)`; change the bind to `-v dir:dir` (rw); keep position (after `.kekkai.yaml` overlay, before history volume)
- [X] T006 [US1] In `buildRunArgs` in `internal/runtime/up.go`: `addEnv("CLAUDE_CONFIG_DIR", dir)` replaces the hardcoded `/home/kekkai/.claude` value; stays first in the builtin env block so user env may override (research R6)
- [X] T007 [US1] In `embed/Dockerfile.tmpl`: remove `/home/kekkai/.claude` from the `RUN mkdir -p /commandhistory /home/kekkai/.claude` line (research R4); keep the `chown -R` of `/home/kekkai`
- [X] T008 [US1] In `embed/Dockerfile.tmpl` CMD: prepend `if [ "$CLAUDE_CONFIG_DIR" != /home/kekkai/.claude ]; then ln -sfn "$CLAUDE_CONFIG_DIR" /home/kekkai/.claude; fi && ` before the startup `echo` (exact string in contracts/sandbox-layout.md); update the comment block above CMD to name the step
- [X] T009 [US1] Manual: `task build`, run quickstart steps 1 and 2; confirm `claude plugin list` parity and hook execution

**Checkpoint**: US1 deliverable. Sandbox loads host plugins.

---

## Phase 4: User Story 2 - Relocated host config dir is honored (Priority: P2)

**Goal**: `$CLAUDE_CONFIG_DIR` on the host is mirrored, probed, and protected from disk-mount shadowing.

**Independent Test**: quickstart steps 3 and 5 (relocated dir mounted at `/tmp/cc`, shadowing mount refused, `:` path refused).

- [X] T010 [P] [US2] In `preflight` in `internal/runtime/preflight_darwin.go`: replace `filepath.Join(home, ".claude")` probe source with the `dir` parameter from T004; destination `/kekkai-probe/claude:ro` unchanged (research R7)
- [X] T011 [P] [US2] In `Up` in `internal/runtime/up.go`, next to the T004 validation: for each `cfg.Disk.Mounts[i]` with `!m.Skip`, if `m.ContainerPath == dir` or `filepath.Rel(dir, m.ContainerPath)` yields a non-`..` relative path, append `disk.mounts[i]: target T would shadow the Claude config dir CFG` (research R3; `protectedContainerPaths` untouched)
- [X] T012 [US2] Manual: quickstart steps 3, 5, and 7 (relocated dir, refusals, project at `$HOME` still allowed with writable nested config dir)

**Checkpoint**: US2 deliverable.

---

## Phase 5: User Story 3 - `~/.claude` still resolves inside the sandbox (Priority: P3)

**Goal**: the symlink from T008 is verified and documented; prompt text reflects the layout.

**Independent Test**: quickstart step 4 (`readlink ~/.claude` prints the host path; listing shows host content).

- [X] T013 [P] [US3] In `internal/runtime/sandboxprompt.go`: replace the `- Filesystem:` bullet of `sandboxPrompt` with the text in contracts/sandbox-layout.md ("your Claude config directory (both mounted at the same path as on the host)")
- [X] T014 [P] [US3] In `docs/my-project/demo-sandbox.txt`: after `ls -ld ~/.claude` change the comment to note it is a symlink to the host path (e.g. `#> Sweet! A symlink to the same path as on the host`)
- [X] T015 [US3] Manual: quickstart step 4; also `kekkai shell` when host user is `kekkai` is out of reach, so verify the guard by `docker run` with `-e CLAUDE_CONFIG_DIR=/home/kekkai/.claude` and confirm no `ln` error in startup output

**Checkpoint**: US3 deliverable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: spec-first governance (constitution I) and final validation.

- [X] T016 [P] SPECIFICATION.md §5.2: replace the `~/.claude → /home/kekkai/.claude (rw)` bullet with the mirrored-path rule (resolution, rw, always on, nesting note, disk-mount refusal), reference `specs/028-mirror-claude-config-dir/contracts/sandbox-layout.md`
- [X] T017 [P] SPECIFICATION.md §5.3: change `CLAUDE_CONFIG_DIR=/home/kekkai/.claude` to `CLAUDE_CONFIG_DIR=<host config dir>` with the resolution rule; note the sandbox prompt filesystem line now names the config dir (supersedes 011 contract wording)
- [X] T018 [P] SPECIFICATION.md §6.3: document the CMD symlink step and the removed bake-time `.claude` mkdir; §7.1: add `ClaudeConfigDir` helper and `validateMirrorPath`; §7.3: add config dir + disk-mount target validation to the one-pass report; §7.4: probe list says "the resolved Claude config dir" instead of `~/.claude`
- [X] T019 [P] README.md: line ~35 mount paragraph ("everything in your Claude config dir, mounted at the same path as on the host, `~/.claude` stays a symlink"), line ~195 security note path wording, line ~203 refusal note gains "and the Claude config dir"
- [X] T020 Run quickstart step 6 (image tag parity across `CLAUDE_CONFIG_DIR` values, single rebuild only from the template change) and step 8 on macOS if available; record results in `specs/028-mirror-claude-config-dir/quickstart.md` under a Results heading
- [X] T021 `gofmt -l ./... ; go vet ./... ; task build` clean

---

## Dependencies & Execution Order

- Phase 2 (T002–T004) blocks everything: every story consumes `dir` from `Up`.
- US1 (T005–T009) needs Phase 2. T005/T006 same file, sequential. T007/T008 same file, sequential; independent of T005/T006.
- US2 (T010–T012) needs Phase 2 only; can run parallel to US1. T010 and T011 touch different files.
- US3 (T013–T015) needs T008 (symlink) for its manual check; T013/T014 can start after Phase 2.
- Phase 6 docs (T016–T019) can start after design is stable (post Phase 2); T020/T021 last.

## Parallel Execution Examples

- After T004: T005 (up.go mounts), T007 (Dockerfile), T010 (preflight), T013 (prompt), T014 (demo), T016–T019 (docs) in parallel.
- T011 after T004, parallel with T010.

## Implementation Strategy

- MVP = Phase 2 + US1 (T002–T009): fixes the reported plugin defect.
- US2 adds correctness for relocated config dirs and the only new validation.
- US3 and Phase 6 are wording plus verification; small, ship together.
