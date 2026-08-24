# Tasks: Builtin Python Runtime

**Input**: Design documents from `/specs/024-python-builtin/`

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

**Purpose**: The two code changes every story builds on (disjoint files, parallelizable).

- [X] T001 [P] Append the "scripting runtime (specs/024)" group to `builtinAptPackages` in internal/runtime/up.go: `python3`, `python3-venv`, `python3-pip`, as a distinct comment-labeled group after the general-tooling group, in this order (research.md R1; contracts/python-runtime.md package set)
- [X] T002 [P] Add `&& printf '[global]\nbreak-system-packages = true\n' > /etc/pip.conf \` to the first root RUN block in embed/Dockerfile.tmpl (the apt block, before `rm -rf /var/lib/apt/lists/*`), with a short comment citing PEP 668 + disposable-sandbox rationale (research.md R3; contracts/python-runtime.md pip.conf contract — exact file content `[global]\nbreak-system-packages = true\n`)

**Checkpoint**: `go build ./cmd/kekkai` clean; all user stories can start.

---

## Phase 3: User Story 1 - Agent runs ad-hoc Python scripts out of the box (Priority: P1) 🎯 MVP

**Goal**: `python3` + full stdlib work in a fresh no-config sandbox from every exec path.

**Independent Test**: quickstart.md S1 (`python3 --version`, stdlib import one-liner, `pip --version`).

### Implementation for User Story 1

- [X] T003 [US1] Validate e2e: `go build -o kekkai-test ./cmd/kekkai`, `up` in a no-config scratch dir (pseudo-TTY trick), then quickstart.md S1 — `python3 --version` reports 3.13.x, `import json,csv,sqlite3,urllib.request` succeeds, `pip --version` resolves, all via `kekkai-test exec` (depends on T001, T002)

**Checkpoint**: MVP — ad-hoc python scripting needs zero configuration.

---

## Phase 4: User Story 2 - Agent installs Python packages without PEP 668 friction (Priority: P2)

**Goal**: Plain `pip install` never hits the externally-managed-environment guard; venvs work; firewall still governs egress (block without opt-in, success with `pypi.org` + `files.pythonhosted.org` in `allowed_domains`).

**Independent Test**: quickstart.md S2 (venv) and S3 (guard-vs-firewall separation, then opt-in install).

### Implementation for User Story 2

- [X] T004 [US2] Validate e2e on the same sandbox as T003: quickstart.md S2 — `python3 -m venv /tmp/v` creates a venv whose `/tmp/v/bin/pip --version` works; then S3 part 1 — `pip install six` with no network config FAILS with a network/firewall error and NOT `externally-managed-environment` (depends on T001, T002)
- [X] T005 [US2] Validate e2e: add `network: {allowed_domains: [pypi.org, files.pythonhosted.org]}` to the scratch config, `down`+`up` (expect NO rebuild — runtime input), then quickstart.md S3 part 2 — `pip install six` succeeds first attempt (non-root `--user` fallback into ~/.local), `python3 -c 'import six'` works; firewall verification probes passed on both runs (depends on T004)

**Checkpoint**: Guard removed, boundary intact — the clarified egress posture proven.

---

## Phase 5: User Story 3 - Existing projects upgrade cleanly (Priority: P2)

**Goal**: Exactly one rebuild per config on upgrade, reuse afterwards; offline-fallback label changed; `apt_packages` duplicates harmless; image growth < 120 MB.

**Independent Test**: quickstart.md S4 (old-binary tag A → new-binary tag B ≠ A → reuse B) and S5 (size delta).

### Implementation for User Story 3

- [X] T006 [US3] Validate e2e: build pre-feature binary from the parent commit (git worktree), run quickstart.md S4 (one rebuild, then reuse; pin claude.version so registry drift can't force a second rebuild; `apt_packages: [python3]` config builds and python3 runs) and S5 (image size delta < 120 MB, expect ~45 MB); confirm the two binaries' images carry different `kekkai.config_hash` labels (contracts/python-runtime.md identity; research.md R4) (depends on T001, T002 — needs the final rendered template)

**Checkpoint**: Identity semantics proven; no churn beyond the one-time rebuild.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: FR-007/FR-008 documentation (spec-first: lands in the same change as the code) and full-sweep validation.

- [X] T007 [P] Amend SPECIFICATION.md: §5.1 builtin package list gains the three packages as the scripting-runtime group (one-line rationale: ad-hoc agent scripting; daemon-free, no new network destinations, PyPI egress stays user opt-in), §6.3 gains a pip.conf bullet (path, exact content, PEP 668 rationale, §5.4 unchanged) (research.md R3–R5)
- [X] T008 [P] Amend README.md: note under the network/allowed_domains section that runtime `pip install` needs `pypi.org` + `files.pythonhosted.org` in `network.allowed_domains` (FR-007, clarification 2026-08-24; same posture as npm registry)
- [X] T009 Full validation sweep: `go vet ./...`, `go build ./cmd/kekkai`, re-confirm quickstart.md S1–S2 on the final binary, firewall probes unchanged, docs grep (SPECIFICATION.md + README mention python3/pip.conf/pypi opt-in) (depends on all prior tasks)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: T001 ∥ T002 (disjoint files) block everything downstream.
- **US1 (Phase 3)**: T003 after T001+T002. **US2 (Phase 4)**: T004 after T001+T002 (shares T003's sandbox); T005 after T004. **US3 (Phase 5)**: T006 after T001+T002.
- **Polish (Phase 6)**: T007/T008 anytime after design is stable; T009 last.

### User Story Dependencies

- US1: none (post-Foundational). US2: no code of its own — validates T002's pip.conf + the clarified egress posture; e2e shares the built image with US1. US3: no code of its own — verifies identity effects of T001/T002.

### Parallel Opportunities

- T001 (up.go) ∥ T002 (Dockerfile.tmpl) ∥ T007 (SPECIFICATION.md) ∥ T008 (README.md) — disjoint files.
- T003 and T004 share one build + live sandbox (single quickstart session covers S1+S2); T005 reuses the same image (runtime config change only).

## Parallel Example: immediately

```bash
Task: "Append scripting runtime group to builtinAptPackages in internal/runtime/up.go"  # T001
Task: "Add pip.conf printf line to embed/Dockerfile.tmpl apt RUN block"                 # T002
Task: "Amend SPECIFICATION.md §5.1 and §6.3"                                            # T007
Task: "Amend README.md network docs with pypi opt-in note"                              # T008
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. T001+T002 → T003. **STOP and VALIDATE**: interpreter + stdlib + pip presence in a fresh sandbox. (T002 is foundational, not US2-only: the pip.conf line must be in the image before the first build to avoid a second full image build during validation.)

### Incremental Delivery

1. T001+T002 (code) → T003/T004 on one sandbox (US1 + US2 guard/venv) → T005 (US2 opt-in install, same image) → T006 (US3 identity, adds the old-binary comparison build) → T007/T008/T009 (docs + sweep). One image build serves US1/US2.
