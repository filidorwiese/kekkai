# Feature Specification: OpenSSH Client in Sandbox Image

**Feature Branch**: `019-openssh-client`

**Created**: 2026-08-18

**Status**: Draft

**Input**: User description: "To allow for ssh git signing, the package `openssh-client` should always be added to the docker image when `ssh_agent: true` (or just add it always if that makes more sense in the code base)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - SSH commit signing inside the sandbox (Priority: P1)

A user has git configured on the host to sign commits with an SSH key (`gpg.format ssh`) and runs kekkai with `git.ssh_agent: true`. Inside the sandbox, the agent commits work. Signing must succeed: git delegates to the SSH tooling, which talks to the forwarded agent socket. Today the sandbox image lacks the SSH client tooling, so every commit fails with a "ssh-keygen not found" style error even though the agent socket and signing config are correctly mounted.

**Why this priority**: This is the reported defect. `ssh_agent: true` already mounts the agent socket and `allowed_signers` precisely to support signing, but the missing client package makes that whole path unusable.

**Independent Test**: On a host with SSH signing configured and an agent holding the signing key, run `kekkai up` in a project with `git.ssh_agent: true`, then create a commit inside the sandbox. The commit is created and carries a valid SSH signature verifiable against `allowed_signers`.

**Acceptance Scenarios**:

1. **Given** host git config uses SSH signing and `git.ssh_agent: true`, **When** a commit is made inside the sandbox, **Then** the commit succeeds and is SSH-signed.
2. **Given** the same setup and `~/.config/git/allowed_signers` present on the host, **When** the signed commit is verified inside the sandbox, **Then** verification reports a good signature.

---

### User Story 2 - SSH-based git remotes (Priority: P2)

A user with `git.ssh_agent: true` and an allowed git host (e.g. `allow_github: true`) fetches or pushes over an `ssh://`/`git@` remote from inside the sandbox. The SSH client is present, so git can open the connection and authenticate via the forwarded agent.

**Why this priority**: Same missing package, second consumer. The existing spec already promises "`git.ssh_agent: true` lets the agent authenticate as the user against any allowed host", which is impossible without an SSH client binary.

**Independent Test**: With `ssh_agent: true` and github allowed, run `git fetch` against a `git@github.com:` remote inside the sandbox; it completes using agent authentication.

**Acceptance Scenarios**:

1. **Given** `git.ssh_agent: true` and the remote's host allowed by the firewall, **When** git talks to an SSH remote inside the sandbox, **Then** the connection authenticates via the forwarded agent and the operation succeeds.

---

### Edge Cases

- `ssh_agent: false`: the package is still present in the image (baked unconditionally), but no agent socket exists; SSH operations that need a key simply fail with a normal authentication error, as on any machine without an agent. No behavior change otherwise.
- User already lists `openssh-client` in `image.apt_packages`: the package manager treats the duplicate as a no-op; the build must not fail. Users may drop it from their config at leisure.
- Existing cached images built before this change: the builtin package list is a bake-time input, so the image hash changes and a rebuild happens naturally on next `up`; no manual invalidation needed.
- Toggling `ssh_agent` in config: must never trigger a rebuild — the package is present regardless of the toggle.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The sandbox image MUST always include the SSH client tooling (`openssh-client`) as part of the builtin package set, regardless of any runtime configuration (including `git.ssh_agent`).
- **FR-002**: With `git.ssh_agent: true` and host git configured for SSH signing, commits created inside the sandbox MUST be signed successfully using the forwarded agent.
- **FR-003**: With `git.ssh_agent: true`, git operations against SSH remotes on allowed hosts MUST be able to authenticate via the forwarded agent.
- **FR-004**: Image identity MUST continue to derive from bake-time inputs only: adding the package changes the builtin set (and therefore the image hash) once, but flipping `git.ssh_agent` MUST NOT change the image or trigger a rebuild.
- **FR-005**: A user-supplied duplicate `openssh-client` entry in `image.apt_packages` MUST NOT break the image build.
- **FR-006**: The project specification's builtin package list (SPECIFICATION.md §5.1) MUST be amended to include the package, in the same change as the code (constitution: Spec-First).

### Assumptions

- "Add it always" is the correct interpretation of the user's parenthetical: the codebase explicitly bakes packages unconditionally so the image is identical regardless of runtime config (existing precedent: `jq`/`aggregate` are baked though only the `allow_github` path uses them), and the constitution forbids runtime config influencing the image hash. A conditional install keyed on `ssh_agent` would violate both.
- `openssh-client` is the only package needed: it provides both the signing helper (`ssh-keygen`) and the transport client (`ssh`). No server component is ever needed in the sandbox.
- Package size (~5 MB installed) is an acceptable cost for all users, in line with the existing "convenience" entries (`gh`, `nano`, `less`).

## Success Criteria *(mandatory)*

- **SC-001**: A user with SSH commit signing configured on the host can create signed commits inside the sandbox on the first attempt, with zero extra configuration beyond `git.ssh_agent: true`.
- **SC-002**: SSH signature verification of those commits succeeds both inside the sandbox and on the host.
- **SC-003**: Changing `git.ssh_agent` between runs never causes an image rebuild; the first run after upgrading rebuilds exactly once due to the changed builtin package set.
- **SC-004**: Users who previously worked around the gap via `image.apt_packages: [openssh-client]` see no build failure and identical behavior after removing the workaround.
