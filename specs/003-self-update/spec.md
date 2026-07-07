# Feature Specification: Self-Update Command

**Feature Branch**: `003-self-update`

**Created**: 2026-07-07

**Status**: Draft

**Input**: User description: "I want to add `kekkai self-update` command to auto-update the local install to the latest release that can be found on https://github.com/filidorwiese/kekkai/. If update show from/to version after updating, if no update is available say something like \"You're on the latest version\""

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Update to the latest release (Priority: P1)

A kekkai user hears a new version is out. They run `kekkai self-update` and the tool fetches the latest published release, replaces the installed binary, and confirms the version change, without the user hunting for the installer command or download page.

**Why this priority**: The entire point of the feature; removes the re-run-the-installer friction that keeps users on stale versions.

**Independent Test**: Install an older release, run `kekkai self-update`, verify the binary now reports the latest version and the output names both versions.

**Acceptance Scenarios**:

1. **Given** an installed kekkai older than the latest published release, **When** the user runs `kekkai self-update`, **Then** the binary is replaced with the latest release for the user's platform and the output shows the old and new version (e.g., "Updated kekkai v0.3.0 -> v0.4.0").
2. **Given** the update succeeded, **When** the user runs `kekkai version` in a new shell, **Then** it prints the new version.
3. **Given** an update in progress, **When** the download or verification fails partway, **Then** the previously installed binary remains in place and functional.

---

### User Story 2 - Already up to date (Priority: P2)

A user runs `kekkai self-update` when no newer release exists. The tool tells them they are current and changes nothing.

**Why this priority**: The most frequent outcome in practice; must be fast, reassuring, and side-effect free.

**Independent Test**: Run `kekkai self-update` on a binary matching the latest release tag; verify the "latest version" message and that the binary file is untouched.

**Acceptance Scenarios**:

1. **Given** the installed version equals the latest published release, **When** the user runs `kekkai self-update`, **Then** the output says they are on the latest version (naming it, e.g., "You're on the latest version (v0.4.0)") and exits successfully without downloading or modifying anything.

---

### User Story 3 - Clear failure when updating is not possible (Priority: P3)

A user runs `kekkai self-update` in a situation where updating cannot or should not proceed (development build, no write permission on the binary, network or verification failure). The tool explains exactly why and what to do instead.

**Why this priority**: Protects trust in the command; a broken or silent failure here can brick the user's install.

**Independent Test**: Run the command as a dev build, against an unwritable binary path, and with the release service unreachable; each produces a distinct, actionable error and leaves the install untouched.

**Acceptance Scenarios**:

1. **Given** a development (unversioned) build, **When** the user runs `kekkai self-update`, **Then** it refuses with a message explaining dev builds cannot self-update and pointing at the standard install methods.
2. **Given** the installed binary lives in a location the user cannot write to, **When** they run `kekkai self-update`, **Then** it fails before downloading anything, naming the path and suggesting how to proceed.
3. **Given** the release service is unreachable or the downloaded artifact fails integrity verification, **When** the user runs `kekkai self-update`, **Then** it aborts with the reason and the existing binary keeps working.

### Edge Cases

- Installed version is newer than the latest published release (e.g., built from an unreleased tag): treated as up to date; report the installed version alongside the latest release.
- Interrupted mid-replacement (Ctrl-C, power loss): replacement is atomic; the binary is either the old or the new version, never truncated.
- The binary is currently being replaced while the same binary is executing: replacement must not corrupt the running process.
- The latest release has no artifact for the user's platform: abort with a clear message; nothing modified.
- Rate limiting or transient errors from the release service: reported as a retryable failure, not a crash.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The tool MUST provide a `self-update` subcommand that determines the latest published release of kekkai from the project's public release channel (https://github.com/filidorwiese/kekkai/).
- **FR-002**: When the installed version is equal to (or newer than) the latest release, the command MUST report that the user is on the latest version, naming it, and exit successfully without downloading or modifying anything.
- **FR-003**: When a newer release exists, the command MUST download the artifact matching the running platform, verify its published integrity checksum, and replace the currently installed executable in place.
- **FR-004**: Replacement MUST be atomic: at no point may a failed or interrupted update leave the installed binary missing, truncated, or non-functional.
- **FR-005**: After a successful update, the command MUST report both the previous and the new version.
- **FR-006**: Integrity verification failure MUST abort the update before the installed binary is touched.
- **FR-007**: A development or unversioned build MUST refuse to self-update with an explanatory message referencing the standard install methods.
- **FR-008**: When the installed binary's location is not writable by the user, the command MUST fail with a message naming the path, before any download starts.
- **FR-009**: All failure modes (network, missing platform artifact, verification, permissions) MUST produce a distinct, actionable error message and a non-zero exit status, and MUST leave the existing installation fully functional.
- **FR-010**: The command MUST require no configuration and take no mandatory arguments; it operates on the binary that is currently executing.

### Key Entities

- **Release**: A published version of kekkai on the public release channel; has a version tag, per-platform artifacts, and published integrity checksums.
- **Installed binary**: The executable currently running the command; has a version (or "dev") and a filesystem location whose writability gates the update.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user on an older release gets to the latest version with a single command in under 60 seconds on a typical connection.
- **SC-002**: 100% of failure modes (network down, checksum mismatch, unwritable path, dev build, missing platform artifact) leave the existing installation working and explain the cause in one message.
- **SC-003**: The up-to-date case completes in under 5 seconds and makes zero changes to the filesystem.
- **SC-004**: A user can tell from the command output alone which of the three outcomes occurred: updated (with from/to versions), already current (with version), or failed (with reason).

## Assumptions

- Releases continue to be published on the project's GitHub releases page with per-platform tarballs and a checksum manifest, exactly as the existing installer consumes them (same artifact naming and platform coverage, including darwin/arm64 from feature 002).
- "Latest" means the newest published release; the command only moves forward, no downgrade or version pinning in scope.
- The check runs on the host, outside the sandbox; host network access to github.com is a given (it is already an install prerequisite).
- Updating the binary does not require touching existing project sandboxes or images; they continue to work with the new binary per the existing image-hash rules.
- No signature scheme exists for release binaries today; checksum verification against the published manifest is the accepted integrity bar (same as install.sh).
- Elevated-permission installs (e.g., binary owned by root in /usr/local/bin) are out of scope for automatic handling; the command fails with guidance rather than escalating privileges.
