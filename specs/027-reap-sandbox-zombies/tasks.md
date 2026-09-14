# Tasks: Reap orphaned processes inside the sandbox

**Input**: Design documents from `/specs/027-reap-sandbox-zombies/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/lifecycle.md, quickstart.md

**Tests**: No test-framework tasks. Validation is end-to-end against a real docker daemon per constitution IV; each story phase carries its quickstart scenarios. A control run against the pre-change binary proves the repro is valid before the fix is trusted.

**Organization**: Tasks grouped by user story. The whole feature is one run-arg addition plus a spec amendment (Phase 2). US1 proves orphans are reaped, US2 proves signals and exit status are unchanged, US3 proves no rebuild and host coverage. SPECIFICATION.md is amended alongside the code (Principle I).

## Codebase impact summary

| Area | Change | Risk |
|---|---|---|
| `internal/runtime/up.go` `buildRunArgs` | insert `"--init"` after `"-it"` in the fixed prefix, with a why-comment (research R6) | low; one literal |
| `SPECIFICATION.md` §7.2 | `docker run --rm -it --init`; docker-init (tini) is PID 1, reaps orphans, forwards signals, exits with claude's status; claude is its child and the tty foreground process group | docs |
| `SPECIFICATION.md` §7.3 | assembly order prefixed with `--rm -it --init` before `--cap-add …` | docs |
| Behavior change for users | none visible; `ps` inside the sandbox shows `docker-init` as PID 1 | none |
| Unchanged | `internal/docker/exec.go`, `embed/Dockerfile.tmpl` CMD, `preflight_darwin.go`, `shell`/`exec`/`down`, config schema, image hash inputs, README | — |

Net: +1 arg and +2 comment lines Go, ~3 lines spec, no new dependency, no new config key or flag.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: e2e harness, a control binary, and the statusline repro

- [X] T001 Build the pre-change control binary from current `main`: `go build -ldflags "-X main.version=v0.0.0-ctl" -o /tmp/kekkai-ctl ./cmd/kekkai`; create scratch project `~/kekkai-qs-027-a` (under `$HOME`, not `/tmp`: snap daemon has a private `/tmp`) with a minimal `.kekkai.yaml` (pinned `claude.version`) and `git init`; confirm `docker info --format '{{.InitBinary}}'` prints `docker-init`
- [X] T002 [P] Record baseline into `/tmp/kekkai-qs-027-baseline.txt`: `docker images kekkai --format '{{.Tag}} {{.ID}}'` (quickstart 12) and a copy of the current `statusLine` block from `~/.claude/settings.json` so it can be restored
- [X] T003 [P] Write the statusline repro script `~/.claude/statusline-zombie-repro.sh` (`#!/bin/sh`, spawns a nested subshell that outlives its parent: `sh -c '(sleep 0.2) & exit 0'; echo ok`), `chmod +x`; do NOT switch `settings.json` to it yet (T007 / T011 do that per scenario)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the one code change and its spec amendment; everything the stories validate

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 In internal/runtime/up.go `buildRunArgs`: change `args := []string{"run", "--rm", "-it",` to `args := []string{"run", "--rm", "-it", "--init",` and add a comment above the slice explaining why: claude (node) as PID 1 never reaps re-parented grandchildren of statusline/hook helpers, so they accumulate as zombies; `--init` makes docker-init (tini) PID 1, which reaps orphans, forwards SIGTERM, and exits with claude's status; it is a run arg, not a bake-time input, so the image hash is unchanged (specs/027)
- [X] T005 [P] Amend SPECIFICATION.md §7.2 Lifecycle: `docker run --rm -it` → `docker run --rm -it --init`; add one sentence after it: docker-init (tini) is PID 1 and reaps orphaned processes, forwards signals it receives to its child and exits with the child's status; claude is its child and the tty foreground process group, so Ctrl-C and SIGWINCH reach claude directly. Keep the existing CMD description intact
- [X] T006 [P] Amend SPECIFICATION.md §7.3 Run args assembly: prefix the order with `--rm -it --init` (fixed, not configurable) → `--cap-add NET_ADMIN --cap-add NET_RAW` … (rest unchanged)
- [X] T007 `go build ./... && go vet ./...`; `task build` (→ `/tmp/kekkai`); `grep -n -- '--init' internal/runtime/up.go SPECIFICATION.md` shows the arg, §7.2 and §7.3; `kekkai up` in `~/kekkai-qs-027-a` via `script -qec "/tmp/kekkai up" /dev/null </dev/null &` — no rebuild, §9.6 probes pass (quickstart 13), then quickstart 1: `/tmp/kekkai exec ps -o pid,ppid,pgid,tpgid,comm` → PID 1 `docker-init`; `claude` has PPID 1 and PGID = TPGID = its own PID

