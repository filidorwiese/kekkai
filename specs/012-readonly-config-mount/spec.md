# Feature Specification: Read-Only Config Mount

**Feature Branch**: `012-readonly-config-mount`

**Created**: 2026-07-09

**Status**: Draft

**Input**: User description: ".kekkai.yaml should be read-only-mounted inside the container - so that claude code can read it. If no .kekkai.yaml is present in the workspace, shadow mount an empty file. On start (`kekkai up`) change the possibility that a user can have a typo file named .kekkai.yml (no `a` in yaml) - warn about the typo and exit if such a file exists. From now on only .kekkai.yaml is allowed and read for configuration."

## Clarifications

### Session 2026-07-09

- Q: What should the comment-only placeholder file contain when no `.kekkai.yaml` exists? → A: `# no .kekkai.yaml in workspace - kekkai runs on defaults; create one on the host ('kekkai init') to customize`

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Config is readable but immutable inside the sandbox (Priority: P1)

As a kekkai user, when I run `kekkai up`, the agent inside the sandbox can read `.kekkai.yaml` (so it can explain the active restrictions and suggest concrete config changes when something is blocked), but it cannot modify the file. Today the workspace mount exposes the config writable, meaning the agent could silently loosen its own restrictions (add network destinations, unhide secrets) that would take effect on the next `kekkai up`. This closes that self-escalation path.

**Why this priority**: Security boundary integrity (Constitution Principle II). A sandboxed agent that can edit its own sandbox policy defeats the purpose of the sandbox.

**Independent Test**: Start a sandbox in a project with a `.kekkai.yaml`; from inside the container, read the file (succeeds, content matches host) and attempt to write/delete/rename it (all fail).

**Acceptance Scenarios**:

1. **Given** a workspace with a `.kekkai.yaml`, **When** the sandbox starts, **Then** the file is visible at its usual workspace location inside the container with identical content to the host file.
2. **Given** a running sandbox, **When** the agent attempts to write to, truncate, delete, or replace `.kekkai.yaml`, **Then** the operation fails and the host file is unchanged.
3. **Given** a running sandbox, **When** the agent reads `.kekkai.yaml`, **Then** it sees the configuration that was used to start the sandbox.

---

### User Story 2 - No config file: comment-only placeholder blocks creation (Priority: P2)

As a kekkai user running without a `.kekkai.yaml` (defaults mode), the sandbox exposes a read-only placeholder at the config location instead, containing a single brief comment that explains the situation: `# no .kekkai.yaml in workspace - kekkai runs on defaults; create one on the host ('kekkai init') to customize`. The agent sees a comments-only config (consistent with "comments-only file means all defaults") that tells it defaults are active and that configuration happens on the host; it cannot create a real `.kekkai.yaml` on the host from inside the sandbox.

**Why this priority**: Same self-escalation path as Story 1, via file creation instead of modification. Without the placeholder, the agent could plant a config that the user unknowingly activates on the next `kekkai up`.

**Independent Test**: Start a sandbox in a project without a `.kekkai.yaml`; from inside the container, verify the config path reads as exactly the placeholder comment and cannot be written; verify no `.kekkai.yaml` appears on the host afterwards.

**Acceptance Scenarios**:

1. **Given** a workspace without `.kekkai.yaml`, **When** the sandbox starts, **Then** reading the config path inside the container yields exactly the placeholder comment (no other content).
2. **Given** that placeholder, **When** the agent attempts to write to it or replace it, **Then** the operation fails and no `.kekkai.yaml` is created on the host.
3. **Given** a workspace without `.kekkai.yaml`, **When** the sandbox starts and later exits, **Then** the host workspace still contains no `.kekkai.yaml` and no other artifact created by the placeholder mechanism.

---

### User Story 3 - Typo filename detected: warn and exit (Priority: P2)

As a kekkai user who accidentally named the config `.kekkai.yml` (missing the `a`), `kekkai up` refuses to start: it prints a clear warning identifying the typo and telling me to rename the file to `.kekkai.yaml`, then exits with a failure status. Previously `.kekkai.yml` was silently accepted as a valid config name; from now on only `.kekkai.yaml` is recognized, and treating the typo as "no config" would silently drop all my restrictions - so the safe behavior is to stop.

**Why this priority**: Silent fallback to defaults would disable user-configured protections (hidden secrets, network restrictions) without the user noticing - a security regression, not just a UX issue.

**Independent Test**: Create `.kekkai.yml` in a project, run `kekkai up`, verify the typo message and non-zero exit before any sandbox work begins.

**Acceptance Scenarios**:

1. **Given** a workspace with only `.kekkai.yml`, **When** the user runs `kekkai up`, **Then** kekkai prints a warning naming `.kekkai.yml`, states that only `.kekkai.yaml` is read, suggests renaming, and exits non-zero without starting a sandbox.
2. **Given** a workspace with both `.kekkai.yml` and `.kekkai.yaml`, **When** the user runs `kekkai up`, **Then** kekkai warns about the `.kekkai.yml` typo file and exits non-zero (no ambiguity about which file wins - none does until the typo file is removed).
3. **Given** a workspace with only `.kekkai.yaml`, **When** the user runs `kekkai up`, **Then** startup proceeds normally with no typo warning.

