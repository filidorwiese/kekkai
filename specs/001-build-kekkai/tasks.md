# Tasks: Build Kekkai

**Input**: Design documents from `/specs/001-build-kekkai/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: No unit-test tasks (constitution IV: validation is end-to-end against a real docker
daemon). Each story phase ends with an e2e validation task running its quickstart scenarios.

**Organization**: Tasks grouped by user story from spec.md. US1+US2 are both P1 and land
sequentially (US2's firewall configurability builds on US1's locked-down sandbox).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1–US5 from spec.md

## Path Conventions

Single Go module at repo root per plan.md: `cmd/kekkai/`, `internal/{config,runtime,docker}/`,
`embed/`, `install.sh`, `.github/workflows/release.yml`.

---

## Phase 1: Setup

**Purpose**: Go module and skeleton matching plan.md structure

- [ ] T001 Initialize Go module `go.mod` (module kekkai, Go 1.24) and add `gopkg.in/yaml.v3`
- [ ] T002 [P] Create directory skeleton `cmd/kekkai/`, `internal/config/`, `internal/runtime/`, `internal/docker/`, `embed/` with placeholder `main.go` that prints usage
- [ ] T003 [P] Add `.gitignore` (built binaries, `/tmp` artifacts) at repo root

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Identity, config, docker plumbing, and dispatch every story depends on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Implement identity derivation in `internal/runtime/identity.go`: container name `kekkai-<sanitized-basename>-<sha256(PWD)[:8]>`, authoritative label `kekkai.cwd`, labels `kekkai.image_hash`/`kekkai.version`, history volume name, image tag `kekkai:<sha256(rendered Dockerfile + init-firewall.sh)[:12]>` (data-model.md "Derived identity")
- [ ] T005 [P] Implement config schema + loading in `internal/config/config.go`: structs per data-model.md, `.kekkai.{yml,yaml}` discovery (both → error, none → exact error from contracts/cli.md), strict `KnownFields(true)` decode, `~`/`${VAR}` expansion and target inference (§4.3)
- [ ] T006 Implement validation in `internal/config/validate.go`: all rules from contracts/config.md (base_image `node:*`, claude.version, mount source/duplicate targets, unset `${VAR}` vs optional, reserved env keys, ssh_agent⇒git.enabled, allow_all exclusivity, CIDR parse, domain whitespace, limits) plus legacy-key targeted error; collect and report ALL violations in one pass
- [ ] T007 [P] Implement docker CLI helpers in `internal/docker/cli.go`: image inspect/build, container run/stop/rm/ps by label, volume ls/rm — all shelling out to `docker` (research.md R1)
- [ ] T008 [P] Implement interactive exec with SIGINT/SIGTERM forwarding in `internal/docker/exec.go` (§7.2, research.md R9)
- [ ] T009 Implement dispatch in `cmd/kekkai/main.go`: stdlib `flag` per subcommand, subcommands exactly per contracts/cli.md, `version` via `-ldflags -X main.version`, `help` usage, unknown subcommand → usage + exit ≠0

**Checkpoint**: `go build ./... && go vet ./...` pass; `kekkai version|help` work

---

## Phase 3: User Story 1 - Start a sandboxed autonomous session (Priority: P1) 🎯 MVP

**Goal**: `kekkai init` → `kekkai up` reaches an autonomous Claude session in a locked-down
(builtins-only) container; exit removes it

**Independent Test**: quickstart.md Scenarios 1, 2, 7 — init/up happy path, fail-early
validation, version-resolution fallback

- [ ] T010 [P] [US1] Author `embed/Dockerfile.tmpl` per contracts/sandbox.md: node base, user rename node→kekkai (UID kept), builtin apt set §5.1 + user packages, npm global prefix + claude install at resolved version, zsh history wiring, copy init-firewall.sh, single sudoers grant, no docker CLI
- [ ] T011 [P] [US1] Author `embed/init-firewall.sh` base per contracts/sandbox.md: flush preserving Docker DNS NAT, loopback/udp53/established, bridge subnet from own route, ipset with builtin hosts (api.anthropic.com, statsig.anthropic.com via dig), DROP default, icmp-admin-prohibited reject, verification probes (example.com FAILS, api.anthropic.com SUCCEEDS, abort on violation)
- [ ] T012 [P] [US1] Implement `kekkai init` in `internal/runtime/init.go`: starter file per contracts/config.md (active values = defaults, optional sections commented with README-grade comments, GH_TOKEN example beside allow_github), error if config exists
- [ ] T013 [US1] Implement image pipeline in `internal/runtime/up.go`: `go:embed` assets, resolve `claude.version: latest` via `registry.npmjs.org/@anthropic-ai/claude-code/latest` with fallback to newest existing `kekkai:*` image + warning (research.md R5), render template, hash-tag, build only on inspect miss, `--verbose` plain progress
- [ ] T014 [US1] Implement run-args assembly + orchestration in `internal/runtime/up.go`: validate-first (abort pre-docker), refuse existing container for `kekkai.cwd` unless `--force`, caps NET_ADMIN/NET_RAW, builtin mounts ($PWD→/workspace rw, ~/.claude rw, history volume), builtin env §5.3, `CLAUDE_ARGS` (default or claude.args verbatim + args after `--`), CMD `sudo /usr/local/bin/init-firewall.sh && exec claude $CLAUDE_ARGS`, `docker run --rm -it` via exec.go, labels from identity.go
- [ ] T015 [US1] e2e validate: run quickstart.md Scenarios 1, 2, 7 against a real docker daemon; fix or reconcile SPECIFICATION.md in same commit on any deviation

**Checkpoint**: MVP — autonomous locked-down session works end to end

---

## Phase 4: User Story 2 - Network egress is deny-by-default (Priority: P1)

**Goal**: User-controlled allowances (allow_github, domains, CIDRs) and the allow_all escape
hatch, all probe-verified

**Independent Test**: quickstart.md Scenario 3 — default deny, github/domain/CIDR allowances,
allow_all warning

- [ ] T016 [US2] Inject firewall env in `internal/runtime/up.go`: `ALLOW_ALL`, `ALLOW_GITHUB`, `ALLOWED_DOMAINS`, `ALLOWED_CIDRS` from `network.*`, applied after user env so firewall vars stay authoritative (§7.3; env-not-bindmount per research.md R6)
- [ ] T017 [US2] Extend `embed/init-firewall.sh`: `ALLOW_ALL=1` → no restrictions + prominent warning + skip verification; `ALLOWED_DOMAINS` dig once warn+skip on failure; `ALLOWED_CIDRS` literals; `ALLOW_GITHUB=1` → api.github.com/meta CIDRs (jq-validated, aggregated, fetch failure fatal, pre-lockdown) + `https://api.github.com/zen` probe
- [ ] T018 [US2] e2e validate: run quickstart.md Scenario 3; verify no blanket tcp/22, LAN unreachable without CIDR entry

