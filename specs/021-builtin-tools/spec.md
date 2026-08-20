# Feature Specification: Builtin Archive, File-Inspection, and Search Tools

**Feature Branch**: `021-builtin-tools`

**Created**: 2026-08-20

**Status**: Draft

**Input**: User description: "Extend builtin apt packages with archive, file-inspection, and search tools — add unzip, zip, xz-utils, zstd, bzip2, file, ripgrep, fd-find, rsync to kekkai's builtin apt package set, baked into every sandbox image alongside the existing builtins."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent works with common archive and file formats out of the box (Priority: P1)

An autonomous agent inside a fresh sandbox downloads a release asset (`.zip`), a toolchain tarball (`.tar.xz`, `.tar.zst`, `.tar.bz2`), or an unknown file. It extracts, creates, and inspects these with the standard tools (`unzip`, `zip`, `xz`, `zstd`, `bzip2`, `file`) and moves file trees around with `rsync` — without the project author having configured anything.

**Why this priority**: This is the core gap. The base sandbox ships only `tar` and `gzip`; today each of these operations fails mid-task in a confusing way and users must discover the missing package and add it per-project via `image.apt_packages`.

**Independent Test**: Start a sandbox from a project with no config file and run archive round-trips (`.zip`, `.tar.xz`, `.tar.zst`), `file` on a binary, and an `rsync` copy inside it.

**Acceptance Scenarios**:

1. **Given** a fresh sandbox with no project config, **When** the agent runs `unzip`, `zip`, `xz`, `zstd`, `bzip2`, `file`, or `rsync`, **Then** each command resolves and runs.
2. **Given** a fresh sandbox, **When** the agent creates and extracts a `.zip`, a `.tar.xz`, and a `.tar.zst` archive, **Then** each round-trip succeeds with content intact.
3. **Given** a fresh sandbox, **When** the agent runs `file` on an arbitrary file, **Then** it reports the file type.

---

### User Story 2 - Agent searches the workspace with standard fast-search tools (Priority: P2)

An agent (or a user in `kekkai shell`) searches the mounted workspace with `rg` and finds files with `fd` — the canonical command names these tools are known by — and gets fast results without any per-project setup.

**Why this priority**: Code search is the most frequent agent operation after file reads; the tools are near-universal in agent-written shell commands. Lower than P1 only because slower fallbacks (`grep`, `find`) exist in the base image.

**Independent Test**: In a fresh sandbox, run `rg <pattern>` and `fd <name>` against the workspace and verify both return matches.

**Acceptance Scenarios**:

1. **Given** a fresh sandbox, **When** the agent runs `rg` with a pattern present in the workspace, **Then** matches are returned.
2. **Given** a fresh sandbox, **When** the agent runs `fd` with a filename fragment, **Then** matching paths are returned under the canonical command name `fd` (not only a distribution-renamed variant).

---

### User Story 3 - Existing projects upgrade cleanly (Priority: P2)

A user upgrades kekkai. On the next run, each of their projects rebuilds its sandbox image once (the builtin package set is part of the image definition); after that one rebuild, unchanged configs reuse the image exactly as before. Projects that already listed one of the new tools in `image.apt_packages` keep working without edits.

**Why this priority**: The feature must not degrade the existing no-churn guarantee: identity semantics are a standing promise of the tool.

**Independent Test**: Build an image with the previous release, upgrade, run again and observe exactly one rebuild; run a third time and observe reuse. Repeat with a config that lists `ripgrep` in `apt_packages`.

**Acceptance Scenarios**:

1. **Given** a project with an image built by the previous release, **When** the upgraded kekkai runs, **Then** the image rebuilds once and the sandbox starts normally.
2. **Given** the rebuilt image, **When** kekkai runs again with an unchanged config, **Then** the image is reused (no further rebuilds).
3. **Given** a config whose `apt_packages` already lists one of the new builtin tools, **When** the image builds, **Then** the build succeeds and the tool is present (duplication is harmless).

