# Tasks: Yellow Update Notice

**Input**: Design documents from `/specs/008-yellow-update-notice/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/yellow-notice-cli.md, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation;
the story phase runs its quickstart scenarios against ldflags-versioned real binaries.

**Organization**: Single story (US1); one code change + spec amendment, then
e2e validation and regression.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Verify baseline: `go build ./... && go vet ./...` pass on clean tree

---

## Phase 2: User Story 1 - Update notice stands out in the startup output (Priority: P1) 🎯 MVP

**Goal**: Notice renders in the feature-006 advisory yellow when stdout is a terminal and `NO_COLOR` is unset; text/stream/silence frozen

**Independent Test**: quickstart.md scenarios 1-5

- [X] T002 [US1] In internal/runtime/up.go: add unexported helper `yellow(f *os.File, msg string) string` returning msg wrapped in `\033[33m`/`\033[0m` when `f.Stat()` mode has `os.ModeCharDevice` and `NO_COLOR` is unset, else msg unchanged (research.md R2); rewrite `warnNoConfig` to use `yellow(os.Stderr, ...)` (drop its inline color block); wrap the update-notice print in `Up` as `fmt.Println(yellow(os.Stdout, msg))` (research.md R1); amend SPECIFICATION.md §3 update-notice bullet with the yellow rendering note (FR-005, same change as code); `go build ./... && go vet ./...` pass
- [X] T003 [US1] Run quickstart.md scenarios 1-3: `go build -ldflags "-X main.version=v0.0.1" -o /tmp/kekkai-old ./cmd/kekkai`; under `script` TTY the notice is wrapped in `^[[33m`...`^[[0m` with 005-contract text between; with `NO_COLOR=1` under TTY plain; piped stdout plain (no escape codes)
- [X] T004 [US1] Run quickstart.md scenarios 4-5: silence conditions unchanged (current/ahead/dev builds and `KEKKAI_REPO=nonexistent-owner/nonexistent-repo` all print no update text); in a config-less dir under TTY both advisories render yellow on their own streams (`2>/dev/null` drops only the warning, stdout redirect drops only the notice)

**Checkpoint**: US1 delivers the feature

---

## Phase 3: Polish & Cross-Cutting Concerns

- [X] T005 Run quickstart scenario 6 (regression): current-version `up` in a configured project has zero escape codes in piped output; missing-config warning behavior from feature 006 unchanged (TTY yellow, piped plain); final `go build ./... && go vet ./...`

---

## Dependencies & Execution Order

- T001 → T002 → T003/T004 (any order) → T005

## Implementation Strategy

Single-story MVP: T002 is the whole change (~10 net lines: one helper, two
call sites, one spec line). T003-T005 are validation only.
