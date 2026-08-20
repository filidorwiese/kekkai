# Tasks: Quiet Interactive Build Progress Without Custom Apt Repos

**Input**: Design documents from `/specs/022-quiet-build-progress/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/build-output.md, quickstart.md

**Tests**: No unit-test tasks. Validation is end-to-end per constitution IV — quickstart.md scenarios against a real docker daemon, mapped into each story's phase below.

**Organization**: Tasks grouped by user story. Both stories are P1 and ship together (spec: "both stories together are the whole feature"), but each validates independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files / independent scenarios)
- **[Story]**: US1 (quiet default builds), US2 (hint retained with repos)

---

## Phase 1: Setup

**Purpose**: Confirm a clean baseline before touching code

- [X] T001 Verify clean baseline: `go build ./cmd/kekkai && go vet ./...` pass from repo root before any change

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The whole implementation — a `capture` parameter threaded through two files. Both user stories are behaviors of this one change, so it is foundational to both.

**⚠️ CRITICAL**: T002 → T003 → T004 are sequential (T003 depends on T002's new signature).

- [X] T002 Add `capture bool` parameter to `BuildImage` in internal/docker/cli.go (line 60): signature becomes `BuildImage(tag, contextDir string, labels map[string]string, verbose, capture bool) (string, error)`. When `capture` is false: `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr` (child inherits real fds so BuildKit's `auto` progress detects the terminal), return `""` for output on both success and failure. When `capture` is true: keep the existing `io.MultiWriter(os.Std*, &captured)` tee and buffer return exactly as-is. Args/labels assembly, `--progress=plain` on verbose, and the `docker build failed: %w` error text stay identical in both modes (contracts/build-output.md). Update the doc comment to describe both modes.
- [X] T003 Update `buildImage` in internal/runtime/up.go (line 356) to pass `len(aptRepos) > 0` as the new `capture` argument: `docker.BuildImage(tag, dir, labels, verbose, len(aptRepos) > 0)`. No other change — the `aptSignatureHint` call already no-ops on an empty repo slice with empty output (contracts/build-output.md call-site section).
- [X] T004 Compile and vet: `go build ./cmd/kekkai && go vet ./...` pass; grep confirms no other `docker.BuildImage` call sites exist

**Checkpoint**: Fix is complete in code — both stories can now be validated in parallel

---

## Phase 3: User Story 1 - Default builds show the compact progress display again (Priority: P1) 🎯 MVP

**Goal**: Repo-less configs (the default case) get docker's compact auto-collapsing TTY progress back, exactly as pre-020; verbose still forces plain; no hint machinery fires.

**Independent Test**: From an interactive terminal (pseudo-TTY), trigger a build with a repo-less config and observe compact markers (`[+] Building` present, no `#1 [internal] load build definition` step lines).

### Validation for User Story 1 (quickstart.md S1, S3, S5)

- [X] T005 [US1] Run quickstart S1 (SC-001): `go build -o kekkai-test ./cmd/kekkai`; in a fresh dir with a repo-less config (vary `apt_packages`, e.g. `[sl]`, to force a build), run `timeout 20 script -qec "…/kekkai-test up" /dev/null > s1.log 2>&1`; verify `grep -c '\[+\] Building' s1.log` ≥ 1 and `grep -c '#1 \[internal\] load build' s1.log` = 0
- [X] T006 [P] [US1] Run quickstart S3 quiet-mode half (FR-004, SC-004): repeat the S1 pty run with `up --verbose` and another `apt_packages` variant to force a build; verify plain step lines appear even on a terminal
- [X] T007 [P] [US1] Run quickstart S5 (FR-003 no-repos side): force a build failure without `apt_repos` (e.g. `apt_packages: [kekkai-no-such-package-xyz]`); verify build fails, exit non-zero, zero `hint:` lines on stderr

**Checkpoint**: Quiet mode restored and verified — the regression is fixed

---

## Phase 4: User Story 2 - Repo users keep the signature-failure hint (Priority: P1)

**Goal**: With ≥1 `apt_repos` entry, build output is still captured and the specs/020 signature-failure hint contract holds byte-for-byte.

**Independent Test**: Re-run feature 020's signature-failure scenario (repo needing a key, `key_url` omitted) and verify the hint appears after the build error with unchanged wording and exit status.

### Validation for User Story 2 (quickstart.md S2, S3)

- [X] T008 [US2] Run quickstart S2 (SC-002): config with Dart repo entry **without** `key_url` and `apt_packages: [dart]` per specs/020 quickstart S5; run `up` (non-TTY fine, fails at `apt-get update` in ~60–90 s); verify NO_PUBKEY in output, hint line `hint: image.apt_repos[0] (dart): … add key_url …` on stderr after the build error, exit non-zero, and the transcript is plain (step lines present — capture mode)
- [X] T009 [P] [US2] Run quickstart S3 capture-mode half (FR-004): repeat the S2 run with `--verbose`; verify behavior identical to today (plain progress, same hint on failure)

**Checkpoint**: Both stories verified — feature complete

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Identity-preservation proof and final sweep

- [X] T010 Run quickstart S4 (FR-005, SC-003): with an image built by the pre-fix binary present, run the fixed binary's `up` with the same config and verify no `building image` line (tag reused); confirm the `ConfigHash` label of a freshly built image equals the pre-fix binary's for the same config
- [X] T011 Run quickstart S6 sweep: `go vet ./...`, `go build ./cmd/kekkai`; confirm by inspection no changes near the firewall path; standard probes observed during any live run

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none — start immediately
- **Foundational (Phase 2)**: after Setup; T002 → T003 → T004 strictly sequential (signature change ripples); BLOCKS all validation
- **US1 (Phase 3) / US2 (Phase 4)**: both only need Phase 2; fully independent of each other — run in parallel or in any order
- **Polish (Phase 5)**: T010 needs Phase 2 (and benefits from a preserved pre-fix image); T011 last

### Parallel Opportunities

- T006 and T007 after T005 built the test binary (independent scenarios, separate config dirs)
- T009 after T008 (reuses its config)
- Phase 3 and Phase 4 entirely in parallel (different configs, independent daemon builds)

---

## Parallel Example

```bash
# After Phase 2 checkpoint, validate both stories concurrently:
Task: "T005 quickstart S1 — compact markers via pty (repo-less config)"
Task: "T008 quickstart S2 — 020 hint regression gate (Dart repo, no key_url)"
```

---

## Implementation Strategy

Single increment — this is a 2-file regression fix:

1. Phase 1 baseline → Phase 2 implementation (T002–T004): the fix is done
2. Phase 3 (US1) proves the regression is gone — MVP checkpoint
3. Phase 4 (US2) proves the 020 contract survived — both P1 stories ship together
4. Phase 5 proves zero rebuilds and clean sweep

MVP scope: Phases 1–3 (US1 is the user-visible fix); do not ship without Phase 4 (spec: both stories are the whole feature).
