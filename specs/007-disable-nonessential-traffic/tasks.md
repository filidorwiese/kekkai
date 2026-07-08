# Tasks: Disable Nonessential Traffic

**Input**: Design documents from `/specs/007-disable-nonessential-traffic/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/nonessential-traffic-cli.md, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation;
each story phase runs its quickstart scenarios against the real binary.

**Organization**: US1 (env var) and US2 (firewall) are independent code
changes but both amend SPECIFICATION.md, so they run sequentially; US3 is
docs/template cleanup.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Verify baseline: `go build ./... && go vet ./...` pass on clean tree

---

## Phase 2: User Story 1 - Sandbox sends no nonessential traffic (Priority: P1) 🎯 MVP

**Goal**: Every sandbox starts with `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`; user `env` overrides

**Independent Test**: quickstart.md scenarios 1-2

- [X] T002 [US1] In internal/runtime/up.go `buildRunArgs`: add `addEnv("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC", "1")` to the builtin env block (before user env — docker last-value-wins gives FR-002 override, research.md R2); amend SPECIFICATION.md §5.3 builtin env list with the new var + one-line purpose (constitution I, same change as code); `go build ./... && go vet ./...` pass
- [X] T003 [US1] Run quickstart.md scenarios 1-2: default sandbox has `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` in `docker exec ... env`; with `env: CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: "0"` in .kekkai.yaml the container shows `0` (user wins); Claude starts normally both times

**Checkpoint**: US1 delivers the MVP

---

## Phase 3: User Story 2 - Telemetry endpoint leaves the builtin firewall allowance (Priority: P1)

**Goal**: Builtin allowlist = api.anthropic.com only (external); statsig blocked like any other host

**Independent Test**: quickstart.md scenarios 3-4

- [X] T004 [US2] In embed/init-firewall.sh: delete `add_domain statsig.anthropic.com warn` (line ~101) and drop statsig from the §5.4 comment above it (research.md R3); amend SPECIFICATION.md §5.4 (destinations: api.anthropic.com + host.docker.internal warn tier, statsig gone) and §9.4 step 4 (remove the statsig warn+skip clause) — constitution II requires this spec amendment; `go build ./...` passes (embed changes recompile)
- [X] T005 [US2] Run quickstart.md scenarios 3-4: `up` startup shows no `allowed: statsig.anthropic.com` line, both §9.6 probes pass; inside the sandbox `curl https://statsig.anthropic.com` is blocked while `https://api.anthropic.com` responds; pre-change image is not reused — `up` rebuilds (hash change, research.md R4) then subsequent `up` reuses the new image

**Checkpoint**: US1 + US2 validated — sandbox is quiet and the hole is closed

---

## Phase 4: User Story 3 - No stale telemetry references in docs or starter config (Priority: P2)

**Goal**: Zero statsig mentions in README, SPECIFICATION.md, and `kekkai init` output

**Independent Test**: quickstart.md scenario 5

- [X] T006 [P] [US3] Rewrite the always-allowed comment in internal/runtime/init.go `starterConfig` network section (line ~61: api.anthropic.com only) and the matching README.md network-example comment (line ~121); verify the README known-limitations bullet (already user-reworded) matches final behavior, no statsig (research.md R5)
- [X] T007 [US3] Run quickstart.md scenario 5: `grep -ri statsig README.md SPECIFICATION.md` exits 1 (needs T004's spec edits too); fresh `kekkai init` output greps 0 for statsig and names api.anthropic.com as the only always-allowed destination

**Checkpoint**: All stories validated

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T008 Run quickstart scenario 6 (regression): a project with `allow_github` + `allowed_domains` starts with unchanged lines and passing github probe; mounts/limits/exit codes untouched; final `go build ./... && go vet ./...`

---

## Dependencies & Execution Order

- Phase 1 → everything
- T002 → T004 (both edit SPECIFICATION.md — keep sequential, no merge conflicts)
- T006 [P]: independent files (init.go, README.md), can run alongside T002/T004
- T007 needs T004 + T006 (grep spans SPECIFICATION.md and starter output)
- T005 needs T004; T003 needs T002; T008 last

## Implementation Strategy

MVP = Phases 1-2 (env var alone already silences telemetry). US2 closes the
now-unused firewall hole; US3 cleans the docs. Total new code ~5 lines across
internal/runtime/up.go and embed/init-firewall.sh, plus two documentation
files and the starter template comment.
