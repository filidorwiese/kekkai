# Feature Specification: Debian Base Image with Build-Time Node Install (nvm)

**Feature Branch**: `014-debian-nvm-base`

**Created**: 2026-07-09

**Status**: Draft

**Input**: User description: "Replace node:* base image with debian:trixie + build-time Node install via nvm — decouple the Debian release (owned by kekkai) from the Node version (owned by the user), installing Node at image build time from `image.node_version`."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Pick any Node version, sandbox just works (Priority: P1)

A kekkai user sets `image.node_version: 22` in `.kekkai.yaml` and runs `kekkai up`. The sandbox is built on kekkai's own pinned Debian release with Node 22.x installed and set as the default. `node`, `npm`, and `npx` work from every execution path into the container — interactive shells, `docker exec <ctr> node -v`, `sh -c 'node -v'`, and subprocesses spawned by Claude Code — without any profile sourcing tricks by the user.

**Why this priority**: This is the core value of the feature: the user's Node choice is no longer restricted to the tag matrix published by the node image maintainers, and kekkai controls exactly one tested Debian target. If Node is not reachable from non-interactive shells, the sandbox is broken for its primary consumer (Claude Code subprocesses).

**Independent Test**: Set `image.node_version: 22`, run `kekkai up`, then from the host run `docker exec <ctr> node -v` and `docker exec <ctr> sh -c 'node -v'`; both report a 22.x version.

**Acceptance Scenarios**:

1. **Given** `.kekkai.yaml` with `image.node_version: 22`, **When** `kekkai up` builds and starts the sandbox, **Then** `node -v` inside the container reports a 22.x version.
2. **Given** the running sandbox, **When** `docker exec <ctr> node -v` and `docker exec <ctr> sh -c 'node -v'` run (non-login, non-interactive), **Then** both succeed and report the same 22.x version.
3. **Given** `.kekkai.yaml` with a full pinned version (e.g. `image.node_version: "22.11.0"`), **When** the image builds, **Then** the build succeeds and exactly that version is the default node.
4. **Given** the sandbox, **When** the user (or Claude) runs `npm install -g <some-package>`, **Then** the install succeeds without sudo or permission errors and without root-owned files appearing under `/home/kekkai`.

---

### User Story 2 - Default and LTS users keep a zero-config experience (Priority: P2)

A user with no `image.node_version` key (or the explicit value `lts`) gets the latest Node LTS installed at build time. Claude Code is installed globally against that Node at the configured `claude.version`, and startup output shows both the resolved Node version and the resolved Claude Code version so the user can see exactly what the sandbox runs.

**Why this priority**: Most users never set a Node version; the default path must remain zero-config and transparent. Printing the resolved Node version closes the observability gap created by "lts" being a moving target.

**Independent Test**: Remove `image.node_version` from the config, run `kekkai up`, verify the build succeeds, `node -v` reports a current LTS version, and the startup output prints that resolved Node version alongside the Claude Code version.

**Acceptance Scenarios**:

1. **Given** a config with no `image.node_version` key, **When** `kekkai up` runs, **Then** the image builds with the latest LTS Node and startup output prints the resolved Node version and Claude Code version.
2. **Given** `image.node_version: lts`, **When** the image builds, **Then** the build succeeds identically to the omitted-key case.
3. **Given** any successful `kekkai up`, **When** the startup summary prints, **Then** it includes the resolved Node version (actual x.y.z, not the selector) next to the resolved Claude Code version.

---

### User Story 3 - Invalid or legacy config fails fast with a clear error (Priority: P3)

A user who supplies an unsupported Node selector (`node`, `stable`, `lts/*`, `lts/jod`) or still carries the removed `image.base_image` key gets a clear error at config-parse time — before any image build starts — naming the accepted forms or the replacement key.

**Why this priority**: Fail-fast validation protects users from long, confusing build failures and steers legacy configs to the new schema. It is a guardrail, not the core value.

**Independent Test**: Set `image.node_version: lts/*`, run `kekkai up`, and confirm the command exits with a validation error naming the accepted forms without any docker activity.

**Acceptance Scenarios**:

1. **Given** `image.node_version: lts/*` (or `node`, or `stable`), **When** `kekkai up` parses the config, **Then** it fails before any build with an error naming the accepted forms: `lts`, major (`22`), major.minor (`22.11`), full (`22.11.0`).
2. **Given** a config still containing `image.base_image`, **When** the config is parsed, **Then** the error names `image.node_version` as the replacement (migration hint).
3. **Given** an empty `image.node_version: ""`, **When** the config is parsed, **Then** the error tells the user to omit the key for the default `lts`.

---

### Edge Cases

