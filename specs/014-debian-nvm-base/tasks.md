# Tasks: Debian Base Image with Build-Time Node Install (nvm)

**Input**: Design documents from `/specs/014-debian-nvm-base/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/node-version.md, contracts/dockerfile.md, quickstart.md

**Tests**: Per constitution IV, validation is end-to-end against a real docker daemon (no unit-test suite). Each story phase ends with its quickstart scenario(s) as the test tasks.

**Organization**: Tasks grouped by user story; stories independently testable after the Foundational phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 (P1 pick any version), US2 (P2 zero-config lts + startup line), US3 (P3 fail-fast validation)

## Path Conventions

Single Go binary project: `cmd/`, `internal/`, `embed/` at repository root (see plan.md Project Structure).

---

## Phase 1: Setup

**Purpose**: Confirm a clean baseline; no project scaffolding needed (all changes are edits to existing files).

- [X] T001 Verify baseline: `go build ./...` succeeds and docker daemon responds (`docker version`); build test binary `go build -o kekkai-test ./cmd/kekkai`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Platform constants, selector translation, and render plumbing every story depends on. These three tasks touch interlocking APIs (removing `ResolvedBaseImage()` breaks `up.go` until T003/T004 land) — do them as one unit, sequentially.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete (code must compile).

- [X] T002 In `internal/config/config.go`: replace `debianRelease` constant with `DebianBaseImage = "debian:trixie"` and add `NvmVersion = "v0.40.5"` (comment: pinned nvm release tag, never master/latest — FR-003; both are kekkai-owned bake-time inputs); delete `ResolvedBaseImage()`; add `NodeInstallArg()` method on `ImageConfig` returning `"--lts"` for `lts`, else the selector verbatim (research.md R2)
- [X] T003 In `internal/runtime/up.go`: update `renderDockerfile` and its template data struct to `{DebianImage, NvmVersion, NodeInstallArg, NodeVersionRaw, AptPackages, ClaudeVersion}` (data-model.md §4); update `ensureImage` call sites; delete `baseImageMissing` and its invocation block (research.md R8); add `"bash"` to `builtinAptPackages` (research.md R10)
- [X] T004 In `internal/runtime/identity.go`: update `ConfigHash` signature/doc to take the new bake inputs (debian base + nvm tag + node selector + apt packages + firewall script, data-model.md §5); adjust the call in `up.go`; then `go build ./...` must pass

**Checkpoint**: Binary compiles; Dockerfile template render receives new fields (template itself still old — US1 rewrites it).

---

## Phase 3: User Story 1 - Pick any Node version, sandbox just works (Priority: P1) 🎯 MVP

**Goal**: `image.node_version: 22` builds on `debian:trixie` with nvm-installed Node 22.x; node/npm/npx work from every exec path; `npm install -g` needs no sudo.

**Independent Test**: quickstart.md Scenario 1 — set `node_version: "22"`, `kekkai up`, probe `docker exec <ctr> node -v` and `docker exec <ctr> sh -c 'node -v'` → both 22.x.

- [X] T005 [US1] Rewrite `embed/Dockerfile.tmpl` per contracts/dockerfile.md: `FROM {{.DebianImage}}`; keep gh apt-repo bootstrap + apt install of `{{.AptPackages}}`; create user `kekkai` UID/GID 1000, home `/home/kekkai`, shell `/bin/zsh` (replaces node-user rename, research.md R9); as `kekkai`: install nvm at `{{.NvmVersion}}` with `NVM_DIR=/home/kekkai/.nvm`, `SHELL ["/bin/bash","-c"]` + `BASH_ENV` file sourcing `nvm.sh` (FR-005), `nvm install {{.NodeInstallArg}}` wrapped with failure message naming `image.node_version` and `{{.NodeVersionRaw}}` (research.md R7), `nvm alias default`, stable symlink `/home/kekkai/.nvm/current` → resolved version dir; `npm install -g @anthropic-ai/claude-code@{{.ClaudeVersion}}` (nvm npm, user-owned); as root: symlink `node,npm,npx,claude` from `/home/kekkai/.nvm/current/bin/` into `/usr/local/bin` and set `ENV PATH=/home/kekkai/.nvm/current/bin:$PATH`; REMOVE `NPM_CONFIG_PREFIX` + npm-global block (research.md R5); keep zsh-history block, firewall COPY + sudoers block, WORKDIR/SHELL/CMD unchanged; add FR-010 comment at download steps (build runs on host network, domains never in allowed_domains/§5.4)
- [X] T006 [US1] E2E quickstart.md Scenario 1: build `kekkai-test`, config `node_version: "22"`, pseudo-TTY `up` (`script -qec "kekkai-test up" /dev/null </dev/null &`), verify all exec paths report same 22.x, `npm install -g cowsay` without sudo, runtime global on PATH via `sh -c`, zero root-owned files under `/home/kekkai`, workspace file host-owned; cleanup container + this test dir's history volume only
- [X] T007 [US1] E2E quickstart.md Scenario 4: `node_version: "99"` → validation passes, build fails, final error output names `image.node_version` (contracts/node-version.md "well-formed but nonexistent")

**Checkpoint**: MVP — user-chosen Node versions fully working on the Debian base.

---

## Phase 4: User Story 2 - Default and LTS users keep a zero-config experience (Priority: P2)

**Goal**: Omitted key / `lts` builds latest LTS; startup output prints resolved Node x.y.z next to Claude Code version on every `up`.

**Independent Test**: quickstart.md Scenario 2 — no config key, `kekkai up` succeeds, startup shows `kekkai sandbox: node v<x.y.z>, claude <x.y.z>`.

- [X] T008 [US2] In `embed/Dockerfile.tmpl` CMD: prepend startup line `echo "kekkai sandbox: node $(node -v), claude {{.ClaudeVersion}}"` before firewall init (research.md R6, contracts/dockerfile.md "Startup line"); firewall+claude exec behavior after it byte-identical
- [X] T009 [US2] E2E quickstart.md Scenario 2: no-config-key `up` → LTS installed, startup line shows concrete versions (SC-007); explicit `lts` → no rebuild (same hash) and identical behavior; `node_version: "22.11.0"` → exactly v22.11.0 (SC-002 all three shapes covered with T006)

**Checkpoint**: Zero-config path transparent and rebuild-stable.

---

## Phase 5: User Story 3 - Invalid or legacy config fails fast (Priority: P3)

**Goal**: Unsupported selectors and `image.base_image` rejected at config-parse time, before any docker activity, naming accepted forms / replacement key.

**Independent Test**: quickstart.md Scenario 3 — `node_version: lts/*` exits with validation error, zero docker activity.

**Note**: Independent of Phases 3–4 (touches only validation); can run in parallel with US1/US2 after Phase 2.

- [X] T010 [P] [US3] In `internal/config/validate.go`: replace `nodeVersionPattern` with `^(lts|[0-9]+(\.[0-9]+){0,2})$`; update error text to name the accepted forms `lts`, `22`, `22.11`, `22.11.0` (contracts/node-version.md); keep the explicit-empty error text; verify `legacyKeys` in `internal/config/config.go` still maps `image.base_image`/`image.base` → `image.node_version` (no change expected)
- [X] T011 [US3] E2E quickstart.md Scenario 3: each of `lts/*`, `node`, `stable`, `current`, `""`, and `image.base_image` fails pre-docker with contract error text (SC-003); confirm no build output / docker state change

**Checkpoint**: All three stories independently done.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: User-facing docs, spec-first compliance, remaining success criteria.

- [X] T012 [P] In `internal/runtime/init.go` starterConfig: update `node_version` comment to `"lts"` (default) or a version like `22`, `22.11`, `22.11.0` — remove `current` (FR-011)
- [X] T013 [P] In `README.md`: update the `image` section description to the new forms; remove any mention of a configurable node:* base image (FR-011)
- [X] T014 In `SPECIFICATION.md`: §4.2 schema comment + §4.3 `image.node_version` semantics (accepted forms, no node:* resolution), §5.1 apt builtins (+bash), §6.1 bake-time inputs (debian base + nvm tag constants; DELETE the Docker Hub manifest pre-check paragraph), §6.3 Dockerfile contract (align with contracts/dockerfile.md incl. startup line, kekkai user creation, nvm layout, no NPM_CONFIG_PREFIX). Constitution I: MUST land in the same commit as the code changes
- [X] T015 E2E quickstart.md Scenarios 5 & 6: user-domains-only config builds with no new `allowed_domains` (SC-006) and §9.6 firewall probes pass; upgrade from old node:*-based image rebuilds with zero config changes, `~/.claude`/gitconfig mounts + `kekkai shell`/`exec`/`traffic` work (SC-005, FR-009); complete quickstart sign-off checklist

---

## Dependencies

```text
Phase 1 (T001)
  └─► Phase 2 (T002 → T003 → T004, one compile unit)
        ├─► Phase 3 US1 (T005 → T006, T007)   🎯 MVP
        │     └─► Phase 4 US2 (T008 → T009)   (T008 edits the file T005 rewrote)
        └─► Phase 5 US3 (T010 → T011)         (parallel to US1/US2)
Phases 3–5 ─► Phase 6 (T012 ∥ T013 ∥ T014 → T015 last: full e2e sign-off)
```

- US2 depends on US1's Dockerfile rewrite (same file, CMD section) — sequential.
- US3 is fully independent after Foundational — parallelizable with US1/US2.

## Parallel Execution Examples

- After Phase 2: `T005` (Dockerfile, US1) ∥ `T010` (validate.go, US3) — different files, no shared state.
- Within Phase 6: `T012` ∥ `T013` (init.go, README.md) while `T014` (SPECIFICATION.md) proceeds; `T015` strictly last.
- E2E tasks (T006/T007, T009, T011) share the docker daemon and test dir — run sequentially even when their code tasks were parallel.

## Implementation Strategy

1. **MVP first**: Phases 1–3 only → users can pin any Node version on the Debian base (core value). Ship-testable via Scenario 1.
2. **Incremental**: Add Phase 4 (startup transparency) then Phase 5 (guardrails) — each independently verifiable via its scenario.
3. **Spec-first gate**: T014 (SPECIFICATION.md) must be committed together with the code it describes (constitution I); practically: keep the whole feature to one commit series ending with the full quickstart sign-off (T015).
4. **Rebuild expectation**: first `up` after T005 rebuilds every sandbox image (new bake inputs) — expected per spec edge case; mention in commit message.
