# Tasks: Optional Configuration File

**Input**: Design documents from `/specs/006-optional-config/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/optional-config-cli.md, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation;
each story phase runs its quickstart scenarios against the real binary.

**Organization**: US1 (missing file) and US2 (empty file) share the two
foundational code changes; US3 additionally rewrites the init template.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Verify baseline: `go build ./... && go vet ./...` pass on clean tree

---

## Phase 2: Foundational (Blocking Prerequisites)

**⚠️ CRITICAL**: US1 and US2 validate outputs of these two changes

- [X] T002 In internal/config/config.go: (a) in `Load`, treat `errors.Is(err, io.EOF)` from `dec.Decode` as an empty document — skip the error and fall through to `applyDefaults()` (research.md R2); (b) add exported `Defaults() *Config` returning `&Config{}` with `applyDefaults()` applied (research.md R1); (c) update the `ErrNoConfig` comment — no longer a user-facing `up` error
- [X] T003 In internal/runtime/up.go `Up()`: when `cfg == nil && len(errs) == 1 && errors.Is(errs[0], config.ErrNoConfig)`, print the contract warning line to stderr — wrapped in yellow (`\033[33m`...`\033[0m`) only when `os.Stderr.Stat()` mode has `os.ModeCharDevice` and `NO_COLOR` is unset — then continue with `cfg = config.Defaults()` and empty errs (contracts/optional-config-cli.md, research.md R3); `go build ./... && go vet ./...` pass

**Checkpoint**: Missing/empty config paths work; stories validate behavior

---

## Phase 3: User Story 1 - Try kekkai instantly in any project (Priority: P1) 🎯 MVP

**Goal**: `up` without any config file starts on defaults after one yellow stderr warning

**Independent Test**: quickstart.md scenario 1

- [X] T004 [US1] Run quickstart.md scenario 1: `up` in a dir without config shows exactly one warning line matching contracts/optional-config-cli.md before other output, then default startup; verify stderr routing (`2>/dev/null` hides it) and plain text when piped (no ANSI codes)

**Checkpoint**: US1 delivers the MVP

---

## Phase 4: User Story 2 - Empty or fully commented configuration file works (Priority: P1)

**Goal**: Present-but-empty file means defaults — no parse error, no warning

**Independent Test**: quickstart.md scenario 2

- [X] T005 [US2] Run quickstart.md scenario 2: zero-byte `.kekkai.yaml` then comments-only `.kekkai.yaml`; both start with defaults, no `parse ...: EOF`, no warning line

**Checkpoint**: US1 + US2 validated

---

## Phase 5: User Story 3 - Starter template is fully commented out (Priority: P2)

**Goal**: `kekkai init` writes a comments-only file that runs as-is on defaults

**Independent Test**: quickstart.md scenario 3

- [X] T006 [US3] In internal/runtime/init.go `starterConfig`: comment out the active lines (`image:`, `node_version: lts`, `claude:`, `version: latest`, `args: "--dangerously-skip-permissions"`) so every line is a comment or blank; add one header line stating the file is optional and shown values are the defaults; keep commented example values equal to code defaults (research.md R4, contract); update the const's doc comment; `go build ./...` passes
- [X] T007 [US3] Run quickstart.md scenario 3: fresh `init` output has zero active lines (`grep -cv '^\s*\(#\|$\)' .kekkai.yaml` = 0), `up` runs on defaults without warning; uncomment `node_version` as `24` and verify a node:24 image build

**Checkpoint**: All stories validated

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T008 Update docs (FR-008, same commit as code per constitution I): SPECIFICATION.md §4.1 rewrite the **Required** paragraph (optional; absent = defaults + exact warning line; empty/comments-only = defaults silent; conflict rule kept), §3 `up` line mentions the warning, §4.5 rewrite for the fully commented starter (copy/paste rule now applies to commented values); README.md rewrite the "config file is required" (line ~74) and "Each project needs a .kekkai.yaml" (line ~78) statements to optional-with-defaults, document the warning; then run quickstart scenario 6 grep — zero stale hits
- [X] T009 Run quickstart scenarios 4–5 (regression + equivalence): configured project unchanged and warning-free; `.yml`+`.yaml` conflict and malformed-file errors intact; no-config run picks the same image tag and exit codes as an all-defaults-file run; final `go build ./... && go vet ./...`

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 (T002 → T003: T003 calls `config.Defaults()`) → US1/US2 in any order
- T006 (US3 code) only needs Phase 1; T007 needs T006 + T002 (empty-document path)
- Sequential preference: T004, T005 (P1 stories), then T006–T007, then polish
- T008 and T009 independent of each other, both after stories

## Implementation Strategy

MVP = Phases 1–3. US2 is validation-only (property of T002); US3 adds the one
story-specific code change (template). Total new code ~40 lines across
internal/config/config.go, internal/runtime/up.go, internal/runtime/init.go,
plus two documentation files.
