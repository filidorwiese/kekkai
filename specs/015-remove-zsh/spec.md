# Feature Specification: Single-Shell Sandbox (bash) and Release Build Repair

**Feature Branch**: `015-remove-zsh`

**Created**: 2026-07-09

**Status**: Draft

**Input**: User description: "remove zsh, drop the p10k var too. also there seems to be a build error on github actions since last change: go.mod requires go >= 1.25.0 (running go 1.24.13; GOTOOLCHAIN=local)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Releases build again (Priority: P1)

A maintainer tags a release. The GitHub Actions release pipeline builds all three platform binaries (linux/amd64, linux/arm64, darwin/arm64) with a Go toolchain that satisfies the module's minimum version, packages them, and publishes the release. Today every build job fails immediately with "go.mod requires go >= 1.25.0 (running go 1.24.13; GOTOOLCHAIN=local)", so no release can ship at all.

**Why this priority**: A broken release pipeline blocks distribution of every change, including the other stories in this feature. Nothing ships until this is fixed.

**Independent Test**: Push a tag (or run the workflow's build steps locally with the same pinned toolchain resolution); all three matrix builds compile and produce tarballs.

**Acceptance Scenarios**:

1. **Given** the current `go.mod` minimum Go version, **When** the release workflow runs, **Then** every matrix build compiles successfully with a toolchain that satisfies that minimum.
2. **Given** a future bump of the Go version in `go.mod`, **When** the release workflow runs, **Then** the CI toolchain follows automatically — no second place needs a manual, drift-prone version bump.

---

### User Story 2 - Sandbox runs on a single shell: bash (Priority: P2)

A kekkai user opens an interactive session with `kekkai shell` and lands in bash. Their command history still persists across sandbox restarts for the same project. Claude Code and all its subprocesses keep working exactly as before. The image no longer carries a second shell that existed only for interactive sessions.

**Why this priority**: Surface reduction (constitution: less code = less debt). Since feature 014, bash is already a pinned, load-bearing part of the image; zsh remained only as the interactive nicety. One shell means one history mechanism, one login-shell story, and one less baked package.

**Independent Test**: Run `kekkai up`, then `kekkai shell` → bash prompt; run commands, exit, restart the sandbox, `kekkai shell` again → history from the previous session is available.

**Acceptance Scenarios**:

1. **Given** a running sandbox, **When** the user runs `kekkai shell`, **Then** an interactive bash session opens in the workspace (no zsh present in the image).
2. **Given** commands were executed in a `kekkai shell` session, **When** the sandbox is stopped and started again for the same project, **Then** those commands appear in the shell history of the new session.
3. **Given** the running sandbox, **When** the environment advertises the user's shell (login shell, `$SHELL`), **Then** both point at bash — no reference to zsh remains anywhere in the container.
4. **Given** the new image, **When** Claude Code starts and spawns subprocesses, **Then** startup, firewall init, and command execution behave identically to before.

---

### User Story 3 - Dead configuration is removed (Priority: P3)

The sandbox environment no longer carries the `POWERLEVEL9K_DISABLE_GITSTATUS` variable — a leftover from the devcontainer heritage that references a zsh theme which was never installed in the kekkai image. Users see one less mystery variable when inspecting their sandbox environment.

**Why this priority**: Pure cleanup; zero user-visible behavior change. Rides along with the zsh removal because that is what makes it unambiguously dead.

**Independent Test**: Start a sandbox and inspect the container environment; the variable is absent and nothing misbehaves.

**Acceptance Scenarios**:

1. **Given** a freshly started sandbox, **When** the container environment is listed, **Then** `POWERLEVEL9K_DISABLE_GITSTATUS` is not set.

---

### Edge Cases

- Existing per-project history volumes contain the old zsh history file: the new bash history starts empty for those projects (old file remains in the volume, unread). Fresh projects are unaffected. This one-time reset is accepted — history is a convenience, not data of record.
- A user's own tooling inside the sandbox invokes `zsh` explicitly (scripts, muscle memory): it now fails with "command not found". Users who want zsh back can add it via `image.apt_packages` — but their interactive `kekkai shell` remains bash.
- The user config `env` section could theoretically set `POWERLEVEL9K_DISABLE_GITSTATUS` themselves: still allowed (it is not a reserved key); kekkai just stops injecting it.
- Shell change alters bake-time inputs: the next `kekkai up` after upgrading triggers an image rebuild (expected; same rule as every image change).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The release pipeline MUST build with a Go toolchain that satisfies the module's declared minimum version, and MUST derive that version from the module definition itself (single source of truth) so future bumps cannot drift.
- **FR-002**: The sandbox image MUST contain exactly one shell for interactive and scripted use: bash. zsh MUST NOT be installed, and no zsh artifacts (rc files, history wiring, login-shell references) may remain in the image.
- **FR-003**: `kekkai shell` MUST open an interactive bash session in the workspace of the running sandbox.
- **FR-004**: The sandbox user's login shell and the advertised shell environment (`$SHELL`) MUST be bash.
- **FR-005**: Interactive shell history MUST persist across sandbox restarts for the same project, appending incrementally (a crashed session does not lose earlier commands), using the same per-project history volume as today.
- **FR-006**: The `POWERLEVEL9K_DISABLE_GITSTATUS` environment variable MUST no longer be injected into the sandbox.
- **FR-007**: All other container behavior — startup version line, firewall init, Claude Code launch, exec paths for node/npm/npx/claude — MUST be byte-for-byte unaffected in observable behavior.
- **FR-008**: User-facing documentation and help text that mention zsh (command help, README, spec) MUST be updated to say bash.

### Key Entities

- **Builtin apt package set (code constant)**: loses `zsh`; bash remains (pinned since feature 014).
- **Per-project history volume**: unchanged identity and mount point; the history file inside it changes from zsh's to bash's format/name.
- **Release workflow toolchain source**: the module definition's Go version becomes the only place the required toolchain is declared.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A tagged release completes end-to-end: 3/3 platform builds succeed and a published release with tarballs and checksums exists.
- **SC-002**: `kekkai shell` lands in bash in 100% of sessions; `which zsh` inside the sandbox finds nothing.
- **SC-003**: Commands from a previous session appear in history after a sandbox restart for the same project.
- **SC-004**: Container environment contains zero occurrences of `POWERLEVEL9K_DISABLE_GITSTATUS`.
- **SC-005**: All feature-014 exec-path checks (node/npm/npx/claude from `docker exec`, `sh -c`, interactive shell) still pass on the rebuilt image.
- **SC-006**: The sandbox image shrinks (zsh and its support files gone); no new packages are added.

## Assumptions

- The Go toolchain fix belongs to this feature as its P1 story despite being unrelated to the shell change; the user bundled them and the pipeline must work before anything ships.
- Deriving the CI Go version from the module file (rather than bumping a second pinned number) is the intended "cannot drift again" fix implied by "since last change".
- The one-time interactive-history reset for existing projects is acceptable and needs no migration tooling.
- `POWERLEVEL9K_DISABLE_GITSTATUS` does not become a reserved env key; users may still set it themselves.
- Constitution §5.1 (builtin apt set) and §6.3 (Dockerfile contract) amendments land in the same commit as the change, per Spec-First.
- Out of scope: making the interactive shell configurable, shipping any zsh compatibility layer, migrating old zsh history into bash.
