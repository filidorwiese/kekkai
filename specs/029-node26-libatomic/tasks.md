# Tasks: Node 26 LTS runs in the sandbox image

**Input**: Design documents from `/specs/029-node26-libatomic/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/builtin-packages.md, quickstart.md

**Tests**: No test-framework tasks. Validation is end-to-end against a real docker daemon per constitution IV; each story phase carries its quickstart scenarios. A control build with the pre-change binary proves the repro before the fix is trusted.

**Organization**: Tasks grouped by user story. The whole feature is one list entry plus a spec amendment (Phase 2). US1 proves the default `lts` selector builds, US2 proves explicit 26 selectors build, US3 proves 22/24 still build and rebuild exactly once. SPECIFICATION.md is amended alongside the code (Principle I).

## Codebase impact summary

| Area | Change | Risk |
|---|---|---|
| `internal/runtime/up.go` `builtinAptPackages` | append `"libatomic1"` in a new "node runtime" group with a why-comment (research R4) | low; one literal |
| `SPECIFICATION.md` §5.1 | new "Node runtime" sentence naming `libatomic1`; extend the "baked regardless of path" sentence | docs |
| Behavior change for users | one image rebuild per configuration on next `kekkai up`; `lts`/26 builds succeed | intended (FR-005) |
| Unchanged | `embed/Dockerfile.tmpl`, `identity.go` hash functions, `init.go` starter, README, config schema, firewall, §5.4 | — |

Net: +1 string and +2 comment lines Go, ~3 lines spec, one 45 KB Debian package in the image, no new config key, flag or egress.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: control binary, scratch project, clean image state

- [X] T001 Build the pre-change control binary from current `main`: `go build -ldflags "-X main.version=v0.0.0-ctl" -o /tmp/kekkai-ctl ./cmd/kekkai`; create scratch project `~/kekkai-qs-029` (under `$HOME`, not `/tmp`: snap daemon has a private `/tmp`) with no `.kekkai.yaml` and `git init`
- [X] T002 [P] Record baseline into `/tmp/kekkai-qs-029-baseline.txt`: `docker images --filter 'reference=kekkai:*' --format '{{.Tag}} {{.ID}} {{.Labels}}'` (tags and `kekkai.config_hash` labels before the change, quickstart 5), then `kekkai prune --yes` so every build in the stories is a cold build
- [X] T003 [P] Control run (quickstart 1): in `~/kekkai-qs-029` run `/tmp/kekkai-ctl up --verbose`; confirm the nvm layer prints `WARNING: Version 'none' does not exist` and the claude layer fails with `npm: command not found`; append both lines to `/tmp/kekkai-qs-029-baseline.txt` — RESULT: not reproduced here: buildkit cached the `nvm install --lts` layer from a pre-Node-26 build (image fc492b5c52d1 carries v24.21.0), so the control succeeded; the user log and the bare-container repro in research.md R1 stand as control evidence. Builder cache deliberately not pruned (host-wide state)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the one code change and its spec amendment; everything the stories validate

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 In internal/runtime/up.go `builtinAptPackages`: after the `"python3", "python3-venv", "python3-pip",` line append a new group with comment `// node runtime (specs/029): Node >= 26 links libatomic.so.1, absent from` / `// the Debian base; without it the nvm-installed node cannot start.` followed by `"libatomic1",`
- [X] T005 [P] Amend SPECIFICATION.md §5.1: after the "Scripting runtime: `python3`, …" sentence insert the "Node runtime: `libatomic1` — …" sentence verbatim from specs/029-node26-libatomic/contracts/builtin-packages.md; in the next paragraph extend the list to `jq`/`aggregate` … `openssh-client` only on the `ssh_agent` path, `libatomic1` only by Node ≥ 26, but all stay baked
- [X] T006 `go build ./... && go vet ./...` clean; `task build` (→ `/tmp/kekkai`); `grep -n libatomic1 internal/runtime/up.go SPECIFICATION.md` shows the list entry and both §5.1 sentences

**Checkpoint**: binary carries the new package set; stories can begin

---

## Phase 3: User Story 1 - Default configuration builds again (Priority: P1) 🎯 MVP

**Goal**: `kekkai up` with no config file builds an image on Node 26.x and starts Claude.

**Independent Test**: quickstart 2 — cold build with no `.kekkai.yaml` succeeds; startup line reports node v26.x.y; `node -v`, `npm -v`, `claude --version` work in-sandbox.

- [ ] T007 [US1] In `~/kekkai-qs-029` (no `.kekkai.yaml`) run `script -qec "/tmp/kekkai up --verbose" /tmp/kekkai-qs-029-us1.log </dev/null &`; wait for the build; confirm in the log: the nvm layer shows `default -> v26.` and NOT `none`; the claude layer runs `claude --version` successfully; the startup line reads `kekkai sandbox: node v26.x.y, claude <ver>`; §9.6 firewall probes pass
- [ ] T008 [US1] With the T007 sandbox running: `/tmp/kekkai exec node -v` → `v26.x.y`; `/tmp/kekkai exec npm -v` → `11.x`; `/tmp/kekkai exec claude --version` → version string; `/tmp/kekkai exec readlink /home/kekkai/.nvm/current` → a `versions/node/v26.x.y` path (not `none`); then `/tmp/kekkai down`

**Checkpoint**: default path works end to end; MVP complete

---

## Phase 4: User Story 2 - Explicit Node 26 selectors build (Priority: P1)

**Goal**: `image.node_version` pinned to `26`, `26.13` or `26.13.0` builds and runs.

**Independent Test**: quickstart 3 — both pins build; startup lines report 26.x and exactly v26.13.0.

