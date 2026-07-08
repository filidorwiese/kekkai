# Feature Specification: kekkai exec

**Feature Branch**: `009-exec-command`

**Created**: 2026-07-08

**Status**: Draft

**Input**: User description: "Implement `kekkai exec` command to execute a cli command within the sandbox environment and exits after completion (passthrough the status code)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run a one-off command in the sandbox (Priority: P1)

A developer has a sandbox running for their project (`kekkai up` in another terminal). From a second terminal in the same project directory, they run `kekkai exec <command> [args...]` to execute a single command inside the sandbox — for example running the project's test suite, a linter, or inspecting a file — and get the command's output and exit code back, without opening an interactive shell.

**Why this priority**: This is the entire feature. Today the only way to run a command inside the sandbox is `kekkai shell` (interactive) — there is no scriptable, one-shot path.

**Independent Test**: With a sandbox running for the current directory, run `kekkai exec true` and `kekkai exec false`; verify output appears and the caller observes exit codes 0 and 1 respectively.

**Acceptance Scenarios**:

1. **Given** a running sandbox for the current directory, **When** the user runs `kekkai exec ls`, **Then** the directory listing from inside the sandbox is printed and kekkai exits 0.
2. **Given** a running sandbox, **When** the user runs a command that fails with exit code 2, **Then** kekkai exits with code 2.
3. **Given** a running sandbox, **When** the executed command writes to both stdout and stderr, **Then** each stream reaches the caller's corresponding stream.

---

### User Story 2 - Use exec from scripts and pipelines (Priority: P2)

A developer wires `kekkai exec` into shell scripts, git hooks, or CI-like automation: piping input into the command, capturing its output, and branching on its exit code.

**Why this priority**: Exit-code passthrough is explicitly requested; its main value is scriptability. Without correct stream and status behavior in non-interactive contexts, the feature only half-works.

**Independent Test**: Run `echo hi | kekkai exec cat | tr a-z A-Z` without a terminal attached; verify `HI` is produced and the pipeline succeeds.

**Acceptance Scenarios**:

1. **Given** a running sandbox, **When** stdin is piped into `kekkai exec <command>`, **Then** the command inside the sandbox receives that input.
2. **Given** a running sandbox and no terminal attached, **When** `kekkai exec` runs, **Then** it completes without requiring interactivity and its output is capturable.
3. **Given** a running sandbox, **When** the user interrupts a long-running `kekkai exec` (Ctrl+C), **Then** the command inside the sandbox is terminated and kekkai exits nonzero.

---

### User Story 3 - Clear failure when no sandbox is running (Priority: P3)

A developer runs `kekkai exec <command>` in a project directory with no running sandbox, and gets a clear error telling them nothing is running and how to start one.

**Why this priority**: Error path, not core value — but without it, failures are confusing and the exit code becomes ambiguous.

**Independent Test**: In a directory with no sandbox, run `kekkai exec ls`; verify a clear stderr message, nonzero exit, and that nothing was executed.

**Acceptance Scenarios**:

1. **Given** no running sandbox for the current directory, **When** the user runs `kekkai exec ls`, **Then** kekkai prints an error to stderr (mentioning `kekkai up`) and exits nonzero without running the command.
2. **Given** the user runs `kekkai exec` with no command, **Then** kekkai prints a usage error to stderr and exits nonzero.

---

### Edge Cases

- Executed command does not exist inside the sandbox → the failure and its exit code come through to the caller like any other command failure.
- Command arguments that look like flags (e.g. `kekkai exec ls -la`) → passed verbatim to the command, never interpreted by kekkai.
- Sandbox exists for a *different* directory but not the current one → treated as "no sandbox running" for this project.
- Executed command produces no output → kekkai stays silent apart from the command itself (no extra banners on stdout).
- Very long-running command → runs until completion or interrupt; no timeout imposed by kekkai.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST provide an `exec` subcommand that runs a user-supplied command with its arguments inside the running sandbox associated with the current directory.
- **FR-002**: `kekkai exec` MUST exit with the same exit code the executed command exited with.
- **FR-003**: The executed command's stdout and stderr MUST be delivered to the caller's stdout and stderr respectively, live (streamed, not buffered until completion); the caller's stdin MUST be forwarded to the command.
- **FR-004**: All words after the `exec` subcommand MUST be passed to the sandbox command verbatim; kekkai MUST NOT parse them as its own flags.
- **FR-005**: If no sandbox is running for the current directory, kekkai MUST print an actionable error to stderr and exit nonzero without executing anything.
- **FR-006**: Invoking `exec` without a command MUST produce a usage error on stderr and a nonzero exit.
- **FR-007**: The command MUST run under the same conditions as the sandbox's normal workload: same user, same project working directory, same environment, and subject to the sandbox's network restrictions.
- **FR-008**: Interrupting `kekkai exec` MUST terminate the command inside the sandbox and return a nonzero exit code; the sandbox itself MUST keep running.
- **FR-009**: `exec` MUST work both with and without a terminal attached (interactive use and scripts/pipelines).
- **FR-010**: The `exec` subcommand MUST resolve the target sandbox the same way `shell` does (by the sandbox's project-directory identity), and MUST NOT create, restart, or remove sandboxes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with a running sandbox can execute a one-off command and see its result in a single invocation, without opening an interactive shell.
- **SC-002**: For any executed command, the exit code observed by the caller equals the command's own exit code in 100% of cases.
- **SC-003**: `kekkai exec` composes with shell pipelines and scripts: piped input, captured output, and exit-code branching all behave as they would for a locally run command.
- **SC-004**: When no sandbox is running, users can tell what went wrong and what to do next from the error message alone, without consulting documentation.

## Assumptions

- `exec` requires an already-running sandbox and never starts one — consistent with `kekkai shell` and with the "minimal surface" principle; auto-starting would blur `up`'s role.
- The command executes in the project working directory inside the sandbox, as the sandbox user (same defaults as the interactive shell).
- Overhead added by kekkai itself is negligible; total runtime is dominated by the executed command.
- An optional `--` separator before the command is acceptable for disambiguation, but everything after `exec` is treated as the command either way.
- Per the constitution's spec-first principle, `SPECIFICATION.md` §3 (CLI) gains the `exec` line in the same change that implements it; that amendment is part of this feature's scope.
