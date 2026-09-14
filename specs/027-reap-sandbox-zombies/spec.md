# Feature Specification: Reap orphaned processes inside the sandbox

**Feature Branch**: `027-reap-sandbox-zombies`

**Created**: 2026-09-09

**Status**: Implemented

**Input**: User description: "Reap orphaned zombie processes inside the kekkai sandbox. Today `kekkai up` runs `docker run` without `--init` and the image CMD ends in `exec claude`, so claude is PID 1 in the container. Grandchild processes spawned by Claude Code (e.g. statusline hooks: `sh -c bash ~/.claude/statusline-wrapper.sh` -> `bash` -> `npm exec ccstat`) re-parent to PID 1 when their intermediate parent exits; Node only reaps its own children, so these accumulate as `<defunct>` zombies for the whole session (one or more per statusline refresh). On the host this does not happen because orphans re-parent to systemd. Goal: sandbox must reap orphaned processes so no zombies accumulate; claude must still receive terminal signals (SIGINT/SIGTERM/SIGWINCH) and its exit code must propagate to `kekkai up`. Preferred approach: add `--init` to the docker run args in buildRunArgs (internal/runtime/up.go) so Docker's bundled tini becomes PID 1; no image rebuild or image hash change. Must work on Linux and macOS (Docker Desktop) and with snap-installed docker. Verify: after a session with a statusline command that spawns subprocesses, `ps` on host shows no defunct children under the container's claude process; ctrl-c and `kekkai down` behave as before; exit code of claude propagates."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Long sessions do not accumulate dead processes (Priority: P1)

A developer runs Claude Code inside a kekkai sandbox with a status line, hooks or other helpers that spawn short-lived subprocesses. Every process those helpers leave behind is cleaned up by the sandbox as soon as it exits. Inspecting the process table on the host during or after a long session shows only live processes belonging to the sandbox, never a growing list of defunct entries.

**Why this priority**: This is the defect being fixed. Today each status line refresh can leave one or more defunct processes that persist for the life of the session. They are harmless individually but grow without bound, consume process table slots, and look like a leak to anyone inspecting the host.

**Independent Test**: Configure a status line command that spawns a nested subprocess, run a sandbox session for several minutes, and list the sandbox's processes from the host. Zero entries are defunct.

**Acceptance Scenarios**:

1. **Given** a sandbox with a status line command that spawns nested subprocesses, **When** the session has run long enough for many refreshes, **Then** the host process listing for that sandbox contains no defunct entries.
2. **Given** a subprocess inside the sandbox whose parent exits before it does, **When** the orphan itself exits, **Then** it is reaped within a second and disappears from the process table.
3. **Given** a sandbox without any status line or hooks, **When** it runs, **Then** behavior is indistinguishable from today.

---

### User Story 2 - Signals and exit status behave exactly as before (Priority: P1)

A developer interrupts a session with Ctrl-C, stops it with `kekkai down`, resizes the terminal, or lets Claude exit on its own. In every case the sandbox reacts as it does today: Claude receives the interrupt, the terminal resize is honored, `kekkai down` stops the sandbox promptly, and the exit status reported by `kekkai up` equals the exit status of Claude.

**Why this priority**: Fixing reaping must not regress the lifecycle guarantees already specified. Introducing an intermediary between the container and Claude risks swallowing signals, delaying shutdown, or masking the exit code. These are the regressions most likely to slip through.

**Independent Test**: Run a sandbox, press Ctrl-C, and confirm Claude handles it as on the host. Run `kekkai down` from another terminal and confirm the sandbox stops within the usual grace period. Resize the terminal and confirm Claude redraws. Exit Claude with a known non-zero status and confirm `kekkai up` returns that status.

**Acceptance Scenarios**:

1. **Given** a running sandbox, **When** the user presses Ctrl-C, **Then** Claude receives the interrupt and behaves as it does when run directly on the host.
2. **Given** a running sandbox, **When** `kekkai down` is executed, **Then** the sandbox stops and is removed within the same time it takes today.
3. **Given** a running sandbox, **When** the terminal window is resized, **Then** Claude redraws to the new size.
4. **Given** Claude exits with a given status, **When** the sandbox ends, **Then** `kekkai up` exits with that same status.
5. **Given** the capture proxy loop is running alongside Claude, **When** Claude exits, **Then** the sandbox terminates as it does today with no lingering processes keeping it alive.

---

### User Story 3 - No rebuild, works on every supported host (Priority: P2)

A developer with an already-built sandbox image upgrades kekkai. The next `kekkai up` starts without rebuilding the image, and the fix is active on Linux hosts (including a snap-installed Docker) and on Docker Desktop for macOS.

**Why this priority**: The image hash is derived from bake-time inputs only; a lifecycle fix must not invalidate every user's cached image. Support across the supported host set is a constitutional constraint, not a nice-to-have.

