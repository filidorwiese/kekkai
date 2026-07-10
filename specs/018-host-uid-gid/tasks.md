# Tasks: Host UID/GID Match

**Input**: Design documents from `/specs/018-host-uid-gid/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/sandbox-identity.md, quickstart.md

**Tests**: Not requested; per constitution IV validation is end-to-end (quickstart.md scenarios), included as validation tasks.

**Organization**: Tasks grouped by user story. Note: US1 owns all `embed/Dockerfile.tmpl` edits (single file) — the collision guard (US3/FR-005) is two shell tokens inside the same RUN line, so it lands with US1 to avoid same-file conflicts; US3 keeps its own validation.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Baseline check: `go vet ./... && go build -o kekkai ./cmd/kekkai` succeeds on current main (repo root)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The identity-resolution helper every story consumes.

- [X] T002 Add `sandboxIdentity() (uid, gid int)` to `internal/runtime/identity.go`: returns `os.Getuid()/os.Getgid()` when both >= 1000, else 1000/1000; pure, no GOOS branch; doc comment states the gate rationale (root/system ids/macOS fallback) per contracts/sandbox-identity.md

**Checkpoint**: Helper compiles; user stories can begin.

---

## Phase 3: User Story 1 - Workspace files keep host ownership (Priority: P1) 🎯 MVP

**Goal**: Sandbox user baked with host uid/gid; workspace files owned uid:gid on host.

**Independent Test**: quickstart.md scenario 1 on this host (uid 1000/gid 1001): `id` in sandbox shows 1000/1001; `stat` on a sandbox-created file shows `1000:1001`.

### Implementation for User Story 1

- [X] T003 [US1] In `embed/Dockerfile.tmpl` replace hardcoded user creation (lines 20-21) with `(getent group {{.Gid}} >/dev/null || groupadd -g {{.Gid}} kekkai) && useradd -m -u {{.Uid}} -g {{.Gid}} -s /bin/bash kekkai`; update the §6.3 comment above it (host-matched identity, includes collision guard per research.md D3)
- [X] T004 [US1] In `embed/Dockerfile.tmpl` change all name-based chowns (`chown -R kekkai:kekkai`, `chown kekkai:kekkai .bashrc`) to numeric `{{.Uid}}:{{.Gid}}` (depends on T003, same file)
- [X] T005 [US1] In `internal/runtime/up.go` `renderDockerfile`: add `Uid, Gid int` to the template data struct and populate from `sandboxIdentity()` (adjust signature or call helper inside; keep §7.3/§6.1 comments accurate)
- [X] T006 [P] [US1] Update `SPECIFICATION.md` §6.3 (line ~186): replace "UID/GID 1000" sentence with host-matched identity + >= 1000 gate + 1000/1000 fallback (constitution I: same commit as code)
- [X] T007 [US1] Validate: quickstart.md scenario 1 (build, `up` via pseudo-TTY, `id` inside, create file, `stat` on host) and scenario 4 (`sudo -l` shows only firewall script; node/npm/claude work)

**Checkpoint**: MVP — the reported defect is fixed on this host.

---

## Phase 4: User Story 2 - Identity change is handled automatically (Priority: P2)

**Goal**: Different host identity → different image, built automatically; offline fallback never crosses identities.

**Independent Test**: quickstart.md scenario 2 — `sudo ./kekkai up` after a normal run builds a second, distinct `kekkai:<hash>` image with no manual cleanup.

### Implementation for User Story 2

- [X] T008 [US2] In `internal/runtime/identity.go` extend `ConfigHash` signature with `uid, gid int`, join both into the hashed input string, update its doc comment ("bake inputs minus the claude version" now includes identity)
- [X] T009 [US2] Update `ConfigHash` call site in `internal/runtime/up.go:169` (`ensureImage`) to pass `sandboxIdentity()` values (resolve once, share with `renderDockerfile`)
- [X] T010 [US2] Validate: quickstart.md scenario 2 — second identity triggers automatic rebuild to a different tag; `docker images 'kekkai:*'` shows both; no manual steps needed

**Checkpoint**: Identity changes are self-healing; ImageTag (free) + ConfigHash (T008) both identity-aware.

---

## Phase 5: User Story 3 - Unusual host identities do not break the sandbox (Priority: P3)

**Goal**: Root/system-range ids fall back safely; gid collisions never fail the build; guarantees hold.

**Independent Test**: root-invoked sandbox reports 1000/1000 and passes firewall verification; a build with a pre-existing gid succeeds.

### Implementation for User Story 3

> Fallback logic (T002) and collision guard (T003) already landed; this phase proves them.

- [X] T011 [P] [US3] Validate root fallback: inside the scenario-2 root-built sandbox run `id` → `uid=1000 gid=1000`, firewall verification passes, `sudo -l` lists only the firewall script (contracts C2/C6); clean up root-built image afterwards
- [X] T012 [P] [US3] Validate collision guard: scratch `docker build` of a rendered Dockerfile whose `{{.Gid}}` pre-exists in the base (e.g. render with a fixture uid/gid where the group was pre-created in a derived test image, or temporarily render Gid matching an existing group) → build succeeds, `id -g` in the image equals that gid (contracts C5); document result in specs/018-host-uid-gid/quickstart.md notes

**Checkpoint**: All contracts C1-C6 exercised.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T013 Final `go vet ./... && go build ./cmd/kekkai`; re-read changed comments for accuracy (`embed/Dockerfile.tmpl`, `internal/runtime/identity.go`, `internal/runtime/up.go`)
- [X] T014 Full quickstart.md pass end-to-end; remove test project dir and stale `kekkai:*` test images (`kekkai down`, `docker rmi` of root-built tag)

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1 → Phase 2 → user stories
- US1 (Phase 3): needs T002 only
- US2 (Phase 4): needs T002; independent of US1 code-wise (different functions), but validation (T010) is most meaningful after US1 landed
- US3 (Phase 5): validation-only; needs T003 (collision guard) and T002 (fallback), i.e. after US1
- Phase 6 after all stories

### Within Stories

- T003 → T004 (same file), then T005; T006 parallel to all US1 code tasks; T007 last
- T008 → T009 → T010
- T011 ∥ T012

### Parallel Opportunities

- T006 (SPECIFICATION.md) parallel with T003-T005
- T008 (identity.go) parallel with T003/T004 (template) if US1/US2 worked concurrently — different files
- T011 ∥ T012 (independent validations)

## Parallel Example: User Story 1

```text
Track A: T003 → T004 → T005 (template then wiring)
Track B: T006 (SPECIFICATION.md §6.3)
Join:    T007 (e2e validation)
```

## Implementation Strategy

**MVP first**: T001 → T002 → T003-T007 (US1). Stop, validate on this host (the 1000/1001 case that triggered the feature). US2 (T008-T010) closes the offline-fallback hole; US3 (T011-T012) proves edge cases. Single-commit delivery is fine given size — constitution I requires SPECIFICATION.md (T006) in the same commit as the code regardless.
