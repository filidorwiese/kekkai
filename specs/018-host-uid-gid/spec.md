# Feature Specification: Host UID/GID Match

**Feature Branch**: `018-host-uid-gid`

**Created**: 2026-07-10

**Status**: Draft

**Input**: User description: "The group id of the current user is not taken into account when spinning up kekkai. Host: uid=1000(fili) gid=1001(fili); inside sandbox: uid=1000(kekkai) gid=1000(kekkai). Sandbox user should match the host user's uid and gid so workspace files keep correct host ownership."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Workspace files keep host ownership (Priority: P1)

A developer whose host account has a primary group id different from their user id (e.g. uid 1000, gid 1001) runs a kekkai sandbox in a project. The agent inside the sandbox creates and modifies files in the workspace. After the session, every file the sandbox touched is owned by the developer's user **and** the developer's primary group on the host — no foreign group ids, no permission surprises in group-based workflows.

**Why this priority**: This is the reported defect. Files landing with a wrong group id can break group-shared repositories, backup tooling, and any process relying on group permissions. It silently corrupts ownership on every session.

**Independent Test**: On a Linux host where `id -u` ≠ `id -g`, start a sandbox, create a file in the workspace from inside, exit, and check `stat` output on the host shows the user's own uid and gid.

**Acceptance Scenarios**:

1. **Given** a host user with uid 1000 and gid 1001, **When** the sandbox creates a file in the workspace, **Then** the file on the host is owned by uid 1000 and gid 1001.
2. **Given** a host user with uid 1000 and gid 1000, **When** the sandbox creates a file in the workspace, **Then** the file on the host is owned by uid 1000 and gid 1000 (current behavior preserved).
3. **Given** a running sandbox, **When** the user runs `id` inside it, **Then** the reported uid and gid equal the host user's uid and gid.

---

### User Story 2 - Identity change is handled automatically (Priority: P2)

The same project directory is used by a different host account (or the user's uid/gid changes). The next sandbox start automatically produces an environment matching the new identity, without the user deleting images or running manual cleanup.

**Why this priority**: Without it, the first user's identity gets baked in forever and the fix only works for whoever built the image first.

**Independent Test**: Build a sandbox as one identity, then start kekkai as a user with a different uid/gid; verify the new sandbox reports the new identity.

**Acceptance Scenarios**:

1. **Given** an image built for uid/gid A, **When** a user with uid/gid B runs kekkai, **Then** kekkai produces and uses an environment matching B without manual intervention.
2. **Given** the offline fallback path (no network to rebuild), **When** kekkai selects a previously built image, **Then** it only selects one matching the current user's identity.

---

### User Story 3 - Unusual host identities do not break the sandbox (Priority: P3)

Users with atypical identities — running as root, having a low/system-range uid or gid, or a gid that collides with a group already present in the sandbox base system — can still start a sandbox, and all sandbox functions (firewall setup, developer tooling, command history) work unchanged.

**Why this priority**: Correctness guard. Rare setups must degrade safely rather than fail the build or weaken the sandbox.

**Independent Test**: Start kekkai as root and as a user whose gid matches a group that already exists in the base system; both sandboxes start and pass firewall verification.

**Acceptance Scenarios**:

1. **Given** kekkai invoked by root (uid 0), **Then** the sandbox user falls back to a safe non-root default identity and the sandbox starts normally.
2. **Given** a host gid that already exists as a group in the sandbox base system, **Then** the sandbox build still succeeds and the sandbox user's gid equals the host gid.
3. **Given** any supported identity, **Then** in-sandbox privileges are unchanged: only the firewall script is elevated, tooling works without sudo.

### Edge Cases

- Host gid collides with a group name/id already present in the base system (e.g. gid 100 `users`): build must not fail; sandbox user joins/reuses that gid.
- Host uid collides with an existing system account in the base image: build must not fail.
- Root user (uid 0) or system-range ids: fall back to the historical default (1000/1000) rather than baking a privileged or system identity.
- macOS: the container runtime's file sharing already maps ownership to the host user; the feature must not change or break macOS behavior.
- Existing users with uid 1000/gid 1000: behavior identical to today (a one-time image rebuild is acceptable).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: On Linux hosts, the sandbox user's numeric uid and gid MUST equal the invoking host user's uid and gid.
- **FR-002**: Files created or modified in the workspace from inside the sandbox MUST appear on the host as owned by the invoking user's uid and primary gid.
- **FR-003**: The host identity MUST be part of the sandbox environment's identity: a different host uid/gid MUST automatically yield a matching environment on next start, with no manual cleanup.
- **FR-004**: The offline fallback selection of previously built environments MUST take host identity into account, never reusing an environment baked for a different identity.
- **FR-005**: Builds MUST succeed when the host uid or gid collides with an account or group already present in the base system, still honoring the host's numeric ids.
- **FR-006**: When invoked as root or with an id outside the safe range, kekkai MUST fall back to the historical default identity (1000/1000) instead of failing or baking a privileged identity.
- **FR-007**: All existing sandbox guarantees MUST hold for every supported identity: firewall-only elevation, tooling usable without privileges, persistent command history, read-only config mount.
- **FR-008**: macOS behavior MUST remain unchanged; the feature MUST NOT introduce macOS-specific failures.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On a host where uid ≠ gid, 100% of files created by the sandbox in the workspace carry the user's own uid and gid on the host.
- **SC-002**: Users never need manual ownership repair (chown/chgrp) on workspace files after a sandbox session.
- **SC-003**: A host identity change requires zero manual steps: the next `kekkai up` produces a correct environment on its own.
- **SC-004**: Sandbox startup succeeds and firewall verification passes for all supported identities, including collision and root-fallback cases.
- **SC-005**: Users with the historical 1000/1000 identity observe no behavioral difference beyond at most one automatic rebuild.

## Assumptions

- Only the primary group matters; supplementary host groups are not mirrored into the sandbox.
- Baking the identity at environment build time (rather than adjusting at container start) is acceptable; consequence: one environment per identity, and a one-time rebuild when identity changes or when upgrading to this feature.
- Root and system-range ids (uid or gid < 1000, except the exact historical 1000 pair) fall back to 1000/1000; matching such identities is out of scope.
- On macOS the container runtime's file-sharing layer owns the uid/gid mapping; matching there is cosmetic and not required.
- Environments are built per host machine and are not shared between users, so per-identity environments add no distribution burden.
