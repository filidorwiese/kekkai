# Tasks: kekkai exec

**Input**: Design documents from `/specs/009-exec-command/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/exec-cli.md, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation;
each story phase runs its quickstart.md scenarios against a real docker sandbox.

**Organization**: Three stories share one small implementation (US1 carries it);
US2 and US3 phases validate the behaviors those stories own (pipes/signals,
error paths).

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Verify baseline: `go build ./... && go vet ./...` pass on clean tree

---

## Phase 2: User Story 1 - Run a one-off command in the sandbox (Priority: P1) 🎯 MVP

**Goal**: `kekkai exec <command> [args...]` runs the command in the running sandbox for `$PWD`, streams output, exits with the command's exit code

**Independent Test**: quickstart.md scenarios 1-5, 8

- [X] T002 [US1] Create internal/runtime/exec.go: `Exec(cmdArgs []string) (int, error)` mirroring shell.go — `os.Getwd()`, `docker.ContainersByLabel(LabelCwd+"="+pwd)`, first running container → `docker.Interactive` with `exec -i` (+`-t` only when `os.Stdin.Stat()` mode has `os.ModeCharDevice`, research.md R1), container ID, then cmdArgs verbatim; no running container → error `no running sandbox for <pwd>, run 'kekkai up'` exit 1 (research.md R5); `go build ./...` passes
- [X] T003 [US1] In cmd/kekkai/main.go: add `case "exec"` dispatching to `runtime.Exec` — strip one optional leading `--` from args, no `flag.FlagSet` (FR-004, research.md R4); zero words left → error `usage: kekkai exec [--] <command> [args...]` exit 1; add the exec entry to the `usage` string per contracts/exec-cli.md help-text block; `go build ./... && go vet ./...` pass
- [X] T004 [P] [US1] Amend SPECIFICATION.md §3: add `kekkai exec` line to the CLI block and a behavior bullet (verbatim args, exit-code passthrough, conditional TTY, requires running sandbox, exact strings in specs/009-exec-command/contracts/exec-cli.md); update README.md command digest (constitution I, same change as code)
- [X] T005 [US1] e2e per quickstart.md: build `/tmp/kekkai-test`, start sandbox under pseudo-TTY (`script -qec ... &`), run scenarios 1-5 (ls output exit 0; `false` → 1; `exit 42` → 42; `ls -la` flags reach ls; leading `--` stripped) and scenario 8 (`nosuchcmd` → 127, docker not-found on stderr)

**Checkpoint**: one-shot commands work with exit-code passthrough — MVP delivered

---

## Phase 3: User Story 2 - Use exec from scripts and pipelines (Priority: P2)

**Goal**: exec composes with pipes and scripts — stdin forwarded, no TTY error when piped, streams separated, interrupt returns control

**Independent Test**: quickstart.md scenarios 6, 7, 11

- [X] T006 [US2] e2e per quickstart.md scenarios 6-7: `echo hi | /tmp/kekkai-test exec cat` prints `hi` exit 0 with no "input device is not a TTY" error; `sh -c 'echo out; echo err >&2'` with redirects lands `out`/`err` on the correct streams (FR-003, FR-009)
- [X] T007 [US2] e2e per quickstart.md scenario 11: from an interactive terminal run `exec sleep 300`, Ctrl+C → prompt returns promptly with nonzero exit, `docker ps` still shows the sandbox (FR-008); confirm the non-TTY interrupt limitation reads as documented in contracts/exec-cli.md

**Checkpoint**: scriptable path proven — pipelines, capture, and interrupt behave

---

## Phase 4: User Story 3 - Clear failure when no sandbox is running (Priority: P3)

**Goal**: actionable errors, exit 1, nothing executed

**Independent Test**: quickstart.md scenarios 12, 13

- [X] T008 [US3] e2e per quickstart.md scenarios 12-13: `exec` with no command → exact usage string on stderr, exit 1; from a directory with no sandbox → `no running sandbox for <pwd>, run 'kekkai up'` on stderr, exit 1, nothing executed (FR-005, FR-006)

**Checkpoint**: all three stories independently validated

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T009 e2e per quickstart.md scenarios 9-10, 14: `exec whoami`/`exec pwd` show sandbox user and `/workspace` (FR-007); `exec curl https://example.com` blocked by firewall (FR-007); sandbox still running after all scenarios (FR-010); `kekkai shell` still works (shared resolution unchanged)
- [X] T010 Cleanup test resources (`docker rm -f` sandbox, remove `kekkai-history-*` test volume) and final `go build ./... && go vet ./...` on the finished tree

---

## Dependencies & Execution Order

- T001 → T002 → T003 → T005; T004 [P] anytime after T001 (docs-only, parallel with T002/T003)
- T005 → T006/T007/T008 (any order — same running sandbox) → T009 → T010

## Parallel Opportunities

- T004 (SPECIFICATION.md/README) runs parallel to T002/T003 (different files)
- T006, T007, T008 are independent scenario groups against the same sandbox

## Implementation Strategy

MVP = Phase 2: T002+T003 are the whole change (~35 net lines: one new file
mirroring shell.go, one dispatch case, usage text) plus the T004 spec amendment.
Phases 3-5 add no code — they validate behaviors already carried by the
conditional-TTY and error-path decisions baked into T002/T003. If any scenario
fails, fix within the owning story's phase before proceeding.
