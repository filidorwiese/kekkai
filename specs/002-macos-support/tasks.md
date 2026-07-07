# Tasks: macOS Support

**Input**: Design documents from `/specs/002-macos-support/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: No unit-test tasks — constitution Principle IV mandates end-to-end validation instead; each story closes with its quickstart.md scenarios.

**Organization**: Tasks grouped by user story. SPECIFICATION.md + constitution amendments already landed (commits 0d9c280, 096ef3b).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 (sandboxed session, P1), US2 (fail-fast guidance, P2), US3 (installer, P3)

## Phase 1: Setup

**Purpose**: Confirm the codebase cross-compiles to the new target before touching behavior

- [ ] T001 Verify darwin/arm64 baseline build: `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/kekkai` from repo root; fix any accidental platform-specific code if it fails (none expected — pure Go, stdlib + yaml.v3)

---

## Phase 2: Foundational

No blocking prerequisites: the three stories touch disjoint mechanisms (run-args/firewall vs preflight vs distribution). Design docs and spec amendments (the usual foundation) are already merged.

**Checkpoint**: T001 green → all stories can start.

---

## Phase 3: User Story 1 - Mac user runs a sandboxed session (Priority: P1) 🎯 MVP

**Goal**: `kekkai up` (and all lifecycle commands) work on an Apple silicon Mac with Linux-identical confinement; Mac-host reachability parity per FR-008.

**Independent Test**: quickstart.md Scenarios 1-3 on Docker Desktop/Apple silicon; Scenario 7 on Linux (regression).

### Implementation for User Story 1

- [ ] T002 [US1] Platform-split `git.ssh_agent` in internal/runtime/up.go: on `runtime.GOOS == "darwin"` bind `/run/host-services/ssh-auth.sock:/ssh-agent` and skip the linux `$SSH_AUTH_SOCK` env check (research.md R2, SPECIFICATION §4.4/§5.2); linux path byte-for-byte unchanged; `SSH_AUTH_SOCK=/ssh-agent` env set on both
- [ ] T003 [P] [US1] Add `add_domain host.docker.internal warn` to the builtin hosts section of embed/init-firewall.sh (after statsig line, comment noting §5.4 darwin host-parity / warn+skip on linux); verification probes untouched
- [ ] T004 [US1] e2e validate on Docker Desktop/Apple silicon: quickstart.md Scenario 1 (happy path + lifecycle parity), Scenario 2 (confinement), Scenario 3 (host.docker.internal reachability), Scenario 4 first bullet (`ssh-add -l` lists host keys with forwarding on)
- [ ] T005 [US1] e2e validate Linux regression: quickstart.md Scenario 7 on a Linux box — no preflight container, firewall warn+skips host.docker.internal, probes pass (expected image rebuild once: firewall script is a hash input)

**Checkpoint**: MVP — Mac users with a correctly configured runtime have a fully working sandbox.

---

## Phase 4: User Story 2 - Missing capabilities fail fast with runtime-specific guidance (Priority: P2)

**Goal**: darwin preflight probes (bind-ability + agent socket) abort before sandbox work with contract-format errors; runtime identity decorates hints only (FR-003/FR-004, contracts/preflight.md).

**Independent Test**: quickstart.md Scenarios 4 (colima without `--ssh-agent`) and 5 (unshared path) fail at preflight with the exact hint table text; unknown runtime with passing probes proceeds.

### Implementation for User Story 2

- [ ] T006 [P] [US2] Add `Info()` to internal/docker/cli.go returning server OperatingSystem and Name via `docker info --format '{{.OperatingSystem}}|{{.Name}}'`
- [ ] T007 [US2] Create internal/runtime/preflight_darwin.go (`//go:build darwin`) + internal/runtime/preflight_stub.go (`//go:build !darwin`, no-op): RuntimeIdentity enum (DockerDesktop/OrbStack/Colima/Unknown, matching per research.md R3, detection invoked only on failure) and the hint table from contracts/preflight.md
- [ ] T008 [US2] Implement the probe in internal/runtime/preflight_darwin.go and wire into internal/runtime/up.go after `ensureImage`: one `docker run --rm` from the built image, ro-binds of $PWD, ~/.claude, ~/.gitconfig (when git.enabled), each resolved non-skipped disk.mount source, agent socket when ssh_agent; cmd `test -S /ssh-agent` else `true`; failure → stderr format `kekkai: preflight failed — <capability>` + detail + fix line, exit 1 (contracts/preflight.md; data-model.md PreflightCheck)
- [ ] T009 [US2] e2e validate: quickstart.md Scenario 5 on Docker Desktop (unshared path → File Sharing hint, no sandbox container created); Scenario 4 second bullet on colima without `--ssh-agent` [community tier — simulate via faked identity if no colima available]; confirm error text matches contract verbatim (SC-004)

