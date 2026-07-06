# Feature Specification: Build Kekkai

**Feature Branch**: `001-build-kekkai`

**Created**: 2026-07-06

**Status**: Draft

**Input**: User description: "Build kekkai per SPECIFICATION.md"

> Design authority: `SPECIFICATION.md` at the repo root is the source of truth (per constitution
> Principle I). This spec restates it as prioritized, testable user journeys and requirements;
> on any conflict, SPECIFICATION.md wins and both documents must be reconciled in the same commit.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Start a sandboxed autonomous session (Priority: P1)

A developer in a project directory creates a starter config with `kekkai init`, edits it if
needed, and runs `kekkai up`. Kekkai builds the sandbox image if needed, starts an isolated
container with the project mounted, locks down network egress, and drops the developer into a
fully autonomous Claude Code session. Exiting the session removes the container.

**Why this priority**: This is the product. Without init/up producing a working, locked-down
session, nothing else has value.

**Independent Test**: In a scratch project, run `kekkai init` then `kekkai up`; verify Claude
starts, the project is writable at `/workspace`, `https://example.com` is blocked from inside,
`https://api.anthropic.com` is reachable, and the container is gone after exit.

**Acceptance Scenarios**:

1. **Given** a project without `.kekkai.yaml`, **When** the user runs `kekkai init`, **Then** a
   starter config is written whose active values equal built-in defaults; a second `kekkai init`
   errors.
2. **Given** a project with a valid config, **When** the user runs `kekkai up`, **Then** the image
   is built only if missing, the container starts, the egress firewall is applied and verified,
   and Claude Code launches autonomously in `/workspace`.
3. **Given** a project without a config, **When** the user runs `kekkai up`, **Then** it errors
   with "no .kekkai.yaml found, run `kekkai init`" and touches nothing.
4. **Given** an invalid config (unknown key, bad CIDR, missing base image), **When** the user runs
   `kekkai up`, **Then** all violations are reported in one pass and no docker work happens.
5. **Given** a running sandbox for the same directory, **When** the user runs `kekkai up`, **Then**
   it refuses unless `--force` is given.
6. **Given** a running session, **When** the user exits Claude or sends SIGINT/SIGTERM, **Then**
   the container is removed.

---

### User Story 2 - Network egress is deny-by-default (Priority: P1)

The sandbox blocks all outbound traffic except explicitly allowed destinations: the built-in
Anthropic endpoints always, plus whatever the user opts into (`allow_github`, domains, CIDRs).
Startup self-verifies the firewall and aborts if it does not hold.

**Why this priority**: The container-as-boundary threat model collapses without enforced egress
control; equal-first priority with US1 because `up` must never start Claude behind a broken
firewall.

**Independent Test**: Start a sandbox with no `network` block; from inside, verify only Anthropic
endpoints are reachable. Add `allow_github` / a domain / a CIDR and verify each becomes reachable
while everything else stays blocked.

**Acceptance Scenarios**:

1. **Given** no `network` config, **When** the sandbox starts, **Then** only built-in Anthropic
   hosts (plus loopback, DNS, docker bridge subnet) are reachable; all other egress is rejected.
2. **Given** `allow_github: true`, **When** the sandbox starts, **Then** GitHub API/git/ssh work
   and `https://api.github.com/zen` is probe-verified.
3. **Given** any firewall verification probe fails (example.com reachable, or a required endpoint
   unreachable), **When** starting, **Then** startup aborts before Claude runs.
4. **Given** `allow_all: true` alone, **When** the sandbox starts, **Then** no restrictions apply
   and a prominent warning is printed; combined with any other network key it is a validation
   error.

---

### User Story 3 - Control what the agent can read and touch (Priority: P2)

The user decides disk exposure: extra mounts with read-only/optional flags, secrets hidden by
exact path, git identity opt-in (commits carry user identity only when enabled; without it the
repo history is readable but commits are impossible), optional ssh-agent forwarding, env
passthrough, and CPU/memory limits.

**Why this priority**: Core protections beyond network; builds on a working US1 sandbox.

