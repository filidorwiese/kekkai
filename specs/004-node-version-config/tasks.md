# Tasks: Node Version Config

**Input**: Design documents from `/specs/004-node-version-config/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/config.md, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation against a
real docker daemon; each story phase ends with its quickstart scenarios.

**Organization**: Config-layer rename is foundational (all stories and compilation
depend on it); stories then validate independently.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Verify baseline: `go build ./... && go vet ./...` pass on clean tree

---

## Phase 2: Foundational (Blocking Prerequisites)

**⚠️ CRITICAL**: The field rename breaks compilation repo-wide; T002–T004 must land together before any story validation.

- [X] T002 In internal/config/config.go: replace `ImageConfig.BaseImage` with `NodeVersion string yaml:"node_version"`; replace `DefaultBaseImage` const with `DefaultNodeVersion = "lts"` and `debianRelease = "trixie"`; add `ResolvedBaseImage()` method returning `"node:" + NodeVersion + "-" + debianRelease`; default NodeVersion in `applyDefaults()`; add `imageKeysSet` populated via `presentSectionKeys(data, "image")`; legacyKeys: add `"image.base_image": "image.node_version"` and re-point `"image.base"` to `image.node_version` (data-model.md)
- [X] T003 In internal/config/validate.go: replace base_image rule with node_version rule — explicit-empty (key present, value empty/blank) is an error suggesting omission for default; non-empty must match `^[a-z0-9.]+$` else error naming the offending value (contracts/config.md table)
- [X] T004 In internal/runtime/up.go: use `cfg.Image.ResolvedBaseImage()` at the `ConfigHash` and `renderDockerfile` call sites; `go build ./... && go vet ./...` pass

**Checkpoint**: Binary compiles; config layer complete

---

## Phase 3: User Story 1 - Configure only the Node version (Priority: P1) 🎯 MVP

**Goal**: Single `image.node_version` value (default `lts`) selects the sandbox Node version

**Independent Test**: quickstart.md scenarios 1–2 (default lts, pinned version, rebuild semantics)

- [X] T005 [US1] In internal/runtime/init.go starterConfig: replace `base_image: node:24-trixie` line and its comment with `node_version: lts` + README-grade comment (version selector, resolved internally to pinned Debian node image, must exist as published tag)
- [X] T006 [US1] Run quickstart.md scenarios 1–2 end-to-end (real docker): init → default lts sandbox, `node --version` LTS; pin `22` → v22; unchanged re-up → no rebuild; change to `24` → new hash. Record results in quickstart notes or report

**Checkpoint**: US1 fully functional — MVP

---

## Phase 4: User Story 2 - Clear migration from the old key (Priority: P2)

**Goal**: Old-key configs fail with guided migration error; generated config documents only the new key

**Independent Test**: quickstart.md scenario 3 (legacy-key error, one-pass report, no docker activity)

- [X] T007 [US2] Run quickstart.md scenario 3: config with `base_image:` → exit 1, message `"image.base_image" is now image.node_version`, no docker calls; also verify combined report (legacy key + another violation in one pass)

**Checkpoint**: US1 + US2 independently validated

---

## Phase 5: User Story 3 - Helpful failure for a non-existent version (Priority: P3)

**Goal**: Malformed selectors die in validation; well-formed-but-unpublished versions fail fast pre-pull with a clean error; unreachable registry never blocks

**Independent Test**: quickstart.md scenarios 4–6

- [X] T008 [US3] In internal/runtime/up.go: add `baseImageExists(ref)` pre-check — anonymous token from auth.docker.io scope `repository:library/node:pull`, then manifest HEAD on registry-1.docker.io with docker+OCI Accept headers, 10s timeout (research.md R1); returns exists/missing/inconclusive; wire into `ensureImage` build path only when final tag AND base image miss `docker.ImageExists`; confirmed-missing → error per research.md R7; inconclusive → proceed silently
- [X] T009 [US3] Run quickstart.md scenarios 4–6 (6 skipped: needs host /etc/hosts change; inconclusive path is code-guaranteed — any non-404 proceeds): malformed selector + explicit-empty rejected pre-docker; `99.99` fails fast pre-build naming version; offline-with-local-base still starts

**Checkpoint**: All user stories independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T010 [P] README.md: config example `node_version: lts` (copy/paste safety: active value = default) + any prose mentioning base_image (FR-007)
- [X] T011 [P] SPECIFICATION.md: §4.2 schema, §4.4 validation rule, §4.5 init starter description, §6.1 bake inputs/config_hash wording (resolved base image), new pre-check behavior sentence in §6 (constitution I: same commit as code)
- [X] T012 [P] docs/my-project/.kekkai.yaml: `node_version` replaces `base_image`
- [X] T013 Run quickstart.md scenario 7 sweep: `grep -rn base_image README.md SPECIFICATION.md docs/ internal/runtime/init.go` → only legacyKeys entry in config.go remains; final `go build ./... && go vet ./...`

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 (T002→T003→T004 sequential, same-package coupling) → stories
- US1 (T005–T006) blocks nothing; US2 (T007) needs only Phase 2; US3 (T008–T009) needs only Phase 2
- Stories independent after Phase 2; priority order P1→P2→P3 when sequential
- Polish (T010–T012 parallel, different files) after stories; T013 last

## Implementation Strategy

MVP = Phase 1–3 (config rename + starter + validated default/pinned flow). US2 is
one validation run (mechanism lands in T002). US3 adds the only new code surface
(registry pre-check). Docs sweep last, committed together with code per constitution I.
