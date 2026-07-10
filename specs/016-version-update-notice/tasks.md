# Tasks: Update Notice on `kekkai version`

**Input**: Design documents from `/specs/016-version-update-notice/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/version-notice.md, quickstart.md

**Tests**: Per constitution IV, validation is end-to-end (stamped binaries against the real GitHub releases API; no docker needed). Each story phase ends with its quickstart scenario(s).

**Organization**: One code change serves both stories; US1 validates the notice display, US2 validates the script/offline guarantees of the same code.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 (P1 notice display), US2 (P2 scripts/offline unaffected)

## Path Conventions

Single Go binary project: `cmd/`, `internal/` at repository root (see plan.md).

---

## Phase 1: Setup

**Purpose**: Confirm clean baseline.

- [X] T001 Verify baseline: `go build ./...` succeeds; `git status` clean apart from this feature's spec artifacts

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared styling helper visible to `main.go` — blocks the US1 call site.

- [X] T002 In `internal/runtime/up.go`: export `yellow` → `Yellow` (doc comment: single advisory styling point — yellow iff target is a terminal and NO_COLOR unset); rename the three in-package callers (update notice, missing-config warning, sandbox-context warning); `go build ./...` passes (research.md R3)

**Checkpoint**: `runtime.Yellow` available; behavior unchanged.

---

## Phase 3: User Story 1 - Version check tells me I'm outdated (Priority: P1) 🎯 MVP

**Goal**: `kekkai version` prints the version, then the existing update notice in yellow on stderr when a newer release exists.

**Independent Test**: quickstart.md Scenario 1 — binary stamped `v0.0.1` shows version + yellow advisory naming the latest release and `kekkai self-update`.

- [X] T003 [US1] In `cmd/kekkai/main.go` `case "version"`: keep `fmt.Println(version)` first (stdout, unconditional), then `if notice := selfupdate.Notice(version); notice != "" { fmt.Fprintln(os.Stderr, runtime.Yellow(os.Stderr, notice)) }` (research.md R1/R2; contracts/version-notice.md output matrix; comment: version line first so the primary answer never waits on the courtesy lookup)
- [X] T004 [US1] E2E quickstart.md Scenario 1: `go build -ldflags "-X main.version=v0.0.1" -o kekkai-old ./cmd/kekkai`; terminal run of `./kekkai-old version` → stdout `v0.0.1`, stderr one yellow line `A new version of kekkai is available (v0.0.1 -> v<latest>), run 'kekkai self-update' to upgrade` (byte-identical to the specs/005 pinned text, SC-004)

**Checkpoint**: MVP — stale binaries self-report on `version`.

---

## Phase 4: User Story 2 - Scripts and offline use are unaffected (Priority: P2)

**Goal**: stdout stays exactly the version string; every failure path silent, exit 0, bounded time.

**Independent Test**: quickstart.md Scenarios 2–3 — `$(kekkai version)` exact, advisory on stderr only, 404/dev/current all silent.

- [X] T005 [US2] E2E quickstart.md Scenarios 2–3 with the binaries from T004: `v=$(./kekkai-old version 2>/dev/null)` → exactly `v0.0.1`; `./kekkai-old version 2>&1 >/dev/null | head -1` → the advisory; `NO_COLOR=1` → plain text, piped stderr → plain; dev build (`go build -o kekkai-dev ./cmd/kekkai`) → exactly `dev`, no stderr, instant; `KEKKAI_REPO=filidorwiese/does-not-exist ./kekkai-old version` → `v0.0.1` only, exit 0 (silent-failure branch, SC-003); current-tag stamp → version only

**Checkpoint**: Output contract locked on all rows of the matrix.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Spec-first compliance, regression guard on the existing surface.

- [X] T006 [P] In `SPECIFICATION.md` §3: `version` line becomes "print version (plus the §3 update notice on stderr, yellow, when a newer release exists)"; extend the update-notice paragraph (line ~47) with the second surface: synchronous on `version`, stderr, version line always first, same silent conditions. Constitution I: same commit as code
- [X] T007 E2E quickstart.md Scenario 4: `kekkai up` with the stale binary in a test dir still prints the notice once before the interactive handoff (helper rename regression, stdout surface unchanged); complete the quickstart sign-off checklist; cleanup test binaries

---

## Dependencies

```text
Phase 1 (T001) ─► Phase 2 (T002) ─► Phase 3 US1 (T003 → T004) 🎯 MVP
                                      └─► Phase 4 US2 (T005 — same binaries, validation only)
Phases 3–4 ─► Phase 5 (T006 ∥ T007-prep, T007 last: sign-off)
```

- US2 has no code of its own — it validates T003's change; runs after T004 builds the stamped binaries.
- T006 (SPECIFICATION.md) parallel to e2e; T007 strictly last.

## Parallel Execution Examples

- T006 (docs) ∥ T004/T005 (e2e) — disjoint files.
- Within T005 all checks share the two binaries from T004 — run in one shell session.

## Implementation Strategy

1. **MVP**: T002–T004 delivers the feature; T005 locks the contract.
2. **Spec-first gate**: T006 commits together with the code (constitution I).
3. Real-API note: Scenario 1 asserts against the live latest release tag — capture it once (`gh api repos/filidorwiese/kekkai/releases/latest --jq .tag_name`) for exact-string comparison.