---

### Edge Cases

- Both `.kekkai.yml` and `.kekkai.yaml` present: typo check wins - warn and exit (Story 3, scenario 2). The previous "both exist, remove one" error is replaced by the typo error.
- `.kekkai.yml` exists but is a directory or other non-regular file: still treated as the typo case - warn and exit (any entry with that name indicates user confusion).
- `.kekkai.yaml` exists but is not a regular file (directory, dangling symlink): error and exit - the read-only mount needs a regular file, and treating it as "no config" would silently drop intended configuration.
- Host `.kekkai.yaml` edited while the sandbox is running: the sandbox was configured from the content read at startup; the file view inside the container is not required to track live host edits. Restart applies changes, as today.
- `kekkai init` in a workspace containing `.kekkai.yml`: init refuses (as it does today for an existing config) so the user resolves the typo first rather than ending up with two files.
- Agent creates a different file (e.g. `kekkai.yaml`, `.kekkai.yaml.new`) in the workspace: out of scope - only the canonical config path is protected; other files never influence kekkai configuration.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Only the file named `.kekkai.yaml` at the workspace root is recognized and read as configuration; `.kekkai.yml` is no longer accepted as a config file by any kekkai command.
- **FR-002**: `kekkai up` MUST detect a `.kekkai.yml` entry at the workspace root before any sandbox work begins, print a warning that identifies the typo and instructs renaming to `.kekkai.yaml`, and exit with a non-zero status.
- **FR-003**: The typo check MUST fire even when a valid `.kekkai.yaml` also exists (both-present case), and MUST NOT fire when only `.kekkai.yaml` or no config file exists.
- **FR-004**: When `.kekkai.yaml` exists, the sandbox MUST expose it read-only at its workspace-root location inside the container, with content identical to the host file at startup.
- **FR-005**: When `.kekkai.yaml` does not exist, the sandbox MUST expose a read-only placeholder at the same location containing exactly one comment line: `# no .kekkai.yaml in workspace - kekkai runs on defaults; create one on the host ('kekkai init') to customize`. After the sandbox exits, the workspace MUST contain no `.kekkai.yaml` (the empty mount-point file the container runtime materializes during the run is removed at exit, unless the user replaced it with real content meanwhile) and no persistent placeholder artifact may remain elsewhere on the host.
- **FR-006**: All write operations against the in-container config path (write, truncate, delete, rename, replace) MUST fail, and MUST leave the host file (or its absence) unchanged.
- **FR-007**: Existing no-config behavior is preserved: `kekkai up` without `.kekkai.yaml` still runs on defaults with the existing one-line warning; the placeholder does not change config semantics (comments-only file = all defaults).
- **FR-008**: `kekkai init` MUST refuse to write a starter config while a `.kekkai.yml` entry exists, directing the user to resolve the typo file.

### Key Entities

- **Config file (`.kekkai.yaml`)**: the single recognized sandbox policy file at the workspace root; readable inside the sandbox, writable only from the host.
- **Typo file (`.kekkai.yml`)**: any directory entry with this name at the workspace root; never read, always a startup-blocking error for `up`.
- **Config placeholder**: read-only stand-in exposed inside the sandbox when no config file exists; contains a single explanatory comment line and has no host-side footprint.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: From inside a running sandbox, the agent can read the exact active configuration in 100% of startups (real file, or the comment-only placeholder in defaults mode), with zero cases where the config path is missing.
- **SC-002**: Zero host-side configuration changes can originate from inside the sandbox: after any in-container write/create/delete attempt against the config path, the host `.kekkai.yaml` state (content or absence) is byte-for-byte unchanged.
- **SC-003**: 100% of `kekkai up` runs in a workspace containing `.kekkai.yml` stop before any sandbox resources are created, with a message that names the offending file and the required rename.
- **SC-004**: A user who hits the typo error can resolve it with a single rename and immediately start successfully, with no other state to clean up.

## Assumptions

- The workspace root is the only location ever consulted for configuration (matches existing single-file, no-layering design).
- "Warn and exit" for the typo file applies to `kekkai up`; `kekkai init` handles the typo by refusing to write (FR-008); other commands simply never read `.kekkai.yml` (FR-001).
- The read-only view inside the container reflects the file content at sandbox startup; live propagation of host edits into a running container is not required (config is only applied at startup anyway).
- The existing "both files present" error message is superseded by the typo warning; no deprecation period or migration tooling is needed beyond the actionable rename instruction.
- Documentation (SPECIFICATION.md §4/§5, README) will be updated alongside the change per Constitution Principle I; the typo-file rejection and read-only mount become part of the documented contract.