**Checkpoint**: sandbox runs under docker-init with claude as the tty foreground process; stories can begin

---

## Phase 3: User Story 1 - Long sessions do not accumulate dead processes (Priority: P1) 🎯 MVP

**Goal**: orphaned grandchildren of statusline/hook helpers are reaped; zero defunct entries in the sandbox or on the host.

**Independent Test**: quickstart 2, 3, 4 — with the repro statusline, repeated refreshes leave zero `Z` entries under the new binary, and the control binary shows `Z` entries under the same procedure.

- [X] T008 [US1] Switch `~/.claude/settings.json` `statusLine.command` to `bash ~/.claude/statusline-zombie-repro.sh` (from T003); run quickstart 4 (control) FIRST against `/tmp/kekkai-ctl`: `kekkai down`, `script -qec "/tmp/kekkai-ctl up" /dev/null </dev/null &`, drive an interactive session through the pty for ~2 min (or `/tmp/kekkai-ctl exec claude -p "hi" </dev/null` several times plus a few `/tmp/kekkai-ctl exec sh -c 'sh -c "(sleep 0.2) & exit 0"'`), then `/tmp/kekkai-ctl exec ps -eo pid,ppid,stat,comm` → `Z` entries present, PID 1 is claude/node. Record the count in `/tmp/kekkai-qs-027-baseline.txt`
- [X] T009 [US1] Run quickstart 2 against `/tmp/kekkai`: `kekkai down`, `up` again, repeat exactly the T008 procedure, then `/tmp/kekkai exec ps -eo pid,ppid,stat,comm` → zero `Z` entries; orphaned `sleep` disappears within a second of exiting (spec US1 scenario 2)
- [X] T010 [US1] Run quickstart 3 (host view) while the T009 sandbox is still up: `docker top kekkai-<hash> -o pid,ppid,stat,cmd | grep -c defunct` → 0; also `ps -eo pid,ppid,stat,cmd | grep defunct | grep -v grep` filtered on the claude host PID → empty
- [X] T011 [US1] Restore the original `statusLine` block in `~/.claude/settings.json` from T002; `kekkai down`; `up` once more without any subprocess-spawning statusline and confirm the startup lines and prompt are unchanged (spec US1 scenario 3)

**Checkpoint**: defect reproduced on control, absent on the fix; MVP delivered

---

## Phase 4: User Story 2 - Signals and exit status behave exactly as before (Priority: P1)

**Goal**: Ctrl-C, `kekkai down`, terminal resize, exit codes 0 / non-zero / 128+signal, startup-failure path and mpr supervision are observably unchanged.

**Independent Test**: quickstart 5–11 — each lifecycle check gives the same result as before the change.

