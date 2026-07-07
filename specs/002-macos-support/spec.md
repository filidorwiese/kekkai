# Feature Specification: macOS Support

**Feature Branch**: `002-macos-support`

**Created**: 2026-07-07

**Status**: Draft

**Input**: User description: "I want to offer OSX support for this tool"

> **Governance note**: the macOS exclusion was removed from `SPECIFICATION.md` §11 on 2026-07-07 (this feature's clarification session). The constitution's Linux-only platform constraint still needs amending in the same change that delivers this feature (Principle I, spec-first).

## Clarifications

### Session 2026-07-07

- Q: Support model for macOS docker runtimes (Docker Desktop / OrbStack / colima / other)? → A: Capability probing gates functionality; runtime identity is detected only to enrich error messages with fix-hints, never to allow or block.
- Q: `git.ssh_agent` behavior on macOS? → A: Supported when the runtime exposes the host agent socket inside its VM; configured-but-absent is a preflight error with a runtime-specific hint. Never hard-error unconditionally, never degrade silently.
- Q: Mac-host reachability inside the sandbox? → A: Parity with Linux: Mac host is always-allowed builtin (like the Linux bridge subnet), not configurable. The nuance that this also reaches Mac services bound to localhost (broader than Linux) is documented, not mitigated.
- Q: Intel (x86_64) Mac support? → A: Apple silicon (arm64) only; installer refuses Intel Macs with a clear message. Revisit on demand.
- Q: Per-release validation matrix on macOS? → A: Docker Desktop is the only maintainer-validated runtime per release. OrbStack, colima and others are supported through capability probes and community-validated; documentation states this support tiering explicitly.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Mac user runs a sandboxed session (Priority: P1)

A developer on an Apple silicon Mac with their docker runtime of choice (Docker Desktop, OrbStack, colima, or any Docker-compatible runtime) wants to run Claude Code inside a kekkai sandbox for their project, with the same disk, network, and secrets guarantees Linux users get today.

**Why this priority**: This is the entire feature. Without a working `kekkai up` on macOS, nothing else has value.

**Independent Test**: On an Apple silicon Mac with any of the three major runtimes, run `kekkai init` then `kekkai up` in a project folder and land in a working Claude Code session inside the sandbox; verify confinement checks pass at startup.

**Acceptance Scenarios**:

1. **Given** an Apple silicon Mac with a running Docker-compatible runtime and a valid `.kekkai.yaml`, **When** the user runs `kekkai up`, **Then** the sandbox builds, all startup confinement verification passes, and Claude Code starts inside it.
2. **Given** a running sandbox on macOS, **When** the user runs `kekkai shell`, `kekkai ps`, `kekkai down`, or `kekkai prune`, **Then** each behaves identically to Linux.
3. **Given** a `.kekkai.yaml` allowing specific network destinations, **When** the sandbox is up on macOS, **Then** allowed destinations are reachable and everything else is blocked, same as on Linux.

---

### User Story 2 - Missing capabilities fail fast with runtime-specific guidance (Priority: P2)

A Mac user whose runtime lacks a capability kekkai needs for their configuration (agent socket not exposed, project folder not shared into the runtime's VM, unsupported architecture) is told exactly which capability is missing and, when the runtime is recognized, the runtime-specific fix, before any sandbox work starts.

**Why this priority**: the known failure modes (agent socket, unshared paths) fail confusingly deep inside Docker if not caught up front. Capability preflight with actionable hints protects both users and the security guarantees, on any runtime.

**Independent Test**: On macOS with a runtime that does not expose the capability (e.g. colima started without agent forwarding), enable `git.ssh_agent` in `.kekkai.yaml` and run `kekkai up`; it must refuse before starting the sandbox, naming the missing capability and the runtime-specific fix.

**Acceptance Scenarios**:

1. **Given** a macOS host whose runtime does not expose a needed capability for the user's `.kekkai.yaml`, **When** the user runs `kekkai up`, **Then** preflight stops before any sandbox work, names the missing capability, and includes a fix-hint specific to the detected runtime when recognized (generic guidance otherwise).
2. **Given** a macOS host running any Docker-compatible runtime, recognized or not, **When** all capability probes pass, **Then** `kekkai up` proceeds normally; runtime identity alone never blocks or degrades functionality.
3. **Given** a project folder not shared into the runtime's VM (e.g. outside Docker Desktop's file-sharing list or colima's mounts), **When** the user runs `kekkai up`, **Then** the failure message explains the folder must be shared and how, per detected runtime.

---

### User Story 3 - Mac user installs kekkai the standard way (Priority: P3)

A Mac user follows the same quick-install instructions as Linux users and gets a working binary on their PATH.

**Why this priority**: Install polish matters for adoption but a manually downloaded binary already unblocks Stories 1-2.

**Independent Test**: Run the documented quick-install command on an Apple silicon Mac; `kekkai version` then works from a new shell.

**Acceptance Scenarios**:

1. **Given** an Apple silicon Mac, **When** the user runs the documented quick-install command, **Then** the correct Mac binary is installed and `kekkai version` prints the current version.
2. **Given** the releases page, **When** a user looks for a download, **Then** a Mac (Apple silicon) binary with checksum is published alongside the Linux ones for every release.

---

### Edge Cases

- Intel (x86_64) Mac: out of scope; installer and binary must say so explicitly rather than install something broken.
- Docker daemon not running or not installed: same clear preflight failure Linux users get, with Mac-appropriate wording.
- Unrecognized runtime (e.g. Rancher Desktop, future runtimes): must work when capability probes pass; error messages fall back to generic guidance without runtime-specific hints.
- Mac services bound only to localhost are reachable through the builtin host allowance on macOS, unlike Linux where the bridge cannot reach loopback-bound host services; the confinement documentation must state this asymmetry explicitly.
- File I/O on shared folders is significantly slower than native Linux binds: documented as a known limitation, not treated as a defect.
- Windows: remains out of scope; nothing in this feature may imply otherwise.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Users on Apple silicon Macs using any Docker-compatible runtime that passes capability preflight MUST be able to run every existing kekkai command (`init`, `up`, `down`, `shell`, `ps`, `prune`, `version`) with behavior identical to Linux.
- **FR-002**: All confinement guarantees (disk exposure limited to configured folders, network limited to allowed destinations, secrets hidden) MUST hold on macOS exactly as specified for Linux, and the existing startup verification MUST run and pass, never be skipped or weakened.
- **FR-003**: SSH agent forwarding on macOS MUST work with Linux-equivalent behavior when the runtime exposes the host agent socket inside its VM. When forwarding is configured but the socket is absent, `kekkai up` MUST stop before any sandbox work, naming the missing capability and how to enable it on the detected runtime. It MUST never proceed silently without the agent.
- **FR-004**: Preflight on macOS MUST gate on probed capabilities only (project folder bindable into the runtime's VM, agent socket present when the config needs it, supported architecture) — never on runtime identity. Runtime identity MAY be detected solely to add a runtime-specific fix-hint to failure messages; unrecognized runtimes get generic guidance.
- **FR-005**: Every release MUST publish an Apple silicon Mac binary with checksum, and the quick-install method MUST select it automatically on supported Macs while refusing clearly on unsupported ones (e.g. Intel Macs).
- **FR-006**: User-facing documentation MUST list macOS (Apple silicon, any Docker-compatible runtime) as supported with explicit support tiering — Docker Desktop maintainer-validated per release; OrbStack, colima and others capability-probed and community-validated — and document the macOS-specific limitations: capability requirements per recognized runtime, slower file I/O on shared folders, and the broader localhost reachability described in FR-008.
- **FR-007**: `SPECIFICATION.md` (§11 macOS removal done 2026-07-07; remaining platform sections) and the constitution's platform constraint MUST be amended in the same change that delivers this feature, per the project's spec-first principle.
- **FR-008**: Sandboxes on macOS MUST reach the Mac host builtin, mirroring the Linux always-allowed bridge-subnet behavior; this is not user-configurable. Documentation MUST note that on macOS this includes Mac services bound only to localhost, which is broader than the Linux equivalent.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A Mac user with a running Docker-compatible runtime goes from the install command to a working sandboxed Claude Code session in under 10 minutes on first use.
- **SC-002**: 100% of existing kekkai commands produce the same user-visible outcome on macOS as on Linux for supported configurations.
- **SC-003**: Startup confinement verification passes on every `kekkai up` on macOS; zero releases ship with verification weakened or skipped for the platform.
- **SC-004**: 100% of the known blocking conditions (missing capability required by the config, Intel Mac, unshared project folder) fail before sandbox creation with a message that names the remedy; a recognized runtime adds the runtime-specific fix.
- **SC-005**: Support questions about macOS setup are answerable from the README alone: prerequisites, install, and every documented limitation.

## Assumptions

- Runtime support is capability-based, not identity-based: any Docker-compatible runtime whose probes pass is supported. Docker Desktop, OrbStack, and colima are the recognized runtimes for fix-hints; only Docker Desktop is maintainer-validated per release, the rest are community-validated. Apple silicon (arm64) only; Intel Macs and Windows stay out of scope.
- SSH agent forwarding relies on the runtime exposing the host agent inside its VM (native on Docker Desktop and OrbStack, opt-in on colima); maintainer verification happens on Docker Desktop, other runtimes are community-validated. Silent degradation is rejected: security-relevant config is never dropped quietly.
- Slower file I/O on shared folders is accepted and documented, not engineered around.
- End-to-end validation against a real Docker Desktop on Apple silicon hardware is available for release testing, consistent with the project's end-to-end validation principle.
- Constitution and `SPECIFICATION.md` amendments are in scope of this feature and land together with it.
