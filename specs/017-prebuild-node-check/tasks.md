# Tasks: Pre-Build Node Version Existence Check

**Input**: Design documents from `/specs/017-prebuild-node-check/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/node-existence-check.md, quickstart.md

**Tests**: Per constitution IV, validation is end-to-end: real `kekkai up` runs against the live nodejs.org index and real docker builds. Each story phase ends with its quickstart scenario(s).

**Organization**: US1 (probe + abort) and US3 (wrapper removal) are separate code changes; US2 is behavioral validation of US1's probe. US1 and US3 both touch `internal/runtime/up.go`/`embed/Dockerfile.tmpl` territory — do code first, then batch the e2e passes on one image generation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 (P1 fail fast pre-build), US2 (P2 never blocks), US3 (P3 wrapper gone)

## Path Conventions

Single Go binary project: `cmd/`, `internal/`, `embed/` at repository root (see plan.md).

---

## Phase 1: Setup

**Purpose**: Confirm clean baseline and live index reachability (test dependency).

- [X] T001 Verify baseline: `go build ./...` succeeds; docker daemon responds; `curl -s -o /dev/null -w '%{http_code}' https://nodejs.org/dist/index.json` → 200

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: None — US1 and US3 are independent code changes; US2 validates US1. Proceed directly.

**Checkpoint**: n/a

---

## Phase 3: User Story 1 - A typo'd Node version fails in seconds, not minutes (Priority: P1) 🎯 MVP

**Goal**: Numeric selectors checked against nodejs.org/dist/index.json before a build starts; confirmed-absent aborts with the contract error; `lts` never checked.

**Independent Test**: quickstart.md Scenario 1 — `node_version: "99"` aborts in seconds, zero docker build activity, error names `image.node_version`.

- [X] T002 [US1] In `internal/runtime/up.go`: add `nodeVersionMissing(selector string) bool` — GET `https://nodejs.org/dist/index.json` (`http.Client{Timeout: 10s}`, no retries), decode `[]struct{ Version string }`, match per contracts/node-existence-check.md (full pin → exact `"v"+sel`; major/major.minor → prefix `"v"+sel+"."`); return true ONLY on 200+parsed+no-match, false on every error path (doc comment: tri-state, mirrors the retired 014 Docker Hub probe semantics — inconclusive must never block, research.md R3). Call it in `ensureImage` inside the `!docker.ImageExists(tag)` branch before the "building image" print, guarded by `cfg.Image.NodeVersion != config.DefaultNodeVersion`; on true return error `image.node_version: %q matches no published Node version — see https://nodejs.org/dist/ for available versions` (research.md R4)
- [X] T003 [US1] E2E quickstart.md Scenario 1 + Scenario 2 step 1: build `kekkai-test`; for `"99"`, `"22.99"`, `"22.99.0"` → `up` exits non-zero in ~seconds with contract text, `docker images` count unchanged, no build output; for `"22"` → "building image" appears (abort the build right after it starts to save time)

**Checkpoint**: MVP — bad versions die pre-build with a config-shaped error.

---

## Phase 4: User Story 2 - The check never blocks a build it can't judge (Priority: P2)

**Goal**: Inconclusive lookups proceed silently; cached-image runs and `lts` make zero index requests.

**Independent Test**: quickstart.md Scenario 3 — dead-proxy env + valid version → build starts with no check-related output.

- [X] T004 [US2] E2E quickstart.md Scenarios 3 + 5 + Scenario 2 step 2 (pin `claude.version` in the test config to isolate the probe, research.md R6): `https_proxy=http://127.0.0.1:9` + `node_version: "22"` → build starts, no check output, no added delay; same env + `node_version: lts` → build starts (lts provably check-free); after one completed build, cached `up` under the dead proxy → sandbox path reached with zero network attempts

**Checkpoint**: Tri-state safety proven on the real binary.

---

## Phase 5: User Story 3 - The confusing in-build error wrapper is gone (Priority: P3)

**Goal**: nvm install step carries no kekkai error echo; native nvm error is the single in-build failure message.

**Independent Test**: quickstart.md Scenario 4 — grep finds no wrapper; forced fallback build failure shows nvm's message once.

- [X] T005 [US3] In `embed/Dockerfile.tmpl`: remove `|| { echo "ERROR: Node version {{.NodeVersionRaw}} not found - check image.node_version in .kekkai.yaml" >&2; exit 1; }` from the nvm install RUN (keep the `&& nvm alias ... && ln -s ...` chain); adjust the step comment to note the host-side pre-check (specs/017) and nvm's native error as the inconclusive-path fallback. In `internal/runtime/up.go`: remove the now-unused `NodeVersionRaw` field from the `renderDockerfile` template data struct (data-model.md §3)
- [X] T006 [US3] E2E quickstart.md Scenario 4: `grep -c 'ERROR: Node version' embed/Dockerfile.tmpl` → 0; `https_proxy=http://127.0.0.1:9` + `node_version: "99"` (probe inconclusive → build runs) → build fails at the nvm step with nvm's native `Version '99' not found` appearing and NO `check image.node_version` echo

**Checkpoint**: All three stories done.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Spec-first compliance, final verification.

- [X] T007 [P] In `SPECIFICATION.md` §6.1: replace the "A well-formed but nonexistent version fails the build with an error naming `image.node_version`" sentence with the pre-check paragraph: before a build whose `image.node_version` is numeric, a best-effort existence check against nvm's source of truth (nodejs.org dist index, 10s, no retries) — confirmed absence aborts naming the key; any inconclusive outcome proceeds (never blocks degraded/offline use); `lts` and cached-image runs make no request; the in-build fallback is nvm's native error (no kekkai wrapper). Constitution I: same commit as code
- [X] T008 Final: `go build ./... && go vet ./...`; complete the quickstart sign-off checklist; cleanup test dir images/volumes/binaries; note in commit message: rendered Dockerfile changed (wrapper removal) → one-time rebuild on next `up`

---

## Dependencies

```text
Phase 1 (T001)
  ├─► Phase 3 US1 (T002 → T003)   🎯 MVP — up.go probe
  └─► Phase 5 US3 code (T005)      — Dockerfile.tmpl + up.go field (after T002: same file)
T002+T005 ─► batched e2e: T003 → T004 → T006 (one image generation, shared daemon)
Phases 3–5 ─► Phase 6 (T007 ∥ e2e, T008 last)
```

- T005 after T002 (both edit `internal/runtime/up.go`).
- Run ALL code edits (T002, T005) before the e2e batch so the rendered-Dockerfile hash changes once and Scenario 2's completed build serves Scenario 5's cache test.
- E2E tasks share the docker daemon — sequential.

## Parallel Execution Examples

- T007 (SPECIFICATION.md) ∥ the e2e batch — disjoint files.
- Within the e2e batch, Scenario 1's three bad selectors run back-to-back in one shell session (no builds, seconds each).

## Implementation Strategy

1. **Code first, validate once**: T002 + T005 land together, then T003/T004/T006 run against one binary and one image generation.
2. **MVP**: T002–T003 alone delivers the P1 value.
3. **Time saver**: Scenario 2's `lts` build is the only full build needed (serves the cache test); the `"22"` build can be interrupted once "building image" proves the check passed.
4. **Spec-first gate**: T007 commits with the code (constitution I).
