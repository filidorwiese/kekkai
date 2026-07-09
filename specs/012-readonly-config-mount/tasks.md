# Tasks: Read-Only Config Mount

**Input**: Design documents from `/specs/012-readonly-config-mount/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/config-mount.md, quickstart.md

**Tests**: No unit-test tasks — per Constitution IV validation is end-to-end against a real docker daemon; each story ends with its quickstart scenario.

**Organization**: Tasks grouped by user story. US1/US2 both modify `internal/runtime/up.go` (sequential); US3 touches `internal/config/config.go` + `internal/runtime/init.go` and can run in parallel with US1/US2.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

**Purpose**: Baseline before touching code — existing project, no scaffolding needed.

- [X] T001 Verify clean baseline: `go vet ./... && go build ./cmd/kekkai` from repo root

---

## Phase 2: Foundational (Blocking Prerequisites)

None — the three stories share no new infrastructure; all constants and helpers are story-local. Proceed directly to user stories.

---

## Phase 3: User Story 1 - Config readable but immutable inside sandbox (Priority: P1) 🎯 MVP

**Goal**: `.kekkai.yaml` bind-mounted read-only at `/workspace/.kekkai.yaml`, layered over the rw workspace mount, closing the self-escalation path.

**Independent Test**: quickstart.md Scenario 2 — inside the container, read succeeds and matches host; write/append/rm/mv all fail; host file byte-identical after `kekkai down`.

### Implementation for User Story 1

- [X] T002 [US1] In `buildRunArgs` (internal/runtime/up.go), immediately after the `pwd:/workspace` bind: if `pwd/.kekkai.yaml` is a regular file (reuse stat-gate pattern), append `-v <pwd>/.kekkai.yaml:/workspace/.kekkai.yaml:ro` per contracts/config-mount.md §3. Comment: why file-over-dir ro bind blocks self-escalation (no SYS_ADMIN → no remount, §5.2 pattern).
- [X] T003 [US1] E2E validate per quickstart.md Scenario 2 against real docker (pseudo-TTY: `script -qec "kekkai up" /dev/null`): read OK, all mutations fail, sha256 of host file unchanged after down. Also confirm no image rebuild was triggered (§6.1 regression guard).

**Checkpoint**: config-present sandboxes have an immutable, readable policy file — MVP done.

---

## Phase 4: User Story 2 - Comment-only placeholder when no config (Priority: P2)

**Goal**: No `.kekkai.yaml` on host → read-only placeholder with the exact one-line comment mounted at the same path; zero workspace/persistent host artifacts.

**Independent Test**: quickstart.md Scenario 3 — in-container file content equals exactly the placeholder comment; writes fail; no `.kekkai.yaml` appears in host workspace; no `kekkai-config-*` temp dir remains after exit.

### Implementation for User Story 2

- [X] T004 [US2] In internal/runtime/up.go add placeholder constant (exact content per contracts/config-mount.md §4, one line + trailing newline) and a helper that writes it 0444 into `os.MkdirTemp("", "kekkai-config-")`, returning file path + cleanup func
- [X] T005 [US2] Wire it up: when `pwd/.kekkai.yaml` is absent, mount the temp file `-v <tmpfile>:/workspace/.kekkai.yaml:ro` (same insertion point as T002); plumb the cleanup func from `buildRunArgs` to `Up` and defer it so removal runs after `docker.Interactive` returns (internal/runtime/up.go)
- [X] T006 [US2] E2E validate per quickstart.md Scenario 3: exact placeholder content in container, immutability, unchanged no-config advisory line (contracts §5), no workspace `.kekkai.yaml` during/after run, temp dir removed after down

**Checkpoint**: config path always readable in-container; creation path from inside sandbox closed.

---

## Phase 5: User Story 3 - Typo file `.kekkai.yml` warns and exits (Priority: P2)

**Goal**: Only `.kekkai.yaml` is ever read; a `.kekkai.yml` entry makes `up` exit non-zero with a rename instruction before any docker work, and makes `init` refuse.

**Independent Test**: quickstart.md Scenario 1 — `touch .kekkai.yml; kekkai up` prints the contract §1 message and exits 1 with no container/image work; both-present case identical; `kekkai init` refuses per contract §2.

### Implementation for User Story 3

- [X] T007 [P] [US3] Rewrite `Discover` in internal/config/config.go per data-model.md state transitions: `os.Lstat` on `.kekkai.yml` (any entry type) → typo error with exact contract §1 text; only `.kekkai.yaml` recognized otherwise; delete the "both exist, remove one" error; update package/func comments and the `Discover` doc comment ("Both .kekkai.yml and .kekkai.yaml present is an error" is obsolete)
- [X] T008 [P] [US3] Split the stat loop in `Init` (internal/runtime/init.go): `.kekkai.yml` check first → typo refusal per contract §2; `.kekkai.yaml` → existing "already exists, not overwriting" message unchanged
- [X] T009 [US3] E2E validate per quickstart.md Scenario 1: typo-only, both-present, and init cases; confirm exit 1, no docker resources created, message text matches contract byte-for-byte

**Checkpoint**: all three stories independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T010 [P] Update SPECIFICATION.md: §4.1 single accepted name + typo rejection (drop `.kekkai.{yml,yaml}` dual acceptance, line 55), §4.5 init typo refusal, §5.2 new builtin mount bullet (config ro bind / placeholder, secrets-shadow precedence note) — same commit as code per Constitution I
- [X] T011 [P] Update README.md Configure section: only `.kekkai.yaml` is read; config visible read-only inside the sandbox
- [X] T012 Full regression sweep per quickstart.md Scenario 4: firewall probes pass, config edit triggers no rebuild, secrets.hide still shadows, `go vet ./... && go build ./...` clean

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1 (T001): start immediately
- Phase 2: empty — no blocker
- US1 (Phase 3) → US2 (Phase 4): sequential, both edit `internal/runtime/up.go` mount assembly (T005 builds on T002's insertion point)
- US3 (Phase 5): independent files — can run in parallel with US1/US2
- Polish (Phase 6): after all stories; T010/T011 parallel, T012 last

### Parallel Opportunities

- T007 + T008 (different files) in parallel; both parallel to T002-T006
- T010 + T011 in parallel
- Example: `Task: "Rewrite Discover in internal/config/config.go"` + `Task: "Split Init stat loop in internal/runtime/init.go"` while another agent does T002

---

## Implementation Strategy

**MVP first**: T001 → T002 → T003 (US1 alone already closes the main escalation path for configured projects). Then US2 completes the boundary for unconfigured projects, US3 the naming contract. Docs (T010/T011) land in the same commit as the code they describe (Constitution I) — if committing per-story, split the SPECIFICATION.md edits accordingly.

**Single-agent order**: T001 → T002 → T003 → T004 → T005 → T006 → T007 → T008 → T009 → T010 → T011 → T012.
