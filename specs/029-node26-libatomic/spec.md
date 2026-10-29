# Feature Specification: Node 26 LTS runs in the sandbox image

**Feature Branch**: `029-node26-libatomic`

**Created**: 2026-10-29

**Status**: Draft

**Input**: User description: "Node 26 LTS fails to run in the sandbox image because the debian:trixie base lacks libatomic1; add it to the builtin apt packages so `image.node_version: lts` (and any Node >= 26 selector) builds again."

## Background

Since late October 2026 the Node.js project designates the 26 line as the active LTS. Its Linux binaries depend on a shared library (`libatomic`) that the minimal Debian base image does not ship. Inside the image build the Node installer downloads and unpacks v26 successfully, but the binary cannot start. The installer's version lookup therefore reports `none`, the default alias and the stable `current` link are never created, and the following build step fails with `npm: command not found`, surfacing as the claude-install retry message. Every user on the default `lts` selector is affected; pinned selectors below 26 still work.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Default configuration builds again (Priority: P1)

A developer with no `.kekkai.yaml`, or one that leaves `image.node_version` at its default, runs `kekkai up`. The image builds, the startup line reports the current Node LTS (26.x) and the pinned Claude version, and Claude starts.

**Why this priority**: The default path is broken for every new install and for every user whose cached image is invalidated by a Claude release. Nothing else in kekkai matters while `kekkai up` cannot produce an image.

**Independent Test**: Remove all `kekkai:*` images, run `kekkai up` in a project without a config file, observe a successful build and the sandbox startup line naming a 26.x Node version.

**Acceptance Scenarios**:

1. **Given** no cached image and the default Node selector, **When** `kekkai up` runs, **Then** the build completes and the startup line reports Node 26.x and the resolved Claude version.
2. **Given** the build log, **When** the Node install step finishes, **Then** it reports the installed version as current and emits no warning about a nonexistent version.
3. **Given** the running sandbox, **When** `kekkai exec node -v` and `kekkai exec npm -v` run, **Then** both print a version.

---

### User Story 2 - Explicit Node 26 selectors build (Priority: P1)

A developer pins `image.node_version` to `26`, `26.13` or `26.13.0`. The image builds and Node runs, with the same outcome as the default selector.

**Why this priority**: Same defect, different entry point. Users who pinned ahead of the LTS switch hit it first.

**Independent Test**: Set `image.node_version: 26`, run `kekkai up`, observe a successful build and a 26.x startup line.

**Acceptance Scenarios**:

1. **Given** `image.node_version: 26`, **When** `kekkai up` runs, **Then** the build completes and the sandbox starts.
2. **Given** `image.node_version: 26.13.0`, **When** `kekkai up` runs, **Then** the build completes and the startup line reports exactly v26.13.0.

---

### User Story 3 - Earlier Node selectors keep working (Priority: P2)

A developer pinned to `22` or `24` upgrades kekkai. Their image rebuilds once and behaves as before.

**Why this priority**: The fix changes the baked package set, which by design invalidates existing images. The rebuild must be a one-time cost with no behavior change for versions that already worked.

**Independent Test**: Set `image.node_version: 24`, run `kekkai up` with the upgraded binary, observe one rebuild and a 24.x startup line; run `kekkai up` again and observe no rebuild.

**Acceptance Scenarios**:

1. **Given** `image.node_version: 24` and an image built by the previous kekkai release, **When** the upgraded `kekkai up` runs, **Then** exactly one rebuild happens and the sandbox starts on Node 24.x.
2. **Given** that rebuilt image, **When** `kekkai up` runs again, **Then** no rebuild happens.

---

### Edge Cases

- A future Node line adds yet another runtime library: out of scope. The same mechanism (extend the builtin package set) applies; nothing here should make that harder.
- The library is already present because a user listed it in `image.apt_packages`: the package manager treats the duplicate as a no-op; the build MUST still succeed.
- The offline fallback (registry unreachable, reuse newest image matching the config hash): images built before this change carry a different config hash and MUST NOT be reused, since they may hold a broken Node install.
- A broken image from a failed build: failed builds tag no image, so the next `kekkai up` after upgrading rebuilds cleanly with no manual cleanup.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: An image built with `image.node_version: lts` MUST produce a working Node, npm and Claude installation while the current LTS is the 26 line.
- **FR-002**: An image built with any explicit Node 26 selector (`26`, `26.x`, `26.x.y`) MUST likewise produce a working Node, npm and Claude installation.
- **FR-003**: Images built with Node selectors that worked before this change (22 and 24 lines) MUST continue to work.
- **FR-004**: The runtime library MUST be part of the builtin package set baked into every image, never user configuration: the image must be identical regardless of runtime config, and the default selector must work without a config file.
- **FR-005**: The change MUST invalidate images built before it (via the bake-time inputs), so no user is left on a cached image with a broken Node install, including via the offline fallback.
- **FR-006**: The builtin package addition MUST NOT open any new network destination at sandbox runtime; install-time downloads continue to run on the daemon's host network as today.
- **FR-007**: `SPECIFICATION.md` §5.1 (builtin apt packages) MUST be updated in the same change per the spec-first principle, naming the package and why it is required.

### Key Entities

- **Builtin package set**: the fixed list of system packages baked into every sandbox image, independent of user configuration; part of the image hash and config hash.
- **Node selector**: the user-facing `image.node_version` value (`lts` or a numeric pin) resolved by the installer at build time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `kekkai up` with no config file and no cached image completes the build and starts Claude on Node 26.x, 100% of attempts on linux/amd64, linux/arm64 and Docker Desktop for macOS.
- **SC-002**: Builds with selectors `lts`, `26`, `24` and `22` all succeed with the upgraded kekkai.
- **SC-003**: Upgrading kekkai triggers exactly one rebuild per configuration; the second `kekkai up` reuses the image.
- **SC-004**: Zero new entries in the sandbox egress allowlist; `kekkai traffic` shows no new destinations during a session.

## Assumptions

- Root cause confirmed by reproduction in a plain `debian:trixie` container: `nvm install --lts` fetches v26.13.0, `node --version` fails with `libatomic.so.1: cannot open shared object file`, `nvm current` prints `none`; installing the Debian package `libatomic1` makes the same sequence succeed. Node 24 succeeds without it.
- The fix is one package in the builtin apt set. It depends only on the C library and gcc base, adds negligible image size, and is a Debian main package reachable from the already-configured mirrors.
- The config-hash change (and hence one-time rebuild for everyone) is accepted; it is the mechanism that guarantees FR-005 and needs no extra migration logic.
- The host-side pre-build existence check and the installer's own error paths are unchanged; this is purely a runtime-library gap in the base image.
- No unit tests; validation is end-to-end per the constitution: build with each selector in SC-002 and confirm the startup line and `kekkai exec node -v`.
