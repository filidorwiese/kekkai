# Feature Specification: Mirror the host project path inside the sandbox

**Feature Branch**: `026-mirror-host-path`

**Created**: 2026-09-09

**Status**: Draft

**Input**: User description: "Mirror the host project path inside the sandbox so per-project Claude Code state stops colliding across projects. kekkai bind-mounts the host cwd at the fixed container path /workspace and sets the container workdir to /workspace. Claude Code keys all per-project state by the sanitized absolute cwd, so every kekkai sandbox on the machine resolves to the same key -workspace: memory, sessions, trust decisions and allowed-tools grants bleed between unrelated projects, and none of it is shared with the host. Mount the host project directory inside the container at the exact same absolute path as on the host and set the working directory to that path. All builtin mounts, user mounts and secret shadows that target the workspace must follow. Dockerfile WORKDIR, sandbox prompt, init/firewall scripts, mpr script, docs and README must not assume a fixed /workspace. Container naming, labels and the per-project history volume keep working. Define behavior for spaces and unusual characters. On macOS the path must be within a runtime shared path; preflight reports clearly when it is not. Existing state under projects/-workspace is not migrated; document that it is orphaned. Symlink-based approaches are out of scope."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Each project keeps its own Claude state (Priority: P1)

A developer uses kekkai in several unrelated projects on the same machine. Inside every sandbox Claude Code sees the project at the same absolute path it has on the host. Claude therefore derives a different project key per project, so auto-memory, session transcripts, trust decisions, allowed-tools grants and prompt history recorded while working in project A are never visible in a sandbox for project B.

**Why this priority**: This is the bug being fixed. Today a memory written about project A is presented to Claude as fact in project B, and a tool grant accepted in A silently applies in B. For a security sandbox, cross-project bleed of permissions is a boundary defect, not a cosmetic one.

**Independent Test**: Start a sandbox in project A, have Claude save a memory, stop. Start a sandbox in project B and verify the memory is absent and the sandbox cwd equals the host path of B.

**Acceptance Scenarios**:

1. **Given** two projects at different host paths, **When** a sandbox is started in each, **Then** the working directory reported inside each sandbox equals that project's host path.
2. **Given** a memory, session or tool grant recorded in a sandbox for project A, **When** a sandbox is started for project B, **Then** none of that state is visible to Claude in B.
3. **Given** a sandbox for project A is stopped and restarted, **When** Claude resumes, **Then** memory, sessions and grants from the earlier A session are present.

---

### User Story 2 - Host and sandbox share the same project state (Priority: P1)

A developer alternates between running Claude Code directly on the host and inside a kekkai sandbox for the same project. Because the project path is identical in both, Claude uses one project key: memory saved on the host is available in the sandbox and vice versa, sessions started on either side can be resumed on the other, and project-local permission rules apply consistently. Trust decisions, MCP servers and prompt history live in a per-config-directory file; they become per-project inside the sandbox but are shared with the host only when the host also uses the shared Claude config directory.

**Why this priority**: Kekkai already shares the whole Claude config directory with the host; per-project state was the only piece that failed to line up. Sharing it is what makes the sandbox a drop-in for host usage rather than a parallel world.

**Independent Test**: Save a memory with Claude Code on the host in a project, then start a kekkai sandbox in that project and verify Claude can recall it; do the reverse.

**Acceptance Scenarios**:

1. **Given** memory or sessions recorded by Claude Code on the host, **When** a sandbox is started in the same project, **Then** Claude inside the sandbox sees them.
2. **Given** memory or sessions recorded inside a sandbox, **When** Claude Code is later run on the host in the same project, **Then** the host sees them.
3. **Given** the host project directory is reached through a symlink, **When** the sandbox is started from the symlinked path, **Then** the path used inside the sandbox is the resolved real path, matching what Claude Code on the host would use.

---

### User Story 3 - Everything that referenced the workspace keeps working (Priority: P2)

A developer with an existing configuration (read-only config bind, no-commit git bind, user mounts, secret shadows, sandbox awareness prompt, traffic and mpr inspection, shell and exec subcommands, command history) upgrades kekkai. Every feature continues to behave as documented; the only visible change is that the project lives at its host path instead of a fixed location.