- [X] T012 [P] [US2] Quickstart 5 (Ctrl-C): `(sleep 3; printf '\003') | script -qec "/tmp/kekkai exec sleep 300" /dev/null` → sleep exits 130 and the sandbox stays up; in an interactive pty session press `^C` at the claude prompt → claude shows its host interrupt behavior, container still running (`docker ps`)
- [X] T013 [P] [US2] Quickstart 7 (resize): during the interactive pty session change the window size (`stty cols 100 rows 30` on the outer pty, or resize the terminal) → claude redraws to the new size; cross-check from a root exec that the container pty's size changed (`docker exec -u root <id> stty size < /dev/pts/0`)
- [X] T014 [US2] Quickstart 6 (down): with a session running, time `/tmp/kekkai down` from another shell → container gone within the usual grace period; the backgrounded `up` returns; `docker ps -a --filter label=kekkai.cwd=$HOME/kekkai-qs-027-a` → empty
- [X] T015 [US2] Quickstart 8 (exit 0): interactive pty session, `/exit` → the `up` process exits 0; read it as `$?` of the `script -e` wrapper (`--rm` removes the container, so `docker inspect .State.ExitCode` is unavailable)
- [X] T016 [US2] Quickstart 9 (non-zero and signal-kill): mirror the run prefix on the current image tag: `docker run --rm -it --init kekkai:<tag> sh -c 'exit 7'; echo $?` → 7; `docker run --rm -it --init kekkai:<tag> sh -c 'kill -9 $$'; echo $?` → 137; additionally with a live sandbox `docker kill -s TERM kekkai-<hash>` → the `up` wrapper exits non-zero (spec edge case: signal-killed claude yields non-zero)
- [X] T017 [US2] Quickstart 10 (startup failure still fails, no hang): after `down` and once `.kekkai.yaml` is writable again, set `network.allowed_domains: [example.com]` (§9.6 must-block probe), `up` → firewall verification fails, container exits non-zero within seconds, no lingering container (`docker ps -a`); revert the config
- [X] T018 [US2] Quickstart 11 (mpr supervision unchanged): with a session running, `/tmp/kekkai exec pkill -f '^python3 /usr/local/bin/kekkai-mpr serve'` (anchored — never the unanchored form), then `/tmp/kekkai exec claude -p "hi" </dev/null` → request succeeds; `/tmp/kekkai mpr` shows the captured request; `/tmp/kekkai exec ps -eo pid,ppid,stat,comm` → proxy restarted, no `Z` entries

**Checkpoint**: 100% of lifecycle checks match pre-change behavior (SC-002)

---

## Phase 5: User Story 3 - No rebuild, works on every supported host (Priority: P2)

**Goal**: upgrading kekkai triggers no image rebuild; the fix is active on Linux (snap Docker) and Docker Desktop for macOS.

**Independent Test**: quickstart 12 and 14 — image tags unchanged after the first `up` on the new binary; zombie scenarios pass on snap Docker and (deferred) Docker Desktop.

- [X] T019 [US3] Quickstart 12 (no rebuild): compare `docker images kekkai --format '{{.Tag}} {{.ID}}'` against the T002 baseline after all `up` runs above → identical; confirm no "building image" output appeared in any `up` log; `git diff --stat embed/` → empty
- [X] T020 [US3] Quickstart 2 on snap Docker (spec US3 scenario 2): this host's daemon is the snap package (`snap list docker` or `docker info` shows the snap paths); note in `/tmp/kekkai-qs-027-baseline.txt` that T009/T010 ran against it, satisfying SC-004 for Linux
- [X] T021 [US3] Quickstart 14 (macOS, deferred): add a "Deferred validation" note to specs/027-reap-sandbox-zombies/quickstart.md listing scenarios 1, 2, 5, 8 for Docker Desktop on Apple silicon, to be run by the maintainer per constitution IV tiering; also note that `docker info --format '{{.InitBinary}}'` must print `docker-init` there and that the darwin preflight probe (§7.4) is untouched
- [X] T022 [US3] Verify the FR-007 failure path is documented, not probed: confirm `internal/runtime/preflight_darwin.go` is unchanged (`git diff --stat`) and that contracts/lifecycle.md records the daemon-side `docker-init: no such file or directory` error as the abort message; no code change

**Checkpoint**: SC-003 and SC-004 (Linux) proven; macOS deferred with explicit checklist

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: hygiene and cleanup

