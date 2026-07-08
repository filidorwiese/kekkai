# Tasks: Update Notice on Up

**Input**: Design documents from `/specs/005-update-notice/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/update-notice-cli.md, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation; each
story phase runs its quickstart scenarios with ldflags-versioned builds.

**Organization**: One shared decision function + one wiring site serve all three
stories (they are outcome rows of the same check), so both are foundational; stories
then validate their outcomes independently.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Verify baseline: `go build ./... && go vet ./...` pass on clean tree

---

## Phase 2: Foundational (Blocking Prerequisites)

**⚠️ CRITICAL**: All user stories validate outputs of these two changes

- [X] T002 In internal/selfupdate/selfupdate.go: add exported `Notice(version string) string` — returns "" immediately for dev builds (same predicate as `Run`: `version == "dev" || !strings.HasPrefix(version, "v")`); otherwise `latestTag(repoSlug())`; any error → ""; `compareVersions(version, latest)` == -1 → return exact contract line `A new version of kekkai is available (<version> -> <latest>), run 'kekkai self-update' to upgrade`; equal/ahead → "" (contracts/update-notice-cli.md, research.md R1/R4/R5)
- [X] T003 In internal/runtime/up.go `Up()`: after validation passes (past the errs report, before container/image work) start `noticeCh := make(chan string, 1); go func() { noticeCh <- selfupdate.Notice(opts.Version) }()`; immediately before `docker.Interactive(args...)` do a non-blocking select read — non-empty message → `fmt.Println(msg)`; default → skip (research.md R2/R3); `go build ./... && go vet ./...` pass

**Checkpoint**: Feature complete in code; stories validate behavior

---

## Phase 3: User Story 1 - Notified of a newer release at startup (Priority: P1) 🎯 MVP

**Goal**: Older installed version → exactly one friendly notice line before handoff

**Independent Test**: quickstart.md scenario 1

- [X] T004 [US1] Run quickstart.md scenario 1: `go build -ldflags "-X main.version=v0.0.1" -o /tmp/kekkai-old ./cmd/kekkai`, run `up` in a prepared project (image prebuilt), verify exactly one line matching contracts/update-notice-cli.md before the interactive handoff and normal startup otherwise

**Checkpoint**: US1 delivers the MVP

---

## Phase 4: User Story 2 - Silence when current (Priority: P1)

**Goal**: Equal or ahead versions produce zero update-related output

**Independent Test**: quickstart.md scenario 2

- [X] T005 [US2] Run quickstart.md scenario 2: builds with `-X main.version=<latest>` and `v99.0.0`, run `up` with each, `grep -i "new version"` on output is empty for both

**Checkpoint**: US1 + US2 validated

---

## Phase 5: User Story 3 - Never blocks, never breaks startup (Priority: P2)

**Goal**: Dev builds, unreachable release service, and failing `up` runs are all silent with unchanged behavior

**Independent Test**: quickstart.md scenarios 3–5

- [X] T006 [US3] Run quickstart.md scenarios 3–5: dev build silent; `KEKKAI_REPO=nonexistent-owner/nonexistent-repo` with old build proceeds normally silent and unblocked; invalid-config run shows error report with no notice line

**Checkpoint**: All stories validated

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T007 SPECIFICATION.md: §3 `up` gains the notice behavior (one line, exact conditions, never blocks/fails); §10 self-update section cross-references that `up` advertises updates via the same release source (constitution I: same commit as code)
- [X] T008 Final: `go build ./... && go vet ./...`; quickstart scenario 6 exit-status spot check (notice run vs silent run — same codes)

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 (T002 → T003: T003 calls Notice) → stories in any order (each only needs Phase 2)
- Sequential preference: P1 stories first (T004, T005), then T006
- Polish last; T007 independent of T008 but both after stories

## Implementation Strategy

MVP = Phases 1–3 (code + notice-shown validation). US2/US3 are validation-only phases —
the silence outcomes are properties of T002/T003. Total new code ~40 lines, no new
files, no new exports beyond `Notice`.