**Why this priority**: The fixed workspace path is assumed in several places (builtin mounts, secret shadows, image definition, prompt text, docs). Missing one would silently break a security feature such as the read-only config or a secret shadow.

**Independent Test**: Run the existing end-to-end validation (config read-only, `.git` read-only with git disabled, a secret shadow, a user mount, `kekkai shell`, `kekkai exec`, `kekkai traffic`, `kekkai mpr`) in a project and confirm each behaves as before with the mirrored path.

**Acceptance Scenarios**:

1. **Given** a project with a config file, **When** the sandbox runs, **Then** the config is readable at its host path inside the sandbox and cannot be modified from inside.
2. **Given** git disabled, **When** the sandbox runs, **Then** the repository history is readable and commits fail, exactly as today.
3. **Given** a `secrets.hide` entry, **When** the sandbox runs, **Then** the entry is shadowed at its host-relative location inside the sandbox.
4. **Given** the sandbox awareness prompt, **When** Claude starts, **Then** the prompt describes the project location without naming a fixed path that no longer exists.
5. **Given** a running sandbox, **When** `kekkai shell`, `kekkai exec`, `kekkai traffic`, `kekkai mpr` and `kekkai down` are used, **Then** they locate and operate on the sandbox as before, and shell/exec start in the mirrored project directory.
6. **Given** command history from earlier sessions in this project, **When** the sandbox restarts, **Then** the history is still present.
7. **Given** two projects, **When** images are built, **Then** both projects reuse the same image; the project path never causes a rebuild.

---

### User Story 4 - Unsupported paths fail early and clearly (Priority: P2)

A developer runs kekkai from a directory whose path cannot be mirrored: on macOS it is outside the directories the container runtime shares with the VM, or the path contains characters the runtime cannot express as a mount destination. Kekkai refuses before any sandbox work with a message that names the problem and the fix.

**Why this priority**: The previous fixed path never failed for these reasons. A cryptic runtime error mid-startup would be a regression in usability; a silent fallback to a different path would reintroduce the collision this feature removes.

**Independent Test**: On macOS, run kekkai from a directory outside the shared paths and verify a clear pre-start error; on any platform, run from a directory whose path contains a colon and verify a clear pre-start error.

**Acceptance Scenarios**:

1. **Given** macOS and a project outside the runtime's shared paths, **When** `kekkai up` runs, **Then** it aborts before starting the sandbox with a message naming the path and how to share it.
2. **Given** a project path containing characters that cannot be expressed unambiguously as a mount destination, **When** `kekkai up` runs, **Then** it aborts before any container work with a message naming the offending path.
3. **Given** a project path containing spaces or non-ASCII characters, **When** `kekkai up` runs, **Then** the sandbox starts and the path is mirrored exactly.

---

### Edge Cases

