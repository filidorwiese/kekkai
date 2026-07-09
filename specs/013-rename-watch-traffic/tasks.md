# Tasks: Rename `kekkai watch` to `kekkai traffic`

**Input**: Design documents from `/specs/013-rename-watch-traffic/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md (stub), contracts/traffic-cli.md, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation against a real docker daemon; quickstart.md scenarios are the test surface and appear as validation tasks below.

**Organization**: Tasks grouped by user story. US1 (rename works) is the MVP; US2 (old name fails loudly) is satisfied by the same dispatch edit and needs only its own validation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 or US2 (maps to spec.md user stories)

---

## Phase 1: Setup

**Purpose**: Confirm a green baseline before touching anything.

- [X] T001 Verify baseline build: `go build ./...` and `go vet ./...` pass at repo root before any rename edits

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: None — pure rename, no shared infrastructure to build. No tasks.

**Checkpoint**: Proceed directly to US1.

---

## Phase 3: User Story 1 - Inspect egress under the new name (Priority: P1) 🎯 MVP

**Goal**: `kekkai traffic` delivers exactly the feature 010 experience; every tool-produced string and living doc says `traffic`, nothing says `watch` (FR-001, FR-003, FR-004, FR-005).

**Independent Test**: With a sandbox running, run `kekkai traffic`, generate traffic, verify the stream per contracts/traffic-cli.md; check `kekkai help` and living docs mention only `traffic`.

### Implementation for User Story 1

- [X] T002 [US1] Rename `internal/runtime/watch.go` → `internal/runtime/traffic.go` (git mv); rename `Watch()` → `Traffic()`; change stale-image hint (line 111) to `sandbox image predates 'kekkai traffic'; run 'kekkai down' and 'kekkai up' to rebuild`; update doc comments naming the command (lines 19, 22). Keep `watcher`, `newWatcher`, `event`, and the "watching egress of sandbox for %s" stderr line unchanged per research R4 and FR-001 (contract strings are byte-identical except the three substitutions)
- [X] T003 [US1] Update `cmd/kekkai/main.go`: usage help line 31 `watch` → `traffic` (entry stays alphabetically/positionally where the contract's help text expects it); `case "watch"` → `case "traffic"` (line 65); usage error → `usage: kekkai traffic (no arguments)` (line 67); call `runtime.Traffic()` (line 69). Depends on T002 (symbol must exist to compile)
- [X] T004 [P] [US1] Update `internal/runtime/up.go` line 32 comment: `// kekkai watch (nflog reader)` → `// kekkai traffic (nflog reader)`
- [X] T005 [P] [US1] Update `embed/init-firewall.sh` comment-only: 4 sites naming `kekkai watch` (lines 29, 79, 100, 160) → `kekkai traffic`. Diff MUST touch comments only — no rule, policy, ACCEPT/REJECT, probe, or sudoers change (FR-005, research R2)
- [X] T006 [P] [US1] Update `SPECIFICATION.md`: §3 command line (line 37) and behavior bullet (line 50, including contract pointer → `specs/013-rename-watch-traffic/contracts/traffic-cli.md`); §5.1 tcpdump note (line 141); §9 NFLOG paragraphs (lines 223, 232) — all `watch` command references → `traffic`
- [X] T007 [P] [US1] Update `README.md` line 66 command digest: `kekkai watch` → `kekkai traffic`
- [X] T008 [P] [US1] Add one-line supersession note at top of `specs/010-inspect-egress/contracts/watch-cli.md` pointing at `specs/013-rename-watch-traffic/contracts/traffic-cli.md`; leave the rest of the file untouched (research R3)
- [X] T009 [US1] Verify: `go build ./...` + `go vet ./...` pass; `grep -ri "kekkai watch" cmd/ internal/ embed/ SPECIFICATION.md README.md` returns zero hits (SC-003; specs/010 history exempt); `git diff embed/init-firewall.sh` shows comment-only changes
- [X] T010 [US1] E2E validation per quickstart.md scenarios 1-3 and 5-10: build `/tmp/kekkai-test`, start sandbox via pseudo-TTY (`script -qec "/tmp/kekkai-test up" /dev/null </dev/null &`, expect one-time image rebuild), verify help text, no-sandbox error, usage error, ALLOW/BLOCK/DNS stream formats, Ctrl+C cleanup, sandbox-stopped exit, stale-image hint, §9.6 probes pass (SC-001, FR-005)

**Checkpoint**: `kekkai traffic` fully functional; all tool output and living docs say `traffic`.

---

## Phase 4: User Story 2 - Old name fails loudly, not silently (Priority: P2)

**Goal**: `kekkai watch` is a plain unknown command: standard error, usage overview listing `traffic`, nonzero exit (FR-002).

**Independent Test**: Run `kekkai watch`; verify unknown-command error, usage lists `traffic`, exit 1.

### Implementation for User Story 2

- [X] T011 [US2] Verify `kekkai watch` (with and without a running sandbox) prints the standard `unknown command "watch"` error plus usage listing `traffic` and exits 1 (quickstart scenario 4, SC-002). No code expected — the `case "watch"` removal in T003 provides the fallthrough; if any watch-specific handling remains in `cmd/kekkai/main.go`, remove it

**Checkpoint**: Both stories independently validated.

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T012 Full quickstart.md pass end-to-end in order (scenarios 1-11) including the final repo-wide grep, then cleanup per quickstart (targeted `docker rm -f` by `kekkai.cwd` label; remove only this test dir's history volume, never bulk-remove `kekkai-history-*`)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Empty — no blocker
- **US1 (Phase 3)**: Starts after T001
- **US2 (Phase 4)**: Validation-only; needs T003 merged (the dispatch edit). Can run any time after T003, but scenario 4 is naturally checked alongside T010's sandbox session
- **Polish (Phase 5)**: After both stories

### Task Dependencies

- T002 → T003 (compile: `runtime.Traffic` must exist)
- T004, T005, T006, T007, T008 [P]: independent files, can run in parallel with each other and with T002/T003
- T009 needs T002-T008 complete
- T010 needs T009 (green build, clean grep)
- T011 needs T003
- T012 needs T010, T011

### Parallel Opportunities

```bash
# After T002+T003 land (or concurrently, different files):
Task: "Update internal/runtime/up.go line 32 comment"            # T004
Task: "Update embed/init-firewall.sh 4 comment sites"            # T005
Task: "Update SPECIFICATION.md §3/§5.1/§9"                       # T006
Task: "Update README.md line 66"                                 # T007
Task: "Add supersession note to 010 watch-cli.md"                # T008
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. T001 baseline
2. T002-T009: complete rename + static verification
3. T010: e2e validate against a real sandbox
4. **STOP and VALIDATE** — US1 alone is a shippable rename

### Incremental Delivery

US2 costs one extra validation run (T011) inside the same sandbox session as T010 — in practice do T010 and T011 in one sitting, then T012 as the final sweep. Single commit is appropriate: FR-004 requires code and docs renamed in the same change.