**Checkpoint**: Full network contract (contracts/sandbox.md "Firewall") holds

---

## Phase 5: User Story 3 - Control what the agent can read and touch (Priority: P2)

**Goal**: git modes, ssh-agent, user mounts, secrets shadowing, env passthrough, resource limits

**Independent Test**: quickstart.md Scenarios 4, 5 — secrets/env/limits and git modes

- [ ] T019 [US3] Implement git mounts in `internal/runtime/up.go` per §5.2: enabled → `~/.gitconfig` ro; disabled/omitted → `$PWD/.git` ro bind when repo (skip otherwise); ssh_agent → `$SSH_AUTH_SOCK`→`/ssh-agent` + env + allowed_signers ro optional, hard error at up when host socket unset
- [ ] T020 [US3] Implement user disk.mounts, user env, and `limits` (`--cpus`/`--memory`) in `internal/runtime/up.go`: assembly order per data-model.md "Container run inputs"; optional-missing source → skip+notice, non-optional missing → warn
- [ ] T021 [US3] Implement secrets shadowing in `internal/runtime/up.go` per §8: host stat before run; file → `/dev/null:<path>:ro`, dir → tmpfs, missing → warn+skip; never create host artifacts
- [ ] T022 [US3] e2e validate: run quickstart.md Scenarios 4, 5; confirm no-commit mode unbreakable from inside (no SYS_ADMIN remount)

**Checkpoint**: All disk/secrets/git/env/limits knobs behave per contract

---

## Phase 6: User Story 4 - Manage sandbox lifecycle across projects (Priority: P2)

**Goal**: ps/shell/down/prune resolving by `kekkai.cwd` label only