- Host project path lies inside the sandbox user's own home directory (host user named `kekkai`): the project is still mounted at its exact path; the separately mounted Claude config directory is unaffected. No special handling.
- Host project path is the filesystem root or a very shallow path such as `/home`: mirroring would overlay system directories inside the sandbox. Kekkai MUST refuse paths that are the root directory or a parent of the sandbox user's home or of the Claude config directory.
- Parent directories of the mirrored path do not exist in the image: they are created at start as needed and are not writable by the sandbox user; only the project directory itself is the host bind.
- Existing users with state under the old shared project key: it is left in place and simply no longer used. It is documented as orphaned and safe to delete manually; no automatic migration or notice.
- Symlink in the host path: resolved to the real path before mirroring, so the key matches what Claude Code on the host derives.
- Very long host paths: no kekkai-imposed limit; runtime limits apply and surface through the pre-start check.
- Path changes while a sandbox is running (project directory moved or renamed on the host): out of scope, identical to today's behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The sandbox MUST mount the host project directory at the identical absolute path inside the container and MUST start Claude, `kekkai shell` and `kekkai exec` with that path as the working directory.
- **FR-002**: The host path used for mirroring MUST be the fully resolved real path of the invocation directory (symlinks resolved), so it matches the key Claude Code derives on the host.
- **FR-003**: Every builtin bind that targets a location within the workspace (read-only config file or placeholder, read-only `.git` when git is disabled) MUST target the same location under the mirrored path.
- **FR-004**: Every `secrets.hide` shadow MUST be applied at its host-relative location under the mirrored path.
- **FR-005**: User `disk.mounts` targets MUST keep their documented semantics; any inferred target that previously resolved under the fixed workspace path MUST resolve under the mirrored path instead.
- **FR-006**: The image definition MUST NOT contain the project path; the mirrored path MUST be a runtime input only, so the image hash and image reuse across projects are unchanged.
- **FR-007**: The sandbox awareness prompt, the in-container scripts, the specification, README and docs MUST NOT refer to a fixed workspace path; they MUST describe the project as living at its host path.
- **FR-008**: Container naming, labels, sandbox discovery by subcommands (`down`, `shell`, `exec`, `traffic`, `mpr`) and the per-project command history volume MUST behave exactly as before.
- **FR-009**: Before any container work, kekkai MUST reject a project path that is the filesystem root, a parent of the sandbox user's home, a parent of the Claude config location inside the sandbox, or that contains characters the runtime cannot accept unambiguously as a mount destination; the error MUST name the path and the reason.
- **FR-010**: Paths containing spaces or non-ASCII characters MUST be supported and mirrored byte-exactly.
- **FR-011**: On macOS the pre-start preflight MUST bind the project at its mirrored path (not a probe-only path) so that a project outside the runtime's shared directories is detected before sandbox start, and the error MUST name the path and how to share it.
- **FR-012**: Existing state accumulated under the former shared project key MUST NOT be migrated, deleted or modified; documentation MUST state that it is orphaned and may be removed manually.
- **FR-013**: `SPECIFICATION.md` MUST be updated in the same change (mounts section, run-argument order, preflight description) per the spec-first principle.

### Key Entities

- **Project path**: the resolved absolute host directory kekkai was invoked from; now also the in-sandbox location of the project and the input to Claude Code's project key.
- **Project key**: the identifier Claude Code derives from the working directory to partition per-project state (memory, sessions, trust, tool grants, history). Identical on host and in sandbox after this feature.
- **Workspace-relative bind**: any builtin or user-configured mount or shadow whose target is expressed relative to the project; all now resolve under the project path.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a test with two projects, zero items of per-project Claude state (memory, sessions, trust, tool grants, prompt history) recorded in one project are visible in a sandbox for the other.
- **SC-002**: 100% of per-project memory and sessions recorded on the host are visible in a sandbox for the same project, and vice versa, with no manual step.
- **SC-003**: The full existing end-to-end validation (config read-only, git read-only, secret shadow, user mount, shell, exec, traffic, mpr, history persistence, firewall probes) passes unchanged on Linux and on Docker Desktop for macOS.
- **SC-004**: Starting sandboxes in two different projects triggers no image rebuild attributable to the project path.
- **SC-005**: Every unsupported path (macOS unshared, root, forbidden characters) is rejected before any container is created, with a message that names the path and the fix.
- **SC-006**: No remaining reference to a fixed workspace path in the specification, README, docs, sandbox prompt or in-container scripts.

## Assumptions

- Claude Code keys per-project state solely by the working directory it starts in; the sandbox user's home differing from the host user's home does not affect the key.
- Memory and sessions live under the shared Claude config directory and are therefore host-shared; trust, MCP servers and prompt history live in a file inside that config directory which the host by default does not use, so they are per-project but not host-shared (see research R1). Unifying them is out of scope.
- Rejected in-container overlay targets are kekkai's own directories only (sandbox home, tool directory, history volume); other system paths are not protected because the container is disposable.
- The runtime creates missing parent directories for a bind destination automatically; kekkai does not need to create them and they are not part of the boundary.
- Docker's `-v` form is ambiguous for paths containing `:`; mount syntax choice is an implementation detail, but the set of rejected characters MUST be documented and stable.
- Linux hosts do not need a preflight container for path sharing; only the host-side path checks (FR-009) apply there.
- No one-time notice about orphaned old state is printed; documentation is sufficient given the state is harmless.
- Windows and Intel macOS remain out of scope per the constitution.
- Symlink-based or alias-based schemes that keep a fixed in-container path are out of scope because the working directory is resolved to its real path by the tooling inside the sandbox.