**Checkpoint**: US1 + US2 — every known blocking condition self-diagnoses before sandbox creation.

---

## Phase 5: User Story 3 - Mac user installs kekkai the standard way (Priority: P3)

**Goal**: darwin/arm64 release artifact + installer support with Intel refusal (contracts/distribution.md).

**Independent Test**: quickstart.md Scenario 6.

### Implementation for User Story 3

- [ ] T010 [P] [US3] Extend .github/workflows/release.yml matrix to explicit os/arch includes: linux/amd64, linux/arm64, darwin/arm64; tarball naming `kekkai_${VERSION}_${GOOS}_${GOARCH}.tar.gz`; all three in one SHA256SUMS (contracts/distribution.md)
- [ ] T011 [P] [US3] Update install.sh: accept `Darwin`+arm64 → darwin_arm64 tarball; `Darwin`+x86_64 → refuse "kekkai supports Apple silicon Macs only (Intel Macs are unsupported)" exit 1; other OS message → "Linux and macOS only"; checksum via `sha256sum` when present else `shasum -a 256` (contracts/distribution.md)
- [ ] T012 [US3] e2e validate: quickstart.md Scenario 6 on Apple silicon after next release tag (or against a test tag/KEKKAI_REPO fork); verify Intel refusal via `uname -m` shim; `kekkai version` works from a new shell

**Checkpoint**: All three stories independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T013 [P] README.md digest update per contracts/distribution.md documentation contract: prerequisites (macOS Apple silicon + runtime list), support tiering (Docker Desktop maintainer-validated; others capability-probed, community-validated), known limitations (slower shared-folder I/O; localhost-bound Mac services reachable; colima `--ssh-agent`)
- [ ] T014 [P] Fix base-image doc drift: SPECIFICATION.md §4.2 example and README config example still say `node:22-bookworm`, code default is `node:24-trixie` (violates §4.5 copy/paste safety; pre-existing, flagged 2026-07-07)
- [ ] T015 Full release sign-off: run the quickstart.md checklist end-to-end (Scenarios 1-7) on Docker Desktop/Apple silicon + Linux; verify SC-001 (install→session < 10 min) and SC-003 (firewall probes on every up)

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1 (T001): start immediately; blocks everything (proves the target builds)
- Phase 2: empty — no gate beyond T001
- US1, US2, US3: independent of each other after T001, EXCEPT: T008 edits internal/runtime/up.go which T002 also edits — do T002 before T008 (or accept a merge)
- Polish: T013/T014 anytime; T015 after all stories

### Story Dependency Notes

- US2 uses the image tag from `ensureImage` (existing code) — no US1 code dependency besides the shared up.go file ordering above
- US3 is fully independent (shell + YAML only)
- Validation tasks (T004, T005, T009, T012, T015) need real hardware: Apple silicon + Docker Desktop (maintainer tier), a Linux box, optionally colima [community]

### Parallel Opportunities

```text
After T001:
  Lane A (Go, US1→US2): T002 → T003(P) → T008 path: T002 → T008
  Lane B (Go, US2):     T006, T007 in parallel with Lane A until T008
  Lane C (distribution, US3): T010, T011 in parallel with everything
  Docs: T013, T014 in parallel with everything
Validation serializes on hardware: T004/T005 → T009 → T012 → T015
```

---

## Implementation Strategy

**MVP first**: T001 → T002 + T003 → T004/T005. That alone delivers a working Mac sandbox for correctly configured Docker Desktop setups (US1, the entire feature's core value). Ship-ready checkpoint.

**Increment 2**: T006-T009 makes misconfigured setups self-diagnosing (US2) — the difference between "works for me" and supportable.

**Increment 3**: T010-T012 opens the standard install path (US3); until then, Mac users build from source.

**Wrap**: T013-T015 docs + sign-off.

---

## Notes

- 15 tasks total: US1=4, US2=4, US3=3, setup=1, polish=3
- No new dependencies, no new config keys — any temptation otherwise re-opens plan.md
- Firewall edit (T003) changes the image hash: every user rebuilds once on upgrade; expected, not a bug
- Commit per task or logical group; spec-first already satisfied for design changes
