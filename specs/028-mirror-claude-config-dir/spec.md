# Feature Specification: Mirror Claude Config Dir at Host Path

**Feature Branch**: `028-mirror-claude-config-dir`

**Created**: 2026-09-14

**Status**: Draft

**Input**: User description: "Mirror the Claude config dir at its host path inside the sandbox so plugins and other absolute-path state load. Today ~/.claude is bind-mounted at /home/kekkai/.claude with CLAUDE_CONFIG_DIR pointing there; Claude Code stores host-absolute paths (plugins/installed_plugins.json, plugins/known_marketplaces.json, hook/statusline commands in settings.json, MCP server paths in .claude.json), so inside the sandbox those paths do not exist and every enabled plugin silently fails to load. Fix: resolve the host config dir as $CLAUDE_CONFIG_DIR if set, else ~/.claude; bind it rw at that same path in the container; set CLAUDE_CONFIG_DIR to it; same path for the macOS preflight probe; protect it from disk mounts; /home/kekkai/.claude symlink at container start; do not mirror $HOME; update spec/README/prompt/demo wording. Same approach as specs/026 mirroring $PWD. No host state migration."

## Problem

Claude Code records absolute host paths inside its config directory: where each
installed plugin lives, where marketplaces are checked out, hook and statusline
commands, MCP server locations. The sandbox mounts that directory at a different
path (`/home/kekkai/.claude`), so every recorded path points at a location that
does not exist inside the container. Plugins enabled on the host are silently
absent in the sandbox, and any hook or helper referenced by absolute path fails.
Feature 026 fixed the same class of bug for the project directory by mirroring it
at its host path. This feature applies the same rule to the Claude config dir.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Host plugins work in the sandbox (Priority: P1)

A user has plugins installed and enabled on the host. They run `kekkai up` and
expect the sandbox Claude to offer exactly the same plugin skills, commands, and
agents as the host Claude, with no reinstall and no extra configuration.

**Why this priority**: The reported defect. Without it the sandbox is not "your
regular Claude Code, just contained" as README promises.