**Independent Test**: quickstart.md Scenario 6 — two concurrent projects

- [ ] T023 [P] [US4] Implement `kekkai down` in `internal/runtime/down.go`: stop+remove by label, exit 0 with report when nothing found
- [ ] T024 [P] [US4] Implement `kekkai shell` in `internal/runtime/shell.go`: `docker exec -it <container> zsh` resolved by label, error when no running sandbox
- [ ] T025 [P] [US4] Implement `kekkai ps` in `internal/runtime/ps.go`: list by kekkai labels — name, cwd, image hash, status
- [ ] T026 [US4] Implement `kekkai prune` in `internal/runtime/prune.go`: orphan containers + unused `kekkai:*` images, `--volumes` adds history volumes, confirmation prompt unless `--yes`, never touches running sandboxes' resources
- [ ] T027 [US4] e2e validate: run quickstart.md Scenario 6 with two projects

**Checkpoint**: Day-2 operations complete

---

## Phase 7: User Story 5 - Install and update kekkai (Priority: P3)

**Goal**: Tag-driven releases and curl-pipe install

**Independent Test**: quickstart.md Scenario 8 — tag on fork produces release; install.sh yields working binary

- [ ] T028 [P] [US5] Author `.github/workflows/release.yml`: on `v*` tag — matrix GOOS=linux GOARCH={amd64,arm64}, CGO_ENABLED=0, `-ldflags "-X main.version=<tag>"`, tar.gz per arch, `SHA256SUMS`, GitHub release (research.md R11)
- [ ] T029 [P] [US5] Author `install.sh` at repo root: latest tag via GH API (override `KEKKAI_VERSION`), arch detection (amd64/arm64), download + checksum verify, install to `~/.local/bin/`, PATH hint
- [ ] T030 [US5] e2e validate: run quickstart.md Scenario 8 (rc tag on fork + clean-machine install)

**Checkpoint**: Distribution pipeline proven

---

## Phase 8: Polish & Cross-Cutting Concerns

- [ ] T031 [P] Update `README.md` as user-facing digest of SPECIFICATION.md (constitution I): install, quickstart, config reference, threat-model limits §2
- [ ] T032 Full validation sweep: `go build`/`go vet`, run ALL quickstart.md scenarios end to end, reconcile any SPECIFICATION.md drift in the same commit

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)** → **Foundational (Phase 2)** → user stories
- **US1 (Phase 3)** blocks **US2 (Phase 4)** (firewall env/script extensions build on up.go + base script)
- **US3 (Phase 5)** depends on US1 (extends up.go run-args); independent of US2
- **US4 (Phase 6)** depends on Foundational only (needs identity.go + cli.go); testable best after US1
- **US5 (Phase 7)** depends on Setup only (builds the binary); validate last
- **Polish (Phase 8)** after all stories

### Within phases

- T005 → T006 (validate needs schema); T004, T007, T008 parallel to both; T009 after T004–T008
- T010/T011/T012 parallel; T013 → T014 (same file, pipeline before orchestration)
- T016 → T017 → T018; T019 → T020 → T021 sequential (all edit up.go)
- T023/T024/T025 parallel; T026 after T025 (shares listing helpers)

### Parallel Opportunities

- Phase 2: T005, T007, T008 together; T004 alongside
- Phase 3: T010, T011, T012 together
- Phase 6: T023, T024, T025 together
- Phase 7: T028, T029 together; whole phase can run parallel to Phases 5–6 if staffed

---

## Parallel Example: User Story 1

```bash
Task: "Author embed/Dockerfile.tmpl per contracts/sandbox.md"
Task: "Author embed/init-firewall.sh base (builtins + DROP + verification)"
Task: "Implement kekkai init in internal/runtime/init.go"
```

---

## Implementation Strategy

### MVP First (US1)

1. Phases 1–2 (setup + foundational)
2. Phase 3 (US1) → **STOP and VALIDATE** Scenarios 1, 2, 7
3. A locked-down autonomous sandbox is already usable value

### Incremental Delivery

Each subsequent phase (US2 network config → US3 disk/git knobs → US4 lifecycle → US5
distribution) is independently validatable via its quickstart scenario before moving on; polish
last. Commit after each task or logical group; any spec deviation found during e2e lands in
SPECIFICATION.md in the same commit (constitution I).
