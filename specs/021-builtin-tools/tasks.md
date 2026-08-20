# Tasks: Builtin Archive, File-Inspection, and Search Tools

**Input**: Design documents from `/specs/021-builtin-tools/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation against a real docker daemon; each story closes with its quickstart scenario(s).

**Organization**: Tasks grouped by user story; stories are independently implementable and testable after the Foundational phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1–US3 from spec.md

## Phase 1: Setup

No setup tasks — existing single-binary Go codebase; the feature is a constant extension plus one template line (plan.md Structure Decision).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The one shared code change every story builds on.

- [X] T001 Append the "general tooling (specs/021)" group to `builtinAptPackages` in internal/runtime/up.go: `unzip`, `zip`, `xz-utils`, `zstd`, `bzip2`, `file`, `ripgrep`, `fd-find`, `rsync`, as a distinct comment-labeled group after the convenience group, in this order (research.md R3; contracts/image-tools.md package set)

**Checkpoint**: `go build ./cmd/kekkai` clean; all user stories can start.

---

## Phase 3: User Story 1 - Agent works with common archive and file formats out of the box (Priority: P1) 🎯 MVP

**Goal**: `unzip`, `zip`, `xz`, `zstd`, `bzip2`, `file`, `rsync` all run in a fresh no-config sandbox; archive round-trips succeed.

**Independent Test**: quickstart.md S1 (US1 subset — the seven non-search tools) and S2 (zip/txz/tzst round-trips).

### Implementation for User Story 1

- [X] T002 [US1] Validate e2e: `go build -o kekkai-test ./cmd/kekkai`, `up` in a no-config scratch dir, live sandbox via pseudo-TTY, then quickstart.md S1 for `unzip zip xz zstd bzip2 file rsync` (all resolve + version) and S2 (`ROUNDTRIP-OK`) (depends on T001)

**Checkpoint**: MVP — archive/file work needs zero configuration.

---

## Phase 4: User Story 2 - Agent searches the workspace with standard fast-search tools (Priority: P2)

**Goal**: `rg` and `fd` work under their canonical names from every exec path; `fdfind` keeps working.

**Independent Test**: quickstart.md S1 (search subset: `rg`, `fd`, `fdfind` resolve; `rg`/`fd` return matches against /workspace).

### Implementation for User Story 2

- [X] T003 [P] [US2] Add `ln -s /usr/bin/fdfind /usr/local/bin/fd` to the existing root-owned symlink RUN block (the node/npm/npx/claude block) in embed/Dockerfile.tmpl, per contracts/image-tools.md — same RUN, no new layer (research.md R2)
- [X] T004 [US2] Validate e2e: rebuild binary + image, then quickstart.md S1 search subset — `command -v fd` → `/usr/local/bin/fd`, `command -v fdfind` → `/usr/bin/fdfind`, `rg <pattern>` and `fd <name>` return matches in /workspace from `kekkai-test exec` (depends on T001, T003)

**Checkpoint**: Canonical names contract satisfied.

---

## Phase 5: User Story 3 - Existing projects upgrade cleanly (Priority: P2)

**Goal**: Exactly one rebuild per config on upgrade, reuse afterwards; offline-fallback label changed; `apt_packages` duplicates harmless; image growth < 50 MB.

**Independent Test**: quickstart.md S3 (old-binary tag A → new-binary tag B ≠ A → reuse B) and S5 (size delta).

### Implementation for User Story 3

- [X] T005 [US3] Validate e2e: build pre-feature binary from the parent commit (git worktree), run quickstart.md S3 (one rebuild, then reuse; pin claude.version; `apt_packages: [ripgrep]` config builds and `rg` runs) and S5 (image size delta < 50 MB); confirm the two binaries' images carry different `kekkai.config_hash` labels (contracts/image-tools.md identity) (depends on T001, T003 — needs the final rendered template)

**Checkpoint**: Identity semantics proven; no churn beyond the one-time rebuild.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: FR-006 documentation (spec-first: lands in the same change as the code) and full-sweep validation.

- [X] T006 [P] Amend SPECIFICATION.md: §5.1 builtin package list gains the nine packages as the general-tooling group (with one-line rationale), §6.3 exec-path bullet gains the `fd` symlink alongside node/npm/npx/claude (research.md R7)
- [X] T007 Full validation sweep: `go vet ./...`, `go build ./cmd/kekkai`, quickstart.md S4 (firewall probes unchanged, deb.debian.org unreachable at runtime) and S6 (docs grep), re-confirm S1–S2 pass on the final binary

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: T001 blocks everything.
- **US1 (Phase 3)**: T002 after T001 only. **US2 (Phase 4)**: T003 independent of T001 (different file, can run in parallel with it); T004 after T001+T003. **US3 (Phase 5)**: T005 after T001+T003 (identity check needs the final template).
- **Polish (Phase 6)**: T006 anytime after design is stable; T007 last.

### User Story Dependencies

- US1: none (post-Foundational). US2: T003 is its own code; e2e shares the built image with US1. US3: no code of its own — verifies identity effects of T001/T003.

### Parallel Opportunities

- T001 (up.go) ∥ T003 (Dockerfile.tmpl) ∥ T006 (SPECIFICATION.md) — disjoint files.
- T002 and T004 can share one build + live sandbox (single quickstart session covers S1+S2 for both stories).

## Parallel Example: immediately

```bash
Task: "Append general tooling group to builtinAptPackages in internal/runtime/up.go"  # T001
Task: "Add fd symlink line to embed/Dockerfile.tmpl"                                  # T003
Task: "Amend SPECIFICATION.md §5.1 and §6.3"                                          # T006
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. T001 → T002. **STOP and VALIDATE**: seven tools + round-trips in a fresh sandbox. US2 costs one template line more — in practice ship T001+T003 together to avoid a second full image build during validation.

### Incremental Delivery

1. T001+T003 (code) → T002/T004 on one sandbox (US1+US2 e2e) → T005 (US3 identity) → T006/T007 (docs + sweep). One image build serves US1/US2; US3 adds the old-binary comparison build.