**Independent Test**: Configure each knob in `.kekkai.yaml`, start the sandbox, and observe the
effect from inside (secret reads empty, commit fails without git enabled, mount is read-only,
limits applied).

**Acceptance Scenarios**:

1. **Given** `secrets.hide` listing a file and a directory, **When** the sandbox runs, **Then**
   the file reads empty and the directory appears empty; missing listed paths warn and skip,
   creating no host artifacts.
2. **Given** `git.enabled: true`, **When** the agent commits, **Then** the commit carries the
   user's identity; **Given** git disabled/omitted, **Then** `git log/diff/show` work but
   commit/stage/fetch fail, with no way for the agent to lift the restriction from inside.
3. **Given** a mount with `optional: true` and a missing source, **Then** it is skipped silently
   with a notice; non-optional missing source warns. Unset `${VAR}` in a non-optional context
   errors.
4. **Given** `git.ssh_agent: true` without `git.enabled: true`, **Then** validation fails; with
   no host agent socket at `up`, hard error.
5. **Given** `limits`, **Then** the container runs with those CPU/memory caps.

---

### User Story 4 - Manage sandbox lifecycle across projects (Priority: P2)

The user inspects and cleans up: `kekkai ps` lists running sandboxes, `kekkai shell` opens a
shell in the current project's sandbox, `kekkai down` stops it, `kekkai prune` removes orphaned
containers and unused images (optionally history volumes), `kekkai version`/`help` inform.

**Why this priority**: Day-2 operations; valuable once sandboxes exist.

**Independent Test**: With two projects sandboxed, verify `ps` lists both, `shell`/`down` resolve
the current directory's container only, and `prune` removes leftovers after confirmation.

**Acceptance Scenarios**:

1. **Given** a running sandbox for `$PWD`, **When** `kekkai shell`, **Then** an interactive shell
   opens inside it; `kekkai down` stops and removes it.
2. **Given** sandboxes for multiple directories, **Then** commands resolve the container by the
   recorded working-directory label, never by name guessing.
3. **Given** orphan containers or unused kekkai images, **When** `kekkai prune`, **Then** they are
   listed and removed after confirmation (`--yes` skips it; `--volumes` includes history volumes).

---

### User Story 5 - Install and update kekkai (Priority: P3)

A user installs kekkai with a one-line curl of `install.sh`, which fetches the latest released
binary for their platform into `~/.local/bin/`. Tagged releases publish binaries for both
supported architectures with checksums.

**Why this priority**: Distribution matters for adoption but not for local validation of the tool
itself.

**Independent Test**: Run `install.sh` on a clean Linux machine (amd64 and arm64); `kekkai
version` prints the release version.

**Acceptance Scenarios**:

1. **Given** a published release, **When** the install script runs, **Then** the correct
   architecture's binary lands in `~/.local/bin/` and reports the expected version.
2. **Given** a `v*` tag is pushed, **Then** CI produces linux amd64+arm64 tarballs, a checksum
   file, and a GitHub release.

---

### Edge Cases

- Both `.kekkai.yml` and `.kekkai.yaml` present → error.
- Config uses pre-rewrite schema keys → targeted "schema changed" error pointing at `kekkai init`.
- npm registry unreachable while `claude.version: latest` → reuse newest existing image for this
  config with a warning; no matching image → failure.
- User-configured domain fails DNS resolution at startup → warn and skip that domain, continue.
- GitHub CIDR fetch fails while `allow_github: true` → fatal (fetch happens before lockdown).
- Project directory is not a git repo with git disabled → read-only `.git` bind skipped.
- Reserved env keys (`WORKSPACE`, firewall variables, `SSH_AUTH_SOCK`) in user env → rejected.
- Duplicate mount targets → validation error.
- Same project directory sandboxed twice concurrently → refused (single sandbox per folder).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: CLI MUST provide exactly: `init`, `up`, `down`, `shell`, `ps`, `prune`, `version`,
  `help` (SPECIFICATION.md §3). No `config`/`doctor`.
- **FR-002**: `up` MUST validate the entire config before any docker work and report all
  violations in one pass (§4.4).
- **FR-003**: Config is a single required per-project file with strict unknown-key rejection and
  the schema of §4.2; legacy keys get a targeted migration error (§4.1).
