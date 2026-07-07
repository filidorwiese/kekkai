# Tasks: Self-Update Command

**Input**: Design documents from `/specs/003-self-update/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/self-update-cli.md, quickstart.md

**Tests**: No unit-test tasks; constitution Principle IV mandates end-to-end validation. Each story closes with its quickstart.md scenarios.

**Organization**: Tasks grouped by user story. Nearly all Go work lands in one new file (`internal/selfupdate/selfupdate.go`), so story phases serialize; the split still lets each story ship and validate independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 (update to latest, P1), US2 (already up to date, P2), US3 (clear failures, P3)

## Phase 1: Setup

**Purpose**: Entry point exists and the repo still builds everywhere

- [X] T001 Create internal/selfupdate/selfupdate.go with `Run(version string) error` stub and wire cmd/kekkai/main.go: `case "self-update": err = selfupdate.Run(version)`, reject extra args with a usage error, add help line `  self-update update kekkai to the latest release` (contracts/self-update-cli.md); verify `go build ./...` and `GOOS=darwin GOARCH=arm64 go build ./...`

---

## Phase 2: Foundational

**Purpose**: Spec-first (constitution Principle I): the design change is recorded in the authoritative spec and MUST be committed together with the code of Phase 1/3

- [X] T002 Amend SPECIFICATION.md per plan.md delta: §3 CLI block add `kekkai self-update # update the binary to the latest GitHub release` + semantics bullet (output strings, dev-build refusal, KEKKAI_REPO); §10 add self-update bullet (same artifacts as install.sh, SHA256SUMS verified, atomic replace); §11 delete the "`kekkai update` self-updater." line

**Checkpoint**: spec amendment staged; commit together with T001 (and subsequent code) when a commit is requested

---

## Phase 3: User Story 1 - Update to the latest release (Priority: P1) 🎯 MVP

**Goal**: `kekkai self-update` on an older release replaces the binary with the latest and prints `Updated kekkai <from> -> <to>` (FR-001, FR-003, FR-004, FR-005)

**Independent Test**: quickstart.md Scenario 1 (and 7 for interrupt safety)

### Implementation for User Story 1

- [X] T003 [US1] Release lookup in internal/selfupdate/selfupdate.go: GET `https://api.github.com/repos/${KEKKAI_REPO:-filidorwiese/kekkai}/releases/latest` via net/http (10s timeout), parse `tag_name` with encoding/json; error text `could not determine the latest release of <repo>: <cause>` (research.md R1, contracts/self-update-cli.md)
- [X] T004 [US1] Semver compare in internal/selfupdate/selfupdate.go: parse `vMAJOR.MINOR.PATCH` numerically, tolerate pre-release suffix; returns older/equal/newer (research.md R2)
- [X] T005 [US1] Download + verify + extract in internal/selfupdate/selfupdate.go: fetch `kekkai_<tag>_<GOOS>_<GOARCH>.tar.gz` + `SHA256SUMS` from the release download URL, print `downloading kekkai <tag> (<goos>/<goarch>)`, sha256-verify tarball BEFORE extraction, pull the `kekkai` entry via archive/tar + compress/gzip (research.md R3)
- [X] T006 [US1] Atomic replace in internal/selfupdate/selfupdate.go: resolve os.Executable() + filepath.EvalSymlinks, write to `<dir>/.kekkai-update-<pid>`, chmod 0755, os.Rename over target, remove temp on every failure path; success output `Updated kekkai <from> -> <to>` exit 0 (research.md R4, data-model.md state order)
- [X] T007 [US1] e2e validate quickstart.md Scenario 1 (older release updates, version flips) and Scenario 7 (Ctrl-C mid-download twice: old binary intact, no stray temp files)

**Checkpoint**: MVP; a user on an old release reaches the latest with one command

---

## Phase 4: User Story 2 - Already up to date (Priority: P2)

**Goal**: Current or ahead-of-release installs get the exact no-op messages with zero filesystem writes (FR-002)

**Independent Test**: quickstart.md Scenario 2

### Implementation for User Story 2

