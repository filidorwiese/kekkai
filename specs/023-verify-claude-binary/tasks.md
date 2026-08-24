# Tasks: Verify Claude Binary at Image Build Time

**Input**: Design documents from `/specs/023-verify-claude-binary/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/claude-install-verify.md, quickstart.md

**Tests**: No test-framework tasks — validation is end-to-end against a real docker daemon per constitution IV; each story phase carries its quickstart scenarios.

**Organization**: Tasks grouped by user story. US1 (fail loud) and US2 (retry) both land in the same RUN step of `embed/Dockerfile.tmpl` but are separable increments: US1 is install→verify→fail, US2 inserts the single retry between verify and fail. SPECIFICATION.md is amended alongside each increment (Principle I: same commit as the code it describes).

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Working e2e harness for real-daemon validation

- [X] T001 Build test binary (`go build -o kekkai-test ./cmd/kekkai`) and prepare scratch test project `/tmp/kekkai-qs-023` with a minimal `.kekkai.yaml` (pinned `claude.version` to keep builds deterministic); confirm docker daemon reachable

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Verify the load-bearing assumption every story's UX rests on (research.md R3)

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T002 Confirm BuildKit prints a failed RUN step's output in compact TTY progress AND plain progress: build a throwaway Dockerfile whose RUN echoes a marker to stderr and exits 1 (interactive terminal via `script -qec`, then `--progress=plain`); marker must be visible in both — record result in specs/023-verify-claude-binary/research.md R3 if it deviates (deviation would force a host-side surfacing design instead)

**Checkpoint**: Failure-surfacing mechanism proven — story implementation can begin

---

## Phase 3: User Story 1 - Broken install aborts the build (Priority: P1) 🎯 MVP

**Goal**: A build whose claude install lacks the native binary fails loudly with an actionable message and tags no image; a healthy build is unchanged.

**Independent Test**: quickstart.md S1 (happy path) and S2 (forced failure via `--ignore-scripts`, ignoring the retry-specific expectations): build fails, message contains cause + `rerun 'kekkai up'`, `docker image inspect` misses; healthy build succeeds with ~0.1s overhead.

### Implementation for User Story 1

- [X] T003 [US1] In embed/Dockerfile.tmpl replace `RUN npm install -g @anthropic-ai/claude-code@{{.ClaudeVersion}}` with install → `claude --version` verify → fail-loud group per contracts/claude-install-verify.md (WITHOUT the retry clause yet): on verify failure echo the stderr line containing `claude native binary failed to install` and `rerun 'kekkai up'`, then `exit 1`; keep `{{.ClaudeVersion}}` the only template variable and preserve the surrounding comment style
- [X] T004 [P] [US1] Amend SPECIFICATION.md §6.3 claude-install bullet: install is verified with `claude --version` in the same RUN step (npm drops the platform-native optionalDependency silently on transient registry failure); verification failure aborts the build with an actionable stderr line; reference specs/023-verify-claude-binary/contracts/claude-install-verify.md — commit together with T003
- [X] T005 [US1] e2e quickstart.md S1: fresh build in /tmp/kekkai-qs-023 succeeds; `docker run --rm kekkai:<tag> claude --version` exits 0; `--verbose` build log shows the verify line; overhead vs pre-feature build ≤ a few seconds
- [X] T006 [US1] e2e quickstart.md S2 (fail-loud half): copy rendered Dockerfile, append `--ignore-scripts` to the install command (`--omit=optional` proved ineffective: npm 11.17 installs platform optional deps regardless), `docker build` → nonzero exit, message with cause + remedy visible in compact progress AND plain progress, `docker image inspect` on the attempted tag fails (no image)

**Checkpoint**: Silent-broken-image bug eliminated — MVP shippable

---

## Phase 4: User Story 2 - Transient failure self-heals via retry (Priority: P2)

**Goal**: One identical re-install attempt between first verification failure and build failure, so most transient registry/CDN failures never surface to the user.

**Independent Test**: quickstart.md S2 (retry expectations: retry notice line + second npm run before the failure message) and S3 (identical re-install heals a broken install).

### Implementation for User Story 2

- [X] T007 [US2] In embed/Dockerfile.tmpl insert the single retry clause into the T003 step per contracts/claude-install-verify.md: on first verify failure echo the `kekkai:`-prefixed retry notice to stderr, re-run the byte-identical `npm install -g @anthropic-ai/claude-code@{{.ClaudeVersion}}`, verify again; only then fall through to the fail-loud group (final shape: `((install && verify) || (notice; retry && verify)) || (message; exit 1)`)
- [X] T008 [P] [US2] Extend the SPECIFICATION.md §6.3 sentence from T004 with the retry: one identical re-install retry before the build is failed — commit together with T007
- [X] T009 [US2] e2e quickstart.md S2 (full) + S3: double-`--ignore-scripts` build shows retry notice and a second npm run before failing; and inside a broken-install container an identical `npm install -g … && claude --version` heals to exit 0 (S3 retry soundness)

**Checkpoint**: Transient failures self-heal in-build; double failure still fails per US1

---

## Phase 5: User Story 3 - Existing users rebuild cleanly, offline path unchanged (Priority: P3)

**Goal**: Guardrail validation — the template change forces exactly one rebuild via the normal §6.1 hash path, and the §6.2 offline fallback is byte-identical in behavior. No code: both properties hold by construction (research.md R5); this phase proves them.

**Independent Test**: quickstart.md S4 and S5 pass unmodified.

### Implementation for User Story 3

- [X] T010 [P] [US3] e2e quickstart.md S4: with a pre-feature image present for the test config, post-feature `kekkai-test up` builds `kekkai:<newhash>` exactly once, second `up` reuses it; `kekkai.config_hash` labels of old and new image are identical
- [X] T011 [P] [US3] e2e quickstart.md S5: with `claude.version: latest` and a matching existing image, block registry.npmjs.org for the host process and run `kekkai-test up` → existing `npm registry unreachable … reusing existing image` warning and image reuse, no new verification-related output or failure

**Checkpoint**: All stories validated independently

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T012 Full quickstart.md sweep (S1–S5) on the final state; then clean up test artifacts: test container, `kekkai-history-<hash-of-/tmp/kekkai-qs-023>` volume ONLY, test-built images, `/tmp/kekkai-qs-023`
- [X] T013 [P] Confirm README.md (user-facing digest) needs no change — it should not describe install-step internals; update only if it does

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (P1)** → **Foundational (P2)** → user stories
- **US1 (Phase 3)**: depends only on Foundational — MVP
- **US2 (Phase 4)**: edits the step T003 created → depends on US1 completion (same RUN step, sequential by design)
- **US3 (Phase 5)**: validation-only; needs the final template text → after US2 (or after US1 if shipping MVP without retry)
- **Polish (Phase 6)**: after all desired stories

### Within Each Story

- Template edit (T003/T007) and its SPECIFICATION.md amendment (T004/T008) are different files → [P], but MUST land in the same commit (Principle I)
- e2e tasks follow their story's code tasks

### Parallel Opportunities

- T003 ∥ T004 (different files, one commit); T007 ∥ T008 likewise
- T010 ∥ T011 (independent scenarios, read-only against different network states)
- T013 ∥ T012

---

## Parallel Example: User Story 1

```bash
# In one working session (single commit):
Task: "T003 edit embed/Dockerfile.tmpl install step (verify + fail-loud)"
Task: "T004 amend SPECIFICATION.md §6.3 bullet"
# Then sequentially:
Task: "T005 e2e S1 happy path"
Task: "T006 e2e S2 fail-loud"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. T001–T002, then T003–T006
2. **STOP and VALIDATE**: S1 + S2(fail-loud) — the silent broken image is already impossible here
3. Shippable: without retry, a transient failure costs the user one `kekkai up` rerun instead of a wedged sandbox

### Incremental Delivery

1. US1 → validate → MVP
2. US2 (retry) → validate S2 full + S3
3. US3 → validate guardrails S4 + S5
4. Polish sweep + cleanup

Single-developer feature; parallelism is within-story only (code + spec amendment pairs).