**Independent Test**: Enable one marketplace plugin on the host, run `kekkai up`,
type `/` in the sandbox: the plugin's slash commands are listed. `claude plugin
list` inside the sandbox reports the same plugins as on the host.

**Acceptance Scenarios**:

1. **Given** a plugin enabled in the host user scope, **When** the sandbox starts, **Then** its skills are invocable and no "plugin failed to load" notice appears.
2. **Given** a hook in host settings referencing a script by absolute path under the config dir, **When** the hook fires in the sandbox, **Then** the script runs.
3. **Given** a plugin installed from inside the sandbox, **When** the user later starts host Claude, **Then** the plugin loads there too (paths written in the sandbox are valid on the host).

---

### User Story 2 - Relocated host config dir is honored (Priority: P2)

A user runs host Claude with a custom config directory via the `CLAUDE_CONFIG_DIR`
environment variable. They expect `kekkai up` to carry that same directory into
the sandbox instead of a default `~/.claude` they do not use.

**Why this priority**: The fix depends on the sandbox seeing the same directory
the host wrote paths from. Hardcoding `~/.claude` reintroduces the defect for
exactly these users.

**Independent Test**: Export `CLAUDE_CONFIG_DIR=/tmp/cc` on the host with a
settings file and an enabled plugin, run `kekkai up`, verify the sandbox loads
settings and plugins from it and `/tmp/cc` is the config dir inside the container.

**Acceptance Scenarios**:

1. **Given** `CLAUDE_CONFIG_DIR` set on the host, **When** the sandbox starts, **Then** that directory is mounted read-write at the same path and is Claude's config dir inside.
2. **Given** `CLAUDE_CONFIG_DIR` unset, **When** the sandbox starts, **Then** `~/.claude` (the host home as the OS reports it, symlinks not resolved) is used.
3. **Given** the configured directory does not exist yet, **When** the sandbox starts, **Then** it is created on the host owned by the user before mounting, as today.

---

### User Story 3 - `~/.claude` still resolves inside the sandbox (Priority: P3)

A user opens `kekkai shell` and types `ls ~/.claude`, or reads docs and prompt
text that say `~/.claude`. They expect to land on their Claude state, not an empty
or missing directory.

**Why this priority**: Convenience and continuity. Nothing functional depends on
it, but a missing `~/.claude` in the sandbox shell would look like lost state.

**Independent Test**: `kekkai shell`, then `ls -la ~/.claude` shows the host
content and `readlink ~/.claude` prints the mirrored host path.

**Acceptance Scenarios**:

1. **Given** the sandbox is running, **When** the user lists `/home/kekkai/.claude`, **Then** it is a symlink to the mirrored config dir and shows the host content.
2. **Given** the mirrored path already equals `/home/kekkai/.claude` (host user named `kekkai`), **When** the sandbox starts, **Then** no symlink is attempted and the directory is mounted directly.

---

### Edge Cases

- Host config dir path contains `:` or control characters: refused before any docker work with the same message style as an unmirrorable project path (§7.1).
- Host config dir is `/`, or a parent of `/home/kekkai`, `/usr/local/bin`, or `/commandhistory`: refused for the same reason as project paths.
- Config dir lies inside the project (e.g. project is the home directory): both binds apply, config dir layered after the project bind so it stays read-write and unshadowed. Secrets hiding still only applies to project-relative paths.
- Project lies inside the config dir: allowed. The project bind nests inside the config bind.
- User `disk.mounts` entry targets the config dir or a path beneath it: rejected in the one-pass `up` report. Today no target check exists at all; this is the first one and is scoped to the config dir only.
- `CLAUDE_CONFIG_DIR` on the host is a relative path: made absolute against the invocation directory before use.
- Config dir is a symlink on the host: the unresolved path is what host Claude recorded, so the unresolved path is the mount destination. The source is passed to the runtime as given, unchanged from today.
- Sandbox already running with the previous layout: it keeps the old mount until it exits. Containers are `--rm`, so the next `kekkai up` picks up the new layout with no migration.
- Image hash: the config dir path never enters the image (runtime input, never bake-time, constitution constraint). The runtime symlink step lives in the image CMD, so this release changes the rendered template once, like any release that touches it. That is the only rebuild.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `kekkai up` MUST determine the host Claude config dir as `$CLAUDE_CONFIG_DIR` when set and non-empty, otherwise `<host home>/.claude`, with the home path taken as the OS reports it (no symlink resolution).
- **FR-002**: The config dir MUST be bind-mounted read-write inside the container at exactly its host path.
- **FR-003**: `CLAUDE_CONFIG_DIR` inside the container MUST be set to that same path, before user env so a user `env` entry can still override it.
- **FR-004**: The macOS preflight probe MUST bind the same resolved host config dir it will later mount.
- **FR-005**: The config dir path MUST be validated with the same rules as the project path: no `:` or control characters, not the filesystem root, not a parent of a kekkai-owned container path. Violations abort before docker work with an actionable message.
- **FR-006**: `up` MUST reject any `disk.mounts` target equal to or beneath the mirrored config dir, in the same one-pass validation report. The project path check is not extended: a project containing the config dir (project at `$HOME`) stays allowed.
- **FR-007**: At container start, before Claude launches, `/home/kekkai/.claude` MUST become a symlink to the mirrored config dir. When the mirrored path already is `/home/kekkai/.claude`, no symlink is created. The symlink MUST be created at runtime, never baked into the image.
- **FR-008**: The container home MUST remain `/home/kekkai`; only the config dir is mirrored, nothing else from the host home.
- **FR-009**: The config dir path MUST NOT enter the image or its hash; one image serves every host user. The CMD may change (one rebuild on upgrade, as with any template change).
- **FR-010**: No host file under the config dir may be modified, moved, or rewritten by kekkai. The fix is purely a change of where the directory appears in the container.
- **FR-011**: SPECIFICATION.md §5.2, §5.3, §7.1, §7.3, §7.4, README (mount description, security note, refusal note), the sandbox-awareness prompt filesystem line, and `docs/my-project` demo text MUST describe the config dir as mirrored at its host path, with `~/.claude` retained only as a symlink convenience.

### Key Entities

- **Host config dir**: The directory host Claude Code uses for settings, sessions, plugins, and registries. Path is `$CLAUDE_CONFIG_DIR` or `~/.claude`. Contains files with absolute paths rooted at itself.
- **Mirrored config dir**: The same directory inside the container at the identical path. Read-write. Protected from user mounts.
- **Convenience symlink**: `/home/kekkai/.claude` pointing at the mirrored config dir. Runtime-only.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of plugins enabled on the host are listed and invocable in the sandbox on first start, with no reinstall.
- **SC-002**: A plugin installed from inside the sandbox loads on the host on the next host start, and vice versa, with zero manual path edits.
- **SC-003**: Hooks and helpers referenced by absolute path under the config dir succeed in the sandbox at the same rate as on the host.
- **SC-004**: Users with a relocated host config dir see their own settings in the sandbox on first start, with no extra kekkai configuration.
- **SC-005**: Two users with different home directories share one image tag after upgrade; switching `CLAUDE_CONFIG_DIR` on the host triggers no rebuild.
- **SC-006**: `ls ~/.claude` inside `kekkai shell` shows the host content in every supported host layout.

## Assumptions

- Claude Code derives every absolute path it stores from its config dir root, so mirroring the root at its host path makes all stored paths valid. Confirmed for plugin and marketplace registries; assumed for hooks, statusline, and MCP entries authored by the user against that root.
- Host Claude uses the unresolved home path when writing paths. Mirroring the unresolved path therefore matches what is on disk. Symlinked home directories on the host are not separately handled.
- Docker orders nested bind mounts correctly, so config-dir-inside-project and project-inside-config-dir both work without special handling beyond mount order.
- The host username appearing in a container path is acceptable. It is already exposed by the mirrored project path since feature 026.
- Mirroring the whole host home is out of scope. The image bakes `/home/kekkai` into nvm, bashrc, history, and protected paths, and the project mirror plus the config dir mirror already cover every path Claude Code writes.
- The runtime symlink lives in the container CMD sequence (before firewall init), since the target is only known at run time. Exact CMD ordering is a plan-level detail.
- No migration of existing host state is needed. The previous layout never wrote anything path-dependent to the host that the new layout cannot read.