---

### Edge Cases

- A project's `apt_packages` duplicates a new builtin: the package manager treats the repeated name as a no-op; no error, no behavior change required from the user.
- The offline-fallback image lookup (used when the package registry is unreachable) keys on the bake inputs: after upgrade, pre-upgrade images no longer match the new package set — expected, since the sandbox contents genuinely differ; the fallback must never serve an image missing the new builtins.
- The distribution ships the `fd` tool under a renamed binary (`fdfind`) due to an upstream name clash: the sandbox must still expose the canonical `fd` name agents actually type (see FR-002).
- Firewall, sudoers, and mount behavior: none of the new tools introduce daemons, privileges, or network destinations; runtime behavior must be byte-for-byte unaffected apart from the tools existing.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The builtin package set baked into every sandbox image MUST additionally include: `unzip`, `zip`, `xz-utils`, `zstd`, `bzip2` (archive/compression), `file` (type detection), `ripgrep`, `fd-find` (search), `rsync` (file transfer/copy).
- **FR-002**: Each tool MUST be invocable in the sandbox under its canonical command name (`unzip`, `zip`, `xz`, `zstd`, `bzip2`, `file`, `rg`, `fd`, `rsync`) from every execution path (interactive shell, one-off exec, subprocesses). Where the distribution renames a binary (`fd` → `fdfind`), the canonical name MUST still resolve.
- **FR-003**: The builtin set remains a code constant: this feature adds no config keys, no schema change, and no user-visible toggles. User `apt_packages` continues to append to the builtin set unchanged.
- **FR-004**: The extended package set is a bake-time input: it MUST enter the image identity (triggering exactly one rebuild per project on upgrade) and the config-hash used by the offline fallback (so the fallback never reuses a pre-upgrade image for the new definition). After the one-time rebuild, an unchanged config MUST NOT rebuild.
- **FR-005**: Runtime security behavior MUST be unchanged: no new sudoers entries, no new network destinations, no daemons or services, firewall configuration and verification untouched.
- **FR-006**: SPECIFICATION.md's builtin package list MUST be amended in the same change as the behavior (spec-first).

### Key Entities

- **Builtin package set**: The code-constant list of packages baked into every sandbox image regardless of config; grouped by purpose (firewall/lifecycle, subcommands, convenience). This feature extends the convenience tier with a general-tooling tier.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a sandbox started from a project with no config file, all nine tools run successfully under their canonical names on the first attempt.
- **SC-002**: Archive round-trips (`.zip`, `.tar.xz`, `.tar.zst`) complete successfully inside a fresh sandbox with zero configuration.
- **SC-003**: Upgrading kekkai causes exactly one image rebuild per project config; subsequent runs with unchanged configs reuse the image 100% of the time.
- **SC-004**: Startup firewall verification probes pass unchanged; no new network destinations are reachable from the sandbox.
- **SC-005**: The sandbox image grows by less than 50 MB from the added packages.

## Assumptions

- The nine packages are small, daemon-free, and add no network egress paths; the container remains the security boundary (Principle II untouched — this changes image contents, not the boundary).
- Explicit non-goals, rejected during scoping: `python3`/`make`/`g++` (native-module toolchain — a separate, ~350 MB decision), `wget` (duplicates `curl`), `netcat`/`socat` (firewall-adjacent, low value), `htop`/`tree` (interactive conveniences already covered), `p7zip` (heavy, rare formats — user-config territory).
- A one-time rebuild for every user on upgrade is acceptable and expected; release notes should mention it.
- The canonical-name requirement (FR-002) is the resolution of the `fd`/`fdfind` naming question: agents type upstream names, so the sandbox exposes them; the mechanism (symlink or equivalent) is an implementation choice for planning.
- Constitution alignment: bake-time-inputs-only image hash respected (FR-004); no change to the builtin network destination set or firewall (Principle II); no new config surface (Principle III); validation end-to-end against a real docker daemon (Principle IV).