- **FR-004**: Mount/env expansion and target inference MUST follow §4.3 semantics.
- **FR-005**: `init` MUST write a starter config whose active values equal defaults, with all
  optional sections present but commented (§4.5).
- **FR-006**: The image MUST be derived only from base image, apt packages, and resolved Claude
  version; content-hash tagged; rebuilt only on hash miss. Runtime settings MUST never trigger
  rebuilds (§6.1).
- **FR-007**: `claude.version: latest` MUST resolve to a concrete version before hashing so new
  releases rebuild; registry failure falls back to the newest existing image with a warning
  (§6.2).
- **FR-008**: Containers MUST be identified by a working-directory label as the authoritative
  key, one sandbox per directory, auto-removed on exit/signal (§7).
- **FR-009**: Egress firewall MUST be deny-by-default with built-in Anthropic allowances,
  user-configured additions, mandatory startup verification probes, and an explicit
  all-or-nothing escape hatch (§5.4, §9).
- **FR-010**: Firewall inputs MUST reach the container via environment variables, never
  bind-mounted files (§9); the firewall script is the only root grant inside (§6.3).
- **FR-011**: Secrets hiding MUST shadow exact paths (file → empty read, directory → empty),
  stat-gated on the host so missing paths create no artifacts (§8).
- **FR-012**: Git integration MUST follow §5.2: identity mount when enabled; enforceable
  read-only history when disabled; ssh-agent forwarding only with git enabled.
- **FR-013**: Workspace, Claude config dir, and per-project command history MUST persist across
  sessions; everything else in the container is disposable (§5.2).
- **FR-014**: Releases MUST ship static linux amd64+arm64 binaries with checksums via tag-driven
  CI, installable by the curl script (§10).
- **FR-015**: Out-of-scope items of §11 MUST NOT be implemented (docker-in-sandbox rejected by
  threat model; macOS/Windows; self-update; devcontainer integration; multi-sandbox; per-project
  Dockerfile; global config).

### Key Entities

- **Config (`.kekkai.yaml`)**: per-project declaration of image, claude, git, disk, env, network,
  secrets, limits; sections off when omitted.
- **Sandbox image**: content-addressed build product of bake-time inputs only.
- **Sandbox container**: single running instance per project directory, keyed by cwd label,
  ephemeral.
- **History volume**: per-project persistent shell history.
- **Firewall ruleset**: deny-by-default egress policy assembled at container start from built-ins
  plus user allowances, self-verified.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: From a fresh project, `init` → `up` reaches a working autonomous Claude session in
  under 2 minutes on first build, under 15 seconds on warm image (excluding Claude's own startup).
- **SC-002**: 100% of startups either pass all firewall verification probes or abort before
  Claude runs; zero sessions ever start behind an unverified firewall.
- **SC-003**: Editing runtime-only settings (mounts, env, network, secrets, limits, claude args)
  between runs never triggers an image rebuild; editing any bake-time input always does.
- **SC-004**: Every path listed in `secrets.hide` that exists on the host is unreadable from
  inside; no listed-but-missing path ever appears on the host afterwards.
- **SC-005**: With git disabled, an agent inside the sandbox cannot create a commit by any means;
  with git enabled, commits carry the user's identity.
- **SC-006**: `down`/`shell`/`prune` act on exactly the intended containers across many
  concurrent projects; `prune` never removes a running sandbox's image or volume.
- **SC-007**: Install script yields a working `kekkai` on both supported architectures.

## Assumptions

- Host prerequisites: Docker daemon, git, curl present; user can run docker without root
  escalation issues.
- Claude Code credentials live in the user's Claude config dir and are intentionally shared with
  the sandbox (accepted threat-model limit, §2).
- Validation is end-to-end against a real docker daemon (constitution Principle IV); no unit-test
  suite of consequence.
- Linux amd64/arm64 only; all §11 exclusions stand.
- The known threat-model limits of §2 (DNS side channel, allowed-destination exfiltration, bridge
  subnet reachability, rw Claude config dir, ssh-agent trust) are accepted and documented, not to
  be "fixed" by this feature.
