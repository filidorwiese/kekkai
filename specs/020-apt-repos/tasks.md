# Tasks: Custom Apt Repositories (`image.apt_repos`)

**Input**: Design documents from `/specs/020-apt-repos/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation against a real docker daemon; each story closes with its quickstart scenario(s).

**Organization**: Tasks grouped by user story; stories are independently implementable and testable after the Foundational phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1–US4 from spec.md

## Phase 1: Setup

No setup tasks — existing single-binary Go codebase; the feature threads through the established config → validate → render → hash pipeline (plan.md Structure Decision).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The shared entity both US1 (render) and US2 (validation) build on.

- [X] T001 Add `AptRepo` struct (`Name`, `URL`, `Suite`, `Components`, `KeyURL`, yaml tags per data-model.md) and `AptRepos []AptRepo` field on `ImageConfig` in internal/config/config.go — strict parsing of the new keys must work; no validation yet

**Checkpoint**: Config parses `image.apt_repos`; all user stories can start.

---

## Phase 3: User Story 1 - Install a package from a third-party apt repository (Priority: P1) 🎯 MVP

**Goal**: Declared repos render into the image build before package installation; packages from them install in the same build. Flat repos and keyless (system-keyring) repos work.

**Independent Test**: quickstart.md S1 (Dart repo + `dart` package usable in sandbox) and S2 (backports without `key_url`).

### Implementation for User Story 1

- [X] T002 [P] [US1] Extend embed/Dockerfile.tmpl with a `{{range .AptRepos}}` block after the GitHub CLI repo setup and before the final `apt-get update`: conditional key fetch + chmod to `/etc/apt/keyrings/kekkai-<name>.gpg`, sources line to `/etc/apt/sources.list.d/kekkai-<name>.list` with `arch=` always and `signed-by=` only when a key is set, exactly per contracts/image-render.md; use `{{- -}}` whitespace control so zero repos render byte-identical to today's template
- [X] T003 [US1] Build per-repo render data in `renderDockerfile` in internal/runtime/up.go (KeyringPath, SourcesPath, Options, SuiteLine — `main` default for empty components on non-flat, no components token for flat suites per data-model.md) and pass `cfg.Image.AptRepos` through `ensureImage` (depends on T001, T002)
- [X] T004 [US1] Validate e2e: `go build ./cmd/kekkai`, confirm zero-repo config renders a byte-identical Dockerfile (same image tag as before), then run quickstart.md S1 (Dart happy path incl. in-sandbox `dart --version` and sources-line check) and S2 (backports, no signed-by)

**Checkpoint**: MVP — third-party packages installable via config alone.

---

## Phase 4: User Story 2 - Invalid repository config is rejected before any build (Priority: P2)

**Goal**: Every `apt_repos` field is allowlist-validated at config load; all violations reported in one pass before docker work; `trusted=yes` and shell/apt syntax inexpressible; builtin name reserved.

**Independent Test**: quickstart.md S3 — multi-violation config rejected in one pass, no build output.

### Implementation for User Story 2

- [X] T005 [US2] Implement the `apt_repos` validation block in `Validate()` in internal/config/validate.go per contracts/config-validation.md: field grammars (name/url/key_url/suite incl. flat form with `..` ban/components), required fields, duplicate names, reserved names `github-cli`/`githubcli`, flat-entry components rejection; error strings prefixed `image.apt_repos[<i>]` (+ `(<name>)` when valid) naming the field (depends on T001)
- [X] T006 [US2] Validate e2e: run quickstart.md S3 (bad name, duplicate, http url, `[trusted=yes]` in suite, flat+components, reserved name — all reported in one pass, exit non-zero, zero docker work); spot-check the S1 config still passes

**Checkpoint**: Injection surface closed; US1 + US2 independently verified.

---

## Phase 5: User Story 3 - Repo changes trigger an image rebuild (Priority: P2)

**Goal**: Any `apt_repos` change alters image identity (rebuild); unchanged config reuses the image; `[]` ≡ absent; offline-fallback label covers repos.

**Independent Test**: quickstart.md S4 — edit → rebuild, revert → reuse, zero-repo tag unchanged vs pre-feature.

### Implementation for User Story 3

- [X] T007 [US3] Extend `ConfigHash` in internal/runtime/identity.go with the deterministic repo serialization (config-order `name|url|suite|components|key_url` lines, raw values, empty list ⇒ empty string per contracts/image-render.md) and update its call site in `ensureImage` in internal/runtime/up.go (depends on T001; `ImageTag` needs no change — rendered Dockerfile already covers repos via T002/T003)
- [X] T008 [US3] Validate e2e: run quickstart.md S4 (second `up` reuses tag; suite edit rebuilds; revert reuses original; config without `apt_repos` keeps its pre-feature tag)

**Checkpoint**: Identity semantics proven; no rebuild churn for existing users.

---

## Phase 6: User Story 4 - Helpful failure when a signing key is missing or wrong (Priority: P3)

**Goal**: apt signature failures during build produce a hint naming the offending entry and pointing at `key_url`.

**Independent Test**: quickstart.md S5 — Dart repo without `key_url` fails with NO_PUBKEY plus the hint.

### Implementation for User Story 4

- [X] T009 [P] [US4] Change `BuildImage` in internal/docker/cli.go to tee stdout/stderr through `io.MultiWriter` (terminal + buffer) and return the captured output alongside the error; success path behavior unchanged
- [X] T010 [US4] In internal/runtime/up.go `buildImage`/`ensureImage`: on build failure with `apt_repos` configured, scan captured output for `NO_PUBKEY` / `is not signed` / `EXPKEYSIG` / `NODATA`, attribute by first entry whose `url` appears in the output, print the exact hint lines from contracts/image-render.md to stderr after the build error (never replacing it); no hint when `apt_repos` is empty (depends on T009)
- [X] T011 [US4] Validate e2e: run quickstart.md S5 (build fails, hint names `image.apt_repos[0] (dart)` suggesting `key_url`, exit non-zero); confirm a non-signature build failure prints no hint

**Checkpoint**: All four stories independently functional.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: FR-011 documentation (spec-first: lands in the same change as the code) and full-sweep validation.

- [X] T012 [P] Add commented `apt_repos` block (Dart example from spec) under `image:` next to `apt_packages` in `starterConfig` in internal/runtime/init.go
- [X] T013 [P] Document `apt_repos` fields (example + field semantics, flat repos, key_url behavior) in the README.md image section
- [X] T014 [P] Amend SPECIFICATION.md: §4.2 schema block, §4.4 validation bullets, §6.1 bake-time inputs + `kekkai.config_hash` formula, §6.3 Dockerfile contract registration lines
- [X] T015 Full validation sweep: `go vet ./...`, `go build ./cmd/kekkai`, run quickstart.md S6 (firewall unaffected — repo hosts unreachable at runtime) and S7 (init/README/SPECIFICATION grep), re-confirm S1–S5 pass on the final binary

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: T001 blocks everything.
- **US1 (Phase 3)**: after T001. **US2 (Phase 4)**: after T001 — independent of US1. **US3 (Phase 5)**: T007 after T001; its e2e (T008) needs US1's render (T002/T003). **US4 (Phase 6)**: T009 independent; T010 after T009; its e2e (T011) needs US1's render.
- **Polish (Phase 7)**: T012–T014 anytime after design is stable; T015 last.

### User Story Dependencies

- US1: none (post-Foundational). US2: none — validation is testable without the render path. US3/US4: implementation independent, e2e verification builds on US1.

### Parallel Opportunities

- T002 (template) ∥ T005 (validate.go) ∥ T009 (docker/cli.go) — disjoint files, all unblocked after T001.
- T012 ∥ T013 ∥ T014 — disjoint doc files.
- up.go tasks (T003, T007's call site, T010) touch the same file — keep sequential.

## Parallel Example: after T001

```bash
Task: "Extend embed/Dockerfile.tmpl with {{range .AptRepos}} block"        # T002
Task: "Implement apt_repos validation block in internal/config/validate.go" # T005
Task: "Tee build output in internal/docker/cli.go BuildImage"               # T009
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. T001 → T002/T003 → T004. **STOP and VALIDATE**: Dart repo works end-to-end. That is the MVP — but do not ship without US2 (T005/T006): the injection-surface validation is the safety half of the same feature.

### Incremental Delivery

1. T001 (foundation) → US1 (MVP build path) → US2 (validation) → US3 (identity) → US4 (hint) → Polish/docs (T012–T015), each story e2e-verified at its checkpoint via its quickstart scenario.