- `image.node_version` is well-formed but does not exist (e.g. `99` or `22.99.0`): the image build fails; the error surfaced to the user must point at `image.node_version` as the knob to fix, not read as an internal build bug.
- Host network cannot reach the Node/installer download sources during build: the build fails with a network error; this must not be confused with (or routed through) the sandbox egress firewall, which does not exist yet at build time.
- Existing sandboxes built from the old node:* base: the base-image change alters bake-time inputs, so the next `kekkai up` triggers a rebuild; runtime config still never triggers rebuilds.
- `lts` is a moving target: the resolved LTS is fixed at image build time; a newer LTS release does not invalidate an already-built image (image hash derives from bake-time inputs only, and the selector string is unchanged).
- User adds `nodejs` or `npm` via `image.apt_packages`: Debian's packaged node could shadow or conflict with the build-time-installed Node; the build-time Node must remain the one resolved first on PATH.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The sandbox image MUST be built on a Debian release pinned by kekkai (`debian:trixie`). The base image MUST NOT be user-configurable; configs containing `image.base_image` MUST be rejected with an error naming `image.node_version` as the replacement.
- **FR-002**: The image build MUST install the minimum packages required on a bare Debian for the Node installer and Claude Code to function (`curl`, `ca-certificates`, `git`, `bash`, `procps`), plus the user's `image.apt_packages`.
- **FR-003**: Node MUST be installed at image build time via the nvm install script, pinned to a specific nvm release tag stored as a constant in the kekkai codebase — never `master` or `latest`.
- **FR-004**: The Node version installed MUST come from `image.node_version` and be set as the default. Accepted values, validated at config-parse time before any build: numeric major (`22`), major.minor (`22.11`), full (`22.11.0`), or the literal `lts` (translated to the installer's latest-LTS selector). All other values — including installer aliases like `node`, `stable`, `lts/<codename>` — MUST be rejected with an error naming the accepted forms. Default when the key is omitted: `lts`.
- **FR-005**: `node`, `npm`, and `npx` MUST work from non-login, non-interactive shells and every runtime exec path (`docker exec`, `sh -c`, Claude Code subprocesses). This MUST be guaranteed by symlinking the resolved node/npm/npx binaries into `/usr/local/bin` after install; the build itself uses the installer's documented non-interactive shell pattern (BASH_ENV) so build steps can invoke nvm.
- **FR-006**: `npm install -g @anthropic-ai/claude-code@<claude.version>` MUST work against the installed npm at build time, and the `claude` binary MUST be on PATH for the runtime user.
- **FR-007**: The container user remains the non-root `kekkai` user with home `/home/kekkai` and the same host-UID/GID mapping as today, so workspace files created inside the sandbox stay owned by the host user.
- **FR-008**: nvm and all Node artifacts MUST be installed as the `kekkai` user into its home (not as root), so `npm install -g` needs no elevated permissions and no root-owned files land in `/home/kekkai`.
- **FR-009**: All existing mount targets under `/home/kekkai/...` (git, ssh, claude mounts) MUST continue to work unchanged.
- **FR-010**: Build-time downloads (nvm installer, nvm repo, Node binaries, npm registry) run on the host network before the sandbox firewall exists. Code comments MUST document this so the build is never routed through the egress rules, and these domains MUST NOT be required in the user's `network.allowed_domains`.
- **FR-011**: The `.kekkai.yaml` example (`kekkai init` output) and README MUST describe the `image` section as: `node_version` — `"lts"` (default) or a version number like `22` / `22.11.0` — replacing any mention of a configurable node:* base image.
- **FR-012**: Startup output MUST print the resolved Node version (the actual installed version) alongside the resolved Claude Code version.

### Key Entities

- **`image.node_version` (config key)**: the user's Node selector; one of `lts` | major | major.minor | full version. Bake-time input: changing it changes the image hash and triggers a rebuild.
- **Pinned platform constants (code)**: the Debian base (`debian:trixie`) and the nvm release tag — owned by kekkai, changed only via code releases, both bake-time inputs.
- **Sandbox image**: built from the Debian base + baseline apt packages + user apt packages + nvm-installed Node + globally installed Claude Code; identified by a hash of bake-time inputs only.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With `image.node_version: 22`, `node -v` reports a 22.x version from 100% of execution paths tested: interactive shell, `docker exec <ctr> node -v`, and `docker exec <ctr> sh -c 'node -v'`.
- **SC-002**: Builds succeed for all three selector shapes: `lts`, a major version, and a full pinned version.
- **SC-003**: Every invalid selector (`node`, `stable`, `lts/*`, `lts/<codename>`) and any config containing `image.base_image` fails at config-parse time — zero docker activity — with an error naming the accepted forms or replacement key.
- **SC-004**: `npm install -g` inside the sandbox succeeds without sudo or permission errors, and files created in the workspace from inside the sandbox are owned by the host user.
- **SC-005**: A user upgrading from the previous node:*-based image needs zero config changes when their config only uses supported keys; existing git/claude/ssh mounts function unchanged.
- **SC-006**: No new entries in `network.allowed_domains` are needed for the image build; a config listing only the user's own domains builds successfully.
- **SC-007**: Startup output shows the concrete installed Node version (x.y.z) alongside the Claude Code version on every successful `kekkai up`.

## Assumptions

- `image.node_version` and the `lts` default already exist (specs/004); this feature changes the mechanism behind the key (build-time install on a Debian base instead of node:* tag selection) and tightens the accepted value set.
- `image.base_image` is already rejected via the legacy-key migration map; that behavior is preserved and the message keeps naming `image.node_version`.
- The current Docker Hub tag-existence probe for node:* images becomes obsolete; version-existence failures surface at build time instead (well-formed-but-nonexistent versions cannot be caught at parse time).
- The image hash principle from the constitution holds: only bake-time inputs (now including the nvm tag constant and the Debian base) affect the hash; `lts` resolution is frozen at build time and does not retroactively invalidate images.
- Reading the workspace `.nvmrc`, multiple Node versions per sandbox, and non-Debian bases are explicitly out of scope.
- `SPECIFICATION.md` (source of truth per constitution) is updated together with the implementation; README changes in FR-011 are the user-facing digest.