- [X] T023 [P] Re-read internal/runtime/up.go diff: only the `--init` literal and its comment changed; `gofmt -l internal/` → empty; `go vet ./...` clean
- [X] T024 [P] Re-read SPECIFICATION.md §7.2/§7.3 diff against contracts/lifecycle.md "amendments" section; no other spec sections touched; README needs no change (no user-facing behavior)
- [X] T025 Cleanup: `/tmp/kekkai down` in `~/kekkai-qs-027-a` if running; `docker volume rm kekkai-history-<hash of ~/kekkai-qs-027-a>` ONLY (never a broad `grep kekkai-history`); `rm -rf ~/kekkai-qs-027-a /tmp/kekkai-ctl ~/.claude/statusline-zombie-repro.sh /tmp/kekkai-qs-027-baseline.txt`; confirm `~/.claude/settings.json` statusLine matches the T002 copy
- [X] T026 Tick every completed row in specs/027-reap-sandbox-zombies/quickstart.md (or note deferred), mark spec.md `**Status**: Draft` → `Implemented`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies; T001 first (control binary must be built from the pre-change tree, before T004 edits it), T002/T003 parallel
- **Foundational (Phase 2)**: T004, T005, T006 parallel (different files); T007 after all three. BLOCKS all stories
- **User Stories (Phase 3–5)**: all depend on T007; sequential by priority is recommended because they share one sandbox and one settings.json
- **Polish (Phase 6)**: after all stories

### User Story Dependencies

- **US1 (P1)**: after Phase 2. T008 (control) must run before T009 so the repro is proven valid on the same procedure. T011 restores settings and must be last in the phase
- **US2 (P1)**: after Phase 2; independent of US1. T012/T013 share one interactive session and can run together; T014–T018 each need a fresh `up`/`down` cycle
- **US3 (P2)**: T019 after every other `up` has run (it compares the final image list); T020 relies on T009/T010 having run on snap Docker; T021/T022 are doc checks and can run any time after Phase 2

### Parallel Opportunities

- Phase 1: T002 ‖ T003
- Phase 2: T004 ‖ T005 ‖ T006
- Phase 4: T012 ‖ T013 (same live session)
- Phase 5: T021 ‖ T022 with anything
- Phase 6: T023 ‖ T024

---

## Parallel Example: Phase 2

```bash
Task: "Insert --init after -it in buildRunArgs, internal/runtime/up.go"
Task: "Amend SPECIFICATION.md §7.2 Lifecycle"
Task: "Amend SPECIFICATION.md §7.3 Run args assembly"
# then
Task: "go build && go vet && task build && up && ps → docker-init is PID 1"
```

---

## Implementation Strategy

### MVP First (US1)

1. Phase 1 (control binary, scratch project, repro script)
2. Phase 2 (`--init` + spec) → T007 proves PID 1 is docker-init
3. Phase 3: control run shows zombies, fixed run shows none → MVP done
4. Commit only when asked

### Incremental Delivery

1. Phase 3 → zombie fix proven (SC-001)
2. Phase 4 → lifecycle parity proven (SC-002)
3. Phase 5 → no-rebuild and host coverage (SC-003, SC-004 Linux; macOS deferred)
4. Phase 6 → cleanup, quickstart ticked

---

## Notes

- Test sandbox binary is `/tmp/kekkai` (`task build`); control binary is `/tmp/kekkai-ctl`. Never run either in `$HOME` or a real project.
- `up` needs a pty: `script -qec "/tmp/kekkai up" /dev/null </dev/null &`, poll `docker ps`.
- `kekkai exec` is `docker exec` into the running container and is NOT a proxy for PID 1 exit-status checks (quickstart 9 uses a raw `docker run --init`).
- Never `pkill -x kekkai` or unanchored `pkill -f "kekkai-mpr serve"` (kills the supervisor / the backgrounded up).
- Never add `example.com` to allowed_domains except deliberately in T017 to trigger the must-block probe failure.
- Config edits after `down` must wait until `.kekkai.yaml` is writable again (ro-mount remnant).
