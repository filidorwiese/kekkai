# Tasks: Mirror the host project path inside the sandbox

**Input**: Design documents from `/specs/026-mirror-host-path/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/sandbox-layout.md, quickstart.md

**Tests**: No test-framework tasks — validation is end-to-end against a real docker daemon per constitution IV; each story phase carries its quickstart scenarios.

**Organization**: Tasks grouped by user story. US1 (per-project isolation) and US2 (host/sandbox sharing) are both P1 and are delivered by the same change: Phase 2 builds the path helper, validation and the mirrored `run` args; US1 proves isolation, US2 adds symlink resolution and proves host sharing; US3 sweeps every remaining `/workspace` assumption (shell/exec cwd, image, prompt, preflight probe, spec, docs); US4 completes the rejection paths and the macOS check. SPECIFICATION.md is amended alongside the code (Principle I).

## Codebase impact summary

| Area | Change | Risk |
|---|---|---|
| `internal/runtime/identity.go` | + `ProjectDir()` (Getwd + EvalSymlinks), + `ValidateProjectPath()`, protected-path and charset constants | low; pure functions |
| `internal/runtime/up.go` | replace 6 `/workspace` literals (rw bind, config bind ×2, `.git` bind, secret shadows, `-w`) with `P`; call validation after config validation | medium; touches every builtin bind — covered by quickstart 7–10 |
| `internal/runtime/{shell,exec}.go` | `ProjectDir()`; add `-w P` to `docker exec` | low |
| `internal/runtime/{down,traffic,mpr,init}.go` | `os.Getwd()` → `ProjectDir()` so label lookups agree with `up` | low |
| `internal/runtime/preflight_darwin.go` | probe bind dst `/kekkai-probe/workspace` → `P` | low; darwin only |
| `internal/runtime/sandboxprompt.go` | two wording lines | low; prompt text pinned in contract |
| `embed/Dockerfile.tmpl` | drop `/workspace` from `mkdir`/`chown`; `WORKDIR /home/kekkai` | one-time rebuild for all users |
| `SPECIFICATION.md` | §4.4, §5.2, §5.3 (prompt), §6.3, §7.1, §7.3, §7.4, §8 | docs |
| `README.md` | upgrade/orphan note | docs |
| `specs/011-sandbox-awareness/contracts/sandbox-prompt.md` | pointer to superseding lines | docs |
| Behavior change for users | project visible at host path; symlinked invocations get new container/volume names; old `-workspace` Claude state orphaned; paths with `:` / at `/` / `/home` refused | announce in release notes |
| Unchanged | firewall script, mpr script, config schema, env vars (`WORKSPACE` stays), image hash inputs, `kekkai prune`, `kekkai ps` | — |

Net: ~+60/-20 lines Go, ~-2 lines template, no new dependency, no new config key or flag.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: e2e harness and a baseline record of the pre-change state

- [X] T001 Build test binary (`task build` → `/tmp/kekkai`); create scratch projects `/tmp/kekkai-qs-026-a` and `/tmp/kekkai-qs-026-b` each with a minimal `.kekkai.yaml` (pinned `claude.version`) and `git init`; confirm docker daemon reachable and `~/.claude` holds a working login
- [X] T002 [P] Record baseline: `ls -la ~/.claude/projects/-workspace` (mtime) and `docker images kekkai --format '{{.Tag}}'` into `/tmp/kekkai-qs-026-baseline.txt` for quickstart 15 and 22

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the single path helper, its validation, and the mirrored `docker run` args — everything the stories build on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Add `ProjectDir() (string, error)` to internal/runtime/identity.go: `os.Getwd()` then `filepath.EvalSymlinks`, return the cleaned absolute path; doc comment stating it is the sole source of the project path for labels, names, binds and `-w` (research R2)
- [X] T004 [P] Add `ValidateProjectPath(p string) []error` to internal/runtime/identity.go with code constants `protectedContainerPaths = []string{"/home/kekkai", "/usr/local/bin", "/commandhistory"}`; reject `/`, equal-or-ancestor of a protected path (component-wise via `filepath.Rel`, not string prefix), and any `:` or byte < 0x20; error texts exactly as in contracts/sandbox-layout.md; return all violations
- [X] T005 In internal/runtime/up.go `Up`: replace `os.Getwd()` with `ProjectDir()`; append `ValidateProjectPath(pwd)` results to `errs` after `config.Validate` so they print in the same one-pass `invalid configuration` report (spec FR-009); verify no docker call precedes it
- [X] T006 In internal/runtime/up.go `buildRunArgs` (or equivalent): change `-v pwd:/workspace` → `pwd:pwd`; config bind and placeholder dst → `filepath.Join(pwd, ".kekkai.yaml")`; `.git` ro bind dst → `filepath.Join(pwd, ".git")`; secrets shadow `containerPath` → `filepath.Join(pwd, rel)`; final `-w /workspace` → `-w pwd`; update the §5.2/§8 comments to say "mirrored project path"
- [X] T007 `go build ./... && go vet ./...`; `kekkai up` in `/tmp/kekkai-qs-026-a` (no rebuild expected yet); verify the run cwd via `docker exec <id> readlink /proc/1/cwd` → `/tmp/kekkai-qs-026-a` (`kekkai exec pwd` only agrees after T014 adds `-w`); `docker ps --format '{{.Label "kekkai.cwd"}}'` → same path

**Checkpoint**: sandbox runs with the project at its host path; stories can begin

---

## Phase 3: User Story 1 - Each project keeps its own Claude state (Priority: P1) 🎯 MVP

**Goal**: Claude inside sandboxes for two different projects derives two different project keys; nothing bleeds.

**Independent Test**: quickstart 2 and 3 — memory saved in A is absent in B; `~/.claude/projects/` gains `-tmp-kekkai-qs-026-a` and `-tmp-kekkai-qs-026-b`, and `-workspace` mtime is unchanged.

- [X] T008 [US1] Run quickstart 2 in `/tmp/kekkai-qs-026-a`: one Claude turn, then `kekkai exec ls /home/kekkai/.claude/projects` → contains `-tmp-kekkai-qs-026-a`; compare `-workspace` mtime with T002 baseline
- [X] T009 [US1] Run quickstart 3: have Claude save a memory in A, `kekkai down`, `kekkai up` in B, ask Claude to recall → absent; `kekkai exec cat /home/kekkai/.claude/.claude.json | jq '.projects | keys'` → contains both `/tmp/...-a` and `/tmp/...-b`, no new writes under `/workspace` key
- [X] T010 [US1] Run quickstart 9, 10, 14: `git.enabled: false` → `git log` works, commit fails; `secrets.hide: [.env, certs/]` → empty file, empty dir at `P/.env`, `P/certs`; bash history survives `down`/`up`

**Checkpoint**: isolation proven; builtin ro binds and shadows work under the mirrored path

---

## Phase 4: User Story 2 - Host and sandbox share the same project state (Priority: P1)

**Goal**: the same project key on host and in sandbox, including when invoked through a symlink.

**Independent Test**: quickstart 4, 5, 6 — host-saved memory recalled in sandbox and vice versa; session resumable across sides; symlinked invocation resolves to the real path everywhere.

- [X] T011 [P] [US2] Replace `os.Getwd()` with `ProjectDir()` in internal/runtime/down.go, internal/runtime/traffic.go, internal/runtime/mpr.go, internal/runtime/init.go so label lookups and `init` use the resolved path; keep error messages printing the resolved path
- [X] T012 [US2] Run quickstart 4 and 5 in `/tmp/kekkai-qs-026-a`: save a memory with host `claude`, `kekkai up`, recall inside → present; start a session inside, exit, `claude --resume` on host → listed; confirm both sides write to the same `~/.claude/projects/-tmp-kekkai-qs-026-a/`
- [X] T013 [US2] Run quickstart 6: `ln -s /tmp/kekkai-qs-026-a /tmp/kekkai-qs-026-link; cd` there; `kekkai up`; `docker ps --format '{{.Label "kekkai.cwd"}} {{.Names}}'` → real path and hash of real path; `kekkai exec pwd` → real path; `kekkai down` works from both the link and the real dir

**Checkpoint**: host sharing and symlink resolution proven

---

## Phase 5: User Story 3 - Everything that referenced the workspace keeps working (Priority: P2)

**Goal**: no remaining fixed-path assumption in code, image, prompt, spec or docs; every subcommand and feature behaves as before.

**Independent Test**: quickstart 7, 8, 11, 12, 13, 15, 19, 20 plus `grep -rn /workspace --exclude-dir=specs --exclude-dir=.git .` → empty.

- [X] T014 [P] [US3] Add `-w <P>` to the `docker exec` invocations in internal/runtime/shell.go (`exec -it -w P id bash`) and internal/runtime/exec.go (after `-i[-t]`, before the id); use `ProjectDir()` in both
- [X] T015 [P] [US3] Edit embed/Dockerfile.tmpl: remove `/workspace` from the `mkdir -p` and `chown -R` lines; change `WORKDIR /workspace` → `WORKDIR /home/kekkai` with a comment: neutral fallback, real cwd is a runtime `-w` (specs/026, §6.1 bake-time rule)
- [X] T016 [P] [US3] Reword the two lines in internal/runtime/sandboxprompt.go exactly per contracts/sandbox-layout.md ("only the project directory (mounted at the same path as on the host) and explicitly configured mounts are visible", "in the project root"); update the constant's comment to cite specs/026
- [X] T017 [P] [US3] Change the `secrets.hide` template comment in internal/runtime/init.go from "relative to the workspace root" to "relative to the project root"
- [X] T018 [P] [US3] Amend SPECIFICATION.md: §4.4 add the project-path validation bullet (root, protected ancestors, `:`/control chars, one-pass); §5.2 rewrite the bind list with `$PWD` (resolved, symlinks followed) as both source and destination for the rw bind, config bind, `.git` bind; §5.3 note the prompt wording; §6.3 remove `/workspace` mention, state `WORKDIR /home/kekkai` and that the project path never enters the template; §7.1 identity derives from the resolved real path; §7.3 tail `-w $PWD`, `shell`/`exec` pass `-w`; §7.4 probe binds `$PWD:$PWD:ro`; §8 shadow paths resolve under `$PWD`
- [X] T019 [P] [US3] Add to README.md (configure/limitations area) a short note: the project is mounted at its host path so Claude's per-project memory/sessions are shared with the host; upgrade note that `~/.claude/projects/-workspace` is orphaned and safe to delete; symlinked directories get a new container name once
- [X] T020 [P] [US3] Add a one-line "superseded by specs/026-mirror-host-path/contracts/sandbox-layout.md" note above the Prompt value section in specs/011-sandbox-awareness/contracts/sandbox-prompt.md
- [X] T021 [US3] `go build ./... && go vet ./...`; `grep -rn '/workspace' --exclude-dir=specs --exclude-dir=.git .` → no matches; `kekkai up` in A (expect exactly one rebuild from the template change), then quickstart 7, 8, 11, 12, 13, 19, 20
- [X] T022 [US3] Run quickstart 15: `kekkai up` in B after A → `docker images kekkai` shows one tag, no second build

**Checkpoint**: full feature parity under the mirrored path; docs and spec consistent

---

## Phase 6: User Story 4 - Unsupported paths fail early and clearly (Priority: P2)

**Goal**: every unmirrorable path is refused before any container work with the contract's message; macOS unshared folders surface through preflight naming the real path.

**Independent Test**: quickstart 16, 17, 18, 21.

- [X] T023 [P] [US4] Change the project probe bind in internal/runtime/preflight_darwin.go from `pwd + ":/kekkai-probe/workspace:ro"` to `pwd + ":" + pwd + ":ro"`; use `ProjectDir()`-resolved `pwd` passed by `Up`; update the comment and the probe block in specs/002-macos-support/contracts/preflight.md
- [X] T024 [US4] Run quickstart 16, 17, 18 on linux: `/tmp/my proj ü` starts and `exec pwd` prints it exactly; `/tmp/a:b` exits 1 with the charset error and `docker ps -a` shows no container created; `cd /home && kekkai up` exits 1 naming `/home/kekkai`; also `/home/kekkai` itself if such a dir exists on the host, else skip
- [ ] T025 [US4] **DEFERRED 2026-09-09: no macOS host available in this session; code path (`preflight_darwin.go` probe `P:P:ro`) compiles under `GOOS=darwin go vet`, e2e pending on a Mac.** Run quickstart 21 on macOS Docker Desktop (maintainer-validated tier): project in a folder outside File Sharing → `kekkai: preflight failed — bind` naming `P` with the File Sharing hint; then re-share and confirm `up` proceeds. If no macOS host is available, mark the task deferred in this file and note it in the commit message

**Checkpoint**: rejection paths and macOS detection proven

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T026 [P] Run quickstart 22: `ls -la ~/.claude/projects/-workspace` mtime equals the T002 baseline; `jq '.projects["/workspace"]' ~/.claude/.claude.json` unchanged
- [X] T027 [P] Re-read internal/runtime/up.go comments and internal/runtime/identity.go doc header (§7.1 "any change updates every consumer") for stale `/workspace` wording; fix any found
- [X] T028 Final `go build ./... && go vet ./... && gofmt -l internal cmd` (expect no files); `git diff --stat`; `kekkai down` in all scratch projects; remove `/tmp/kekkai-qs-026-*` and the symlink (created this session)

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 → US1 → US2 → US3 → US4 → Polish.
- US1 and US2 are one delivered change (Phase 2) plus proof; US2 adds only T011.
- US3 and US4 are independent of each other after Phase 2; both need T003/T005/T006.
- T015 (template) forces one rebuild; do it before T021 so only one rebuild happens in the run.

## Parallel Execution Examples

- Phase 2: T003 ∥ T004 (same file, different functions — sequential edits, but independent design); T005/T006 after both.
- Phase 5: T014, T015, T016, T017, T018, T019, T020 all touch different files → run together, then T021.
- Phase 6: T023 ∥ T024 (darwin file vs linux e2e).
- Polish: T026 ∥ T027.

## Implementation Strategy

1. **MVP** = Phase 1–3 (T001–T010): isolation works, ro binds and shadows verified. Shippable on its own except that `kekkai shell`/`exec` still land in `/workspace`-less `WORKDIR` until T014/T015.
2. Add US2 (T011–T013) for symlink-safe lookups and host sharing proof.
3. US3 sweep (T014–T022) closes every fixed-path reference and triggers the single rebuild.
4. US4 (T023–T025) hardens rejection and macOS.
5. Polish, then one commit: code + `SPECIFICATION.md` + README together (Principle I).
