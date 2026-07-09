# Tasks: Sandbox-awareness context for Claude Code

**Input**: Design documents from `/specs/011-sandbox-awareness/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/sandbox-prompt.md, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation;
story phases run quickstart.md scenarios against a real sandbox (SC-001/SC-002
are one-shot manual model-behavior checks).

**Organization**: US1 carries the injection pipeline (prompt + gate + CMD),
US2 the machine-readable marker, US3 the degradation path validation.
Version plumbing is foundational (US1's gate needs it).

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Verify baseline: `go build ./... && go vet ./...` pass on clean tree

---

## Phase 2: Foundational (blocks US1)

- [X] T002 [P] In internal/selfupdate/selfupdate.go export `compareVersions` as `CompareVersions` (rename + doc comment, logic unchanged); update its internal call sites; `go build ./...` passes
- [X] T003 In internal/runtime/up.go change `ensureImage` to return `(tag, claudeVersion string, err error)`: pinned version or `latest`-resolution result; empty string on the registry-fallback image-reuse path (version unknown, research.md R2); update the call in `Up`; `go build ./...` passes

**Checkpoint**: resolved claude version available at run-args assembly

---

## Phase 3: User Story 1 - Claude diagnoses sandbox-caused failures correctly (Priority: P1) 🎯 MVP

**Goal**: pinned prompt (+ config summary) delivered byte-for-byte via `--append-system-prompt` on supported versions

**Independent Test**: quickstart.md scenarios 2-8, 10-11

- [X] T004 [US1] Create internal/runtime/sandboxprompt.go: `const sandboxPrompt` = spec's pinned text verbatim; `const appendPromptMinVersion = "1.0.51"`; `sandboxPromptFor(cfg *config.Config) string` appending the FR-008 summary per contracts/sandbox-prompt.md (allowed domains+CIDRs and secrets.hide names, max 8 items per list then `...and N more`, ≤10 lines, empty lists omitted, `allow_all` → `- network: unrestricted (allow_all)`, no section when all empty); `supportsAppendPrompt(version string) bool` using `selfupdate.CompareVersions` (unknown/empty version → false)
- [X] T005 [US1] In internal/runtime/up.go wire the gate into `buildRunArgs` (needs the claude version passed through from `Up`): supported → `addEnv("KEKKAI_SYSTEM_PROMPT", sandboxPromptFor(cfg))` in the builtin env block; unsupported/unknown → one stderr warning via `yellow()` exactly as pinned in contracts/sandbox-prompt.md (`warning: claude <version> does not support sandbox context injection (needs >= 1.0.51), starting without it`); `go build ./... && go vet ./...` pass
- [X] T006 [US1] In embed/Dockerfile.tmpl replace the CMD with the conditional from contracts/sandbox-prompt.md: `sudo /usr/local/bin/init-firewall.sh && if [ -n "$KEKKAI_SYSTEM_PROMPT" ]; then exec claude $CLAUDE_ARGS --append-system-prompt "$KEKKAI_SYSTEM_PROMPT"; else exec claude $CLAUDE_ARGS; fi` — `$KEKKAI_SYSTEM_PROMPT` double-quoted, `$CLAUDE_ARGS` unquoted (research.md R1)
- [X] T007 [P] [US1] Amend SPECIFICATION.md: §5.3 builtin env gains `KEKKAI_SANDBOX=1` and gated `KEKKAI_SYSTEM_PROMPT` (with the v1.0.51 threshold and warning behavior, pointing at specs/011-sandbox-awareness/contracts/sandbox-prompt.md); §6.3/§7.2 CMD updated to the conditional form; update README.md (sandbox-awareness mention in What you get; `KEKKAI_SANDBOX` in Configure or limitations as fits) (constitution I, same change as code)
- [X] T008 [US1] e2e per quickstart.md: build `/tmp/kekkai-test`, fresh `up` under pseudo-TTY (CMD change rebuilds image — expected); scenarios 2-3 (env var + intact multiline flag on the claude process), 6-8 (summary content, truncation at 12 domains, allow_all line), 10-11 (user `claude.args` and after-`--` extras compose); manual one-shot scenarios 4-5 (ask "are you running in a sandbox?" → references Kekkai; ask to fetch a blocked domain → suggests allowed_domains + `kekkai up`, no workaround) — record both answers in the task notes

**Checkpoint**: prompt delivered intact and acted on — MVP delivered

> T008 model-check answers (2026-07-09, print-mode claude with the injected prompt):
> S4: "Yes. I'm running inside Kekkai, a security sandbox: filesystem limited to the workspace, network restricted to an allowlist (currently only registry.npmjs.org), and `.env.production` is shadowed as a secret file."
> S5 (blocked api.github.com fetch): named the sandbox as the cause, gave the exact `network.allowed_domains` YAML addition, said restart with `kekkai up` — no workaround suggested.

---

## Phase 4: User Story 2 - Scripts and tools can detect the sandbox (Priority: P2)

**Goal**: `KEKKAI_SANDBOX=1` visible to every sandbox process

**Independent Test**: quickstart.md scenario 1

- [X] T009 [US2] In internal/runtime/up.go add `addEnv("KEKKAI_SANDBOX", "1")` to the §5.3 builtin block (before user env, unconditional — independent of the version gate, research.md R3)
- [X] T010 [US2] e2e per quickstart.md scenario 1: `kekkai exec sh -c 'echo $KEKKAI_SANDBOX'` prints `1` (SC-003)

**Checkpoint**: machine-readable marker live

---

## Phase 5: User Story 3 - Graceful degradation on unsupported versions (Priority: P3)

**Goal**: old pinned versions start normally, minus the context, plus one warning

**Independent Test**: quickstart.md scenario 9

- [X] T011 [US3] e2e per quickstart.md scenario 9: pin `claude.version: 1.0.40`, `up` → exactly one yellow warning matching the contract string, container starts, `docker inspect` shows no `KEKKAI_SYSTEM_PROMPT`, claude process has no `--append-system-prompt`; restore `latest` afterwards (FR-006, SC-005)

**Checkpoint**: all three stories independently validated

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T012 e2e per quickstart.md scenarios 12-13: no `CLAUDE.md` created/modified in workspace, nothing kekkai-written in `~/.claude` (SC-004); fresh-`up` regression — §9.6 firewall probes pass, missing-config warning and update notice unchanged
- [X] T013 Cleanup test resources (`docker rm -f` test sandbox; remove ONLY this test dir's `kekkai-history-<hash>` volume — never bulk-remove `kekkai-history-*`) and final `go build ./... && go vet ./...`

---

## Dependencies & Execution Order

- T001 → T002 [P] ∥ T003 → T004 → T005 → T006 → T008
- T007 [P] anytime after T001 (docs-only)
- T009 after T005 (same up.go region) → T010; T011 after T008; T012 → T013 last

## Parallel Opportunities

- T002 (selfupdate) ∥ T003 (up.go) — different files
- T007 (SPECIFICATION.md/README) ∥ any code task
- T008/T010/T011 share one rebuilt image; scenarios batch against the same sandbox where config permits (scenario 9 needs its own `up`)

## Implementation Strategy

MVP = Phases 2-3 (~80 net lines: sandboxprompt.go dominates, up.go wiring,
one CMD line). US2 is two lines + one check. US3 is validation only — its
behavior ships inside T004/T005's gate. The image rebuild lands once at T008's
first `up` and covers all later scenarios except scenario 9's version-pinned
build.