**Independent Test**: With an existing image, upgrade kekkai and run `kekkai up`; observe no image build. Repeat the zombie test from User Story 1 on Linux with snap Docker and on Docker Desktop for macOS.

**Acceptance Scenarios**:

1. **Given** an existing sandbox image, **When** kekkai is upgraded and `kekkai up` runs, **Then** no image rebuild is triggered and the image hash is unchanged.
2. **Given** a Linux host with a snap-installed Docker, **When** a sandbox runs, **Then** orphaned processes are reaped.
3. **Given** a Docker Desktop for macOS host, **When** a sandbox runs, **Then** orphaned processes are reaped and the preflight probe still passes.

---

### Edge Cases

- The container runtime does not provide a reaping helper: `kekkai up` MUST fail before sandbox start with a message naming the missing capability, rather than silently running without reaping.
- The firewall script or capture proxy fails before Claude starts: the sandbox MUST still exit with a failure status as today; the reaper must not turn a startup failure into a hang.
- Claude is killed by a signal rather than exiting normally: `kekkai up` MUST report a non-zero status, consistent with today.
- Orphans that never exit (a runaway daemon started by a hook): out of scope; they are live processes, not zombies, and are torn down with the container.
- `kekkai shell` and `kekkai exec` sessions: they attach to the running container and inherit its reaper; no separate handling required.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The sandbox MUST reap every process that becomes orphaned inside the container, so that no defunct processes accumulate over the lifetime of a session.
- **FR-002**: Claude MUST continue to receive interrupt, terminate and terminal-resize signals delivered to the sandbox exactly as it does today.
- **FR-003**: The exit status of `kekkai up` MUST equal the exit status of Claude, including the non-zero status when Claude is terminated by a signal.
- **FR-004**: `kekkai down` MUST stop and remove the sandbox within the same grace period as today.
- **FR-005**: The fix MUST NOT change any bake-time input; the image hash and image reuse MUST be unaffected, and no rebuild may be triggered by the upgrade.
- **FR-006**: The fix MUST work on linux/amd64 and linux/arm64 hosts, including snap-installed Docker, and on Docker Desktop for macOS (Apple silicon).
- **FR-007**: If the container runtime cannot provide process reaping, `kekkai up` MUST abort before sandbox work with a message that names the missing capability.
- **FR-008**: Startup failures before Claude starts (firewall init, capture proxy) MUST still terminate the sandbox with a failure status; reaping MUST NOT introduce a hang.
- **FR-009**: `SPECIFICATION.md` (§7.2 lifecycle, §7.3 run args assembly) MUST be updated in the same change per the spec-first principle.

### Key Entities

- **Sandbox init process**: the first process in the container, responsible for adopting and reaping orphans and for forwarding signals to Claude. Today this role is held by Claude itself, which does not reap orphans.
- **Orphaned process**: a process whose parent exited before it did; it is re-parented to the init process and must be reaped by it on exit.
- **Defunct (zombie) process**: an exited process whose exit status has not been collected by its parent; it occupies a process table slot until reaped.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After a 10-minute session with a status line command that spawns nested subprocesses, the host process listing for the sandbox shows zero defunct entries.
- **SC-002**: 100% of lifecycle checks (Ctrl-C, `kekkai down`, terminal resize, exit status propagation for 0 and non-zero, signal-kill status) produce the same observable result as before the change.
- **SC-003**: Upgrading kekkai on a host with a cached image triggers zero image rebuilds.
- **SC-004**: The zombie test (SC-001) passes on Linux with snap Docker and on Docker Desktop for macOS.

## Assumptions

- The user's preferred approach is to request the container runtime's bundled init process at run time (`--init`), which makes a tiny reaper the first process and Claude its child. This is a run-argument change only, so it satisfies FR-005 by construction and needs no image change.
- Docker's bundled init forwards received signals to its child and exits with the child's status; terminal-resize signals are delivered by the runtime to the attached process and are unaffected. Planning MUST verify these on Linux and Docker Desktop rather than assume them.
- Snap-installed Docker and Docker Desktop ship the bundled init helper; if a runtime lacks it the run fails fast at container creation, which satisfies FR-007 without a separate probe. Planning MAY add a preflight probe if the failure message is not clear enough.
- The capture proxy loop backgrounded in the CMD remains a child of Claude after Claude replaces the shell; when Claude exits, the container's init terminates remaining processes and the sandbox ends as today.
- The existing startup version line, firewall init and `stty -echo` sequence in the CMD are unchanged.
- No unit tests; validation is end-to-end per the constitution, using a status line that spawns nested subprocesses as the reproduction.
- Baking a reaper into the image (e.g. installing one as an apt package) is not pursued because it would change the image hash and add a dependency for no benefit over the runtime-provided one.
