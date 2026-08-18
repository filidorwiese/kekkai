# Tasks: OpenSSH Client in Sandbox Image

**Input**: Design documents from `/specs/019-openssh-client/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/package-set.md, quickstart.md

**Tests**: No unit-test tasks (constitution IV: e2e validation only). Validation tasks follow quickstart.md.

**Organization**: Tiny feature — one shared code change (Foundational) that both stories consume, then per-story e2e validation.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

No setup needed — existing repo, no new dependencies or structure.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The package-set change itself + mandatory spec amendment (constitution I: same commit). Both user stories depend on this and nothing else.

- [x] T001 Add `"openssh-client"` to `builtinAptPackages` in `internal/runtime/up.go` (~line 26), grouped with git tooling and a short why-comment (SSH signing + `git@` remotes via forwarded agent; baked unconditionally per §6.1)
- [x] T002 Amend SPECIFICATION.md §5.1 apt package list (~line 142): add `openssh-client` with rationale "(SSH signing + `git@` remotes via forwarded agent, §5.2)" — must land in same commit as T001
- [x] T003 Build binary: `go build ./cmd/kekkai` (or `task build`); confirm compile

**Checkpoint**: Binary built with new builtin set. Rendered Dockerfile now includes `openssh-client`; image hash changes once.

---

## Phase 3: User Story 1 — SSH commit signing inside the sandbox (Priority: P1) 🎯 MVP

**Goal**: With host SSH-signing git config and `git.ssh_agent: true`, commits made inside the sandbox are signed and verifiable.

**Independent Test**: quickstart.md steps 1–6; deliverable is a signed, verifiable commit created inside the sandbox with zero in-sandbox config.

- [x] T004 [US1] E2E: run `./kekkai up` in a test project with `git: {enabled: true, ssh_agent: true}`; expect one rebuild, firewall probes pass (pseudo-TTY via `script -qec ... /dev/null` per memory note); verify `kekkai exec 'command -v ssh ssh-keygen ssh-add'` resolves all three
- [x] T005 [US1] E2E: `kekkai exec 'ssh-add -l'` lists host agent keys; `kekkai exec 'cd /workspace && git commit --allow-empty -m sign-test'` succeeds; `git log --show-signature -1` shows good SSH signature (quickstart steps 4–6)

**Checkpoint**: P1 delivered — signing works end-to-end.

---

## Phase 4: User Story 2 — SSH-based git remotes (Priority: P2)

**Goal**: git over `git@`/`ssh://` remotes authenticates via forwarded agent on allowed hosts.

**Independent Test**: `git ls-remote` against an SSH remote inside the sandbox succeeds (needs only T001–T003, not US1).

- [x] T006 [US2] E2E: in test project with `ssh_agent: true` + `allow_github: true`, run `kekkai exec 'git ls-remote git@github.com:<user-owned-repo>.git'`; expect agent auth success (contracts/package-set.md row 5)

**Checkpoint**: P2 delivered — SSH transport works.

---

## Phase 5: Polish & Cross-Cutting Validation

**Purpose**: Identity-contract checks and edge cases from spec + contracts.

- [x] T007 [P] E2E no-rebuild check: flip `ssh_agent: false`, run `up` → zero docker build activity; flip back (contracts identity contract; FR-004)
- [x] T008 [P] E2E duplicate tolerance: set `image: {apt_packages: [openssh-client]}`, run `up` → build succeeds, no apt error; remove again (FR-005)
- [x] T009 Mark quickstart.md outcomes verified; note any deviations in specs/019-openssh-client/ before commit

---

## Dependencies

```text
T001 ─┬─ T002 (same commit) ─→ T003 ─→ T004 ─→ T005   (US1)
      │                          └───→ T006            (US2, independent of US1 e2e)
      │                          └───→ T007, T008 [P]  (polish)
      └────────────────────────────────→ T009 (last)
```

- US1 (T004–T005) and US2 (T006) are independent of each other; both need only Phase 2.
- T007/T008 parallelizable with each other and with US2 (separate config states — run sequentially in one checkout, or note they mutate the same `.kekkai.yaml`).

## Parallel Execution Examples

- After T003: T004 and T006 can run concurrently in separate test projects.
- T007 ∥ T008 only with separate test projects (both edit `.kekkai.yaml`); otherwise sequential.

## Implementation Strategy

MVP = Phase 2 + Phase 3 (T001–T005): the code change plus signed-commit proof. Phase 4 and 5 are confirmation passes with no additional code; total code delta is one Go list entry and one spec sentence.