- [X] T008 [US2] Up-to-date outcomes in internal/selfupdate/selfupdate.go: equal → `You're on the latest version (<installed>)`; installed newer → `You're ahead of the latest release (<installed> > <latest>)`; both exit 0, return before any download or temp file (contracts/self-update-cli.md, SC-003)
- [X] T009 [US2] e2e validate quickstart.md Scenario 2: message exact, mtime unchanged, no temp files

**Checkpoint**: US1 + US2; both happy paths contract-exact

---

## Phase 5: User Story 3 - Clear failure when updating is not possible (Priority: P3)

**Goal**: Every blocked/failed path prints a distinct actionable error, exit 1, installed binary untouched (FR-006..FR-009)

**Independent Test**: quickstart.md Scenarios 3-6

### Implementation for User Story 3

- [X] T010 [US3] Guards in internal/selfupdate/selfupdate.go, before any network I/O: version `dev`/non-`v` → dev-build refusal message with install.sh one-liner; writability preflight (open-for-write probe on target file + dir) → `cannot update: <path> is not writable; fix permissions or reinstall via install.sh` (research.md R4/R5, FR-007/FR-008)
- [X] T011 [US3] Error mapping in internal/selfupdate/selfupdate.go per contracts/self-update-cli.md: HTTP 403/429 → `GitHub API rate limit hit; try again later`; tarball 404 → `no <goos>/<goarch> artifact in release <tag>`; sha256 mismatch → `checksum verification FAILED for <tarball>` (install.sh wording); other transfer errors → `download failed: <cause>`; all exit 1, temp file cleaned
- [X] T012 [US3] e2e validate quickstart.md Scenario 3 (dev build), 4 (chmod 555, fails before "downloading" line), 5 (bad repo/offline), 6 (fork with corrupted SHA256SUMS) [fork needed for 6]

**Checkpoint**: all three stories independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T013 [P] README.md digest update: add `self-update` to the command list and one line on semantics (latest release, checksum-verified, dev builds refuse), per plan.md delta and Principle I (README = digest of spec)
- [X] T014 Full sign-off: run the quickstart.md checklist (Scenarios 1-5 on Linux, Scenario 1 on macOS once a darwin-artifact release exists), confirm output strings match contracts/self-update-cli.md verbatim (SC-004) and both builds from T001 stay green

---

## Dependencies & Execution Order

### Phase Dependencies

- T001 first (everything lands in files it creates/touches)
- T002 any time before the first code commit; MUST be in the same commit as the code (Principle I)
- US1 (T003-T006) sequential: one file, each step feeds the next per data-model.md state order (guards → lookup → compare → download → verify → extract → rename)
- US2 (T008) needs T003+T004 (lookup + compare); independent of T005/T006
- US3 (T010, T011) needs T001 only for T010 (guards run first in the flow) and T005 for T011's checksum/artifact errors
- Polish: T013 any time; T014 last

### Parallel Opportunities

```text
Nearly everything shares internal/selfupdate/selfupdate.go: serialize the Go work.
Parallel lanes: T002 (SPECIFICATION.md) and T013 (README.md) alongside any Go task.
Validation order: T007 → T009 → T012 → T014 (same host/fork setup reused).
```

### Within-file note

T003-T006, T008, T010, T011 all edit internal/selfupdate/selfupdate.go; execute in ID order to avoid conflicts. IDs are sequenced so the file grows in the data-model.md state order.

---

## Implementation Strategy

**MVP first**: T001 → T002 → T003-T006 → T007. Delivers the entire headline capability (update with from/to output). Ship-ready checkpoint.

**Increment 2**: T008-T009, the most common real-world invocation ("You're on the latest version").

**Increment 3**: T010-T012, failure hardening; the command becomes safe to recommend broadly.

**Wrap**: T013-T014 docs + sign-off.

---

## Notes

- 14 tasks: setup=1, foundational=1, US1=5, US2=2, US3=3, polish=2
- Zero new dependencies, zero new config keys; any temptation otherwise re-opens plan.md
- Scenario 6 (checksum mismatch) needs a fork with an editable release (`KEKKAI_REPO`); everything else runs against the real repo
- Commit per task or logical group ONLY when a commit is requested; T002 must ride the same commit as the first code