- [ ] T009 [US2] `printf 'image:\n  node_version: "26"\n' > ~/kekkai-qs-029/.kekkai.yaml`; `script -qec "/tmp/kekkai up" /tmp/kekkai-qs-029-us2a.log </dev/null &`; confirm a build (new tag), startup line `node v26.x.y`, `/tmp/kekkai exec node -v` → 26.x; `/tmp/kekkai down`
- [ ] T010 [US2] `printf 'image:\n  node_version: "26.13.0"\n' > ~/kekkai-qs-029/.kekkai.yaml`; same procedure into `/tmp/kekkai-qs-029-us2b.log`; startup line reads exactly `node v26.13.0`; `/tmp/kekkai down`

**Checkpoint**: every 26 selector form builds

---

## Phase 5: User Story 3 - Earlier Node selectors keep working (Priority: P2)

**Goal**: 22 and 24 still build; each configuration rebuilds exactly once, then caches.

**Independent Test**: quickstart 4 and 5 — `24` builds once and is cached on the second `up`; `22` builds; new tags and `config_hash` labels differ from the T002 baseline.

- [ ] T011 [US3] `printf 'image:\n  node_version: "24"\n' > ~/kekkai-qs-029/.kekkai.yaml`; `script -qec "/tmp/kekkai up" /tmp/kekkai-qs-029-us3a.log </dev/null &`; confirm `building image` line, startup `node v24.x.y`; `/tmp/kekkai down`; run `up` again into `/tmp/kekkai-qs-029-us3b.log` and confirm NO `building image` line; `/tmp/kekkai down`
- [ ] T012 [P] [US3] `printf 'image:\n  node_version: "22"\n' > ~/kekkai-qs-029/.kekkai.yaml`; `up` into `/tmp/kekkai-qs-029-us3c.log`; confirm build and startup `node v22.x.y`; `/tmp/kekkai down`
- [ ] T013 [US3] Hash invalidation (quickstart 5): `docker images --filter 'reference=kekkai:*' --format '{{.Tag}} {{.Labels}}'`; every tag and `kekkai.config_hash` differs from the T002 baseline; the baseline shows no tag in common with the new set

**Checkpoint**: no regression on earlier lines; one rebuild per configuration

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: remaining spec edge cases, egress check, cleanup

- [ ] T014 [P] Duplicate user entry (quickstart 7): `printf 'image:\n  apt_packages: [libatomic1]\n' > ~/kekkai-qs-029/.kekkai.yaml`; `up` builds and starts; `/tmp/kekkai down`
- [ ] T015 [P] No new egress (quickstart 6, FR-006): with a sandbox up from any story, run `/tmp/kekkai traffic` for ~1 minute of normal use; only `api.anthropic.com` and DNS appear as ALLOW; no new destinations; `/tmp/kekkai down`
- [ ] T016 Offline fallback edge case (optional, quickstart 5 second half): with only a pre-029 image present (restore one via `docker pull`/tag is not possible; instead temporarily set `KEKKAI_REPO`-independent block of registry.npmjs.org via `/etc/hosts`) run `/tmp/kekkai up` and confirm `no existing kekkai image matches this config`; revert `/etc/hosts`. Skip if no pre-029 image survived T002; the mechanism is covered by T013
- [ ] T017 Cleanup: `rm -rf ~/kekkai-qs-029 /tmp/kekkai-ctl /tmp/kekkai-qs-029-*.log /tmp/kekkai-qs-029-baseline.txt`; `kekkai prune --yes`; `git status` shows only `internal/runtime/up.go`, `SPECIFICATION.md` and `specs/029-node26-libatomic/` changed; update `specs/029-node26-libatomic/spec.md` Status to Implemented

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: T001 first; T002 and T003 parallel after it (T003 needs the control binary and a pruned state, so T002's prune precedes T003's build)
- **Foundational (Phase 2)**: T004 and T005 parallel; T006 after both. Blocks all stories
- **US1 (Phase 3)**: after T006. T007 → T008
- **US2 (Phase 4)**: after T006, independent of US1. T009 → T010 (same scratch dir, sequential)
- **US3 (Phase 5)**: after T006, independent of US1/US2. T011 and T012 share the scratch dir so run sequentially in practice; T013 after both
- **Polish (Phase 6)**: after all stories; T014/T015 parallel; T016 optional; T017 last

### User Story Dependencies

- US1, US2, US3 are independent; they share one scratch directory, so run them sequentially to keep the `.kekkai.yaml` state unambiguous
- Each story's checkpoint is a standalone pass/fail

### Parallel Opportunities

- T004 ‖ T005 (Go file vs spec file)
- T014 ‖ T015 (two sandboxes need two scratch dirs; otherwise sequential)
- Story phases could run in parallel with separate scratch dirs (`~/kekkai-qs-029-{a,b,c}`), at the cost of parallel image builds

## Implementation Strategy

### MVP First (US1 only)

1. Phase 1 → Phase 2 → Phase 3
2. STOP and validate: default `kekkai up` builds on Node 26 and Claude starts
3. Ship if urgent; US2/US3 are confidence checks on the same one-line change

### Incremental Delivery

1. Phase 2 is the entire code change; every later phase is validation
2. US2 adds explicit-selector coverage, US3 adds regression and hash coverage
3. Polish covers edge cases and egress

### Notes

- All `kekkai up` runs go through `script -qec … </dev/null &` because `up` is interactive and needs a pty
- Cold builds take several minutes each (apt + nvm + npm); `--verbose` on the first one to read the nvm layer
- Commit only when asked; expected diff is two files plus the feature dir
