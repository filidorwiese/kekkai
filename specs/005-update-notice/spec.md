# Feature Specification: Update Notice on Up

**Feature Branch**: `005-update-notice`

**Created**: 2026-07-07

**Status**: Draft

**Input**: User description: "On `kekkai up` show a friendly message when there is a newer version available, if not - show nothing"

## Clarifications

### Session 2026-07-07

- Q: What should the notice tell the user to do? → A: The notice must literally name the `kekkai self-update` command as the way to update.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Notified of a newer release at startup (Priority: P1)

A user runs `kekkai up` while a newer kekkai release exists. Somewhere in the normal
startup output they see one short, friendly line telling them a newer version is
available and how to get it (the built-in self-update command). The sandbox starts
exactly as it would have without the notice.

**Why this priority**: This is the feature — users who never think to run
`kekkai self-update` learn about updates at the moment they use the tool.

**Independent Test**: Run an older-versioned build of `kekkai up` in a valid project
and observe a single notice line naming the newer version; sandbox startup proceeds
normally.

**Acceptance Scenarios**:

1. **Given** the installed version is older than the latest release, **When** the user runs `kekkai up`, **Then** exactly one friendly notice line appears, naming the available version and pointing at `kekkai self-update`, and startup continues normally.
2. **Given** the notice is shown, **When** the sandbox starts, **Then** the notice never delays or interferes with the interactive session handoff.

---

### User Story 2 - Silence when current (Priority: P1)

A user on the latest version (or newer, e.g. running ahead of the latest release) runs
`kekkai up` and sees no version-related output at all.

**Why this priority**: The explicit requirement is "if not - show nothing"; noise-free
startup is part of the feature's definition.

**Independent Test**: Run `kekkai up` with the installed version equal to the latest
release; startup output contains no update-related text.

**Acceptance Scenarios**:

1. **Given** the installed version equals the latest release, **When** the user runs `kekkai up`, **Then** no update-related output appears.
2. **Given** the installed version is newer than the latest release, **When** the user runs `kekkai up`, **Then** no update-related output appears.

---

### User Story 3 - Never blocks, never breaks startup (Priority: P2)

A user runs `kekkai up` while offline, behind a proxy, or when the release service is
slow, rate-limited, or down. Startup behaves exactly as if the feature did not exist:
no notice, no warning, no added failure, no noticeable delay.

**Why this priority**: An informational nicety must never degrade the core `up` flow;
silent failure is the only acceptable failure mode.

**Independent Test**: Run `kekkai up` with the release service unreachable; startup
succeeds with no update-related output and no perceptible slowdown.

**Acceptance Scenarios**:

1. **Given** the release service is unreachable or errors, **When** the user runs `kekkai up`, **Then** startup proceeds with no update-related output and no error.
2. **Given** the release service responds slowly, **When** the user runs `kekkai up`, **Then** the check never adds noticeable wait time to startup (it is abandoned rather than awaited).

---

### Edge Cases

- Development (unversioned) build: no check, no notice — there is no meaningful "newer" for a dev build (same rule as the self-update command's refusal, but silent here).
- Release information malformed or version strings not comparable: treated as inconclusive → silent.
- `kekkai up` aborts earlier for other reasons (invalid config, existing sandbox): no requirement to show the notice; it must never appear amid an error report as if part of it.
- Repository override in effect (same mechanism the self-update command honors): the check follows the same source of truth so the notice never advertises a version self-update would not install.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: During `kekkai up`, the system MUST determine whether a newer release than the installed version is available, using the same release source of truth as the built-in self-update command.
- **FR-002**: When a newer release exists, the system MUST print exactly one friendly notice line naming the newer version and literally naming the `kekkai self-update` command as the way to update.
- **FR-003**: When no newer release exists (equal or ahead), the system MUST print nothing update-related.
- **FR-004**: Any failure or ambiguity in the check (offline, timeout, rate limit, malformed response, incomparable versions) MUST result in silence — never a warning, error, or startup abort.
- **FR-005**: The check MUST NOT add noticeable delay to startup and MUST NOT delay the interactive session handoff; if the answer is not available in time, the notice is skipped for this run.
- **FR-006**: Development (unversioned) builds MUST skip the check entirely.
- **FR-007**: The notice MUST be informational only — it never prompts, never requires input, and never changes `up` behavior or exit status.

### Key Entities

- **Installed version**: The version stamped into the running binary; dev builds carry a non-release marker.
- **Latest release**: The newest published release from the same source the self-update command consults.
- **Update notice**: A single output line shown only when latest > installed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With a newer release available, 100% of `kekkai up` runs show exactly one update notice line; with no newer release, 0% show any update-related output.
- **SC-002**: Startup wall-clock time with the feature (any network condition, including unreachable release service) is indistinguishable from startup without it — no user-perceptible added wait.
- **SC-003**: `kekkai up` never fails or changes exit status because of the update check, under any release-service condition.
- **SC-004**: The advertised version always matches what `kekkai self-update` would install at that moment (same source of truth).

## Assumptions

- The notice appears once per `up` run; no persistence, throttling, or "don't show again" state — one line per startup is acceptable noise and avoids new state files (aligns with the project's minimal-surface principle).
- Placement in startup output is flexible as long as the interactive handoff is not delayed; a run whose startup fails before the notice would appear simply shows no notice.
- The release source and dev-build semantics established for the self-update command are reused, not redefined.
- No new configuration key: the notice is not user-configurable (opt-out not requested; minimal surface).
