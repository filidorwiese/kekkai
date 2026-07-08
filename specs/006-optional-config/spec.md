# Feature Specification: Optional Configuration File

**Feature Branch**: `006-optional-config`

**Created**: 2026-07-08

**Status**: Draft

**Input**: User description: "Option A with clear warning (maybe in yellow?). Also update the default configuration template (that is used for `kekkai init`) to be completely commented out."

## Overview

Today `kekkai up` refuses to start without a `.kekkai.yaml` in the working
directory, even though every configuration field has a safe built-in default and
the defaults are kekkai's most restrictive posture (egress firewall on, no git
identity, no SSH agent, no extra mounts). The file requirement adds friction for
first-time users without adding safety.

This feature makes the configuration file optional: `kekkai up` in a directory
without one starts a sandbox with pure defaults, after printing one clearly
visible (yellow) warning so the run is never silently unconfigured. A present
but empty (or fully commented-out) configuration file means the same thing:
all defaults, no warning needed beyond none. Consistently, `kekkai init` now
writes a starter file in which every line is commented out, so a freshly
initialized project also runs on pure defaults until the user uncomments
something.

## Clarifications

### Session 2026-07-08

- Q: Documentation scope? → A: Update README.md and SPECIFICATION.md wherever they describe the configuration file as required, the `init`-then-`up` flow, or the starter template; document the new warning.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Try kekkai instantly in any project (Priority: P1)

A developer hears about kekkai, installs it, changes into an existing project
directory, and runs `kekkai up` without any setup step. The sandbox starts with
default settings. A single, visually distinct (yellow) warning tells them no
configuration file was found, defaults are in use, and `kekkai init` creates one
to customize.

**Why this priority**: This is the core value: zero-friction onboarding. It
removes the only mandatory setup step while keeping the wrong-directory case
visible.

**Independent Test**: In a directory without any kekkai configuration file, run
`kekkai up`. The sandbox starts with defaults and exactly one warning line
appears before startup output.

**Acceptance Scenarios**:

1. **Given** a directory without `.kekkai.yaml` or `.kekkai.yml`, **When** the
   user runs `kekkai up`, **Then** one yellow warning states that no
   configuration was found, defaults apply, and `kekkai init` customizes —
   and the sandbox starts with all-default settings.
2. **Given** the same directory, **When** the sandbox starts, **Then** its
   behavior is identical to a sandbox started from a configuration file that
   contains only defaults (firewall on, no git identity, no extra mounts).
3. **Given** a directory with a valid `.kekkai.yaml`, **When** the user runs
   `kekkai up`, **Then** no such warning appears and behavior is unchanged
   from today.

---

### User Story 2 - Empty or fully commented configuration file works (Priority: P1)

A user runs `kekkai init`, looks at the starter file, changes nothing (every
line is commented out), and runs `kekkai up`. The sandbox starts with defaults.
Likewise, a hand-created empty `.kekkai.yaml` works instead of failing with a
parse error.

**Why this priority**: Directly coupled to the commented-out starter template:
`init` followed immediately by `up` must work. Also fixes the existing failure
where an empty file aborts with a parse error (`parse ...: EOF`).

**Independent Test**: Create an empty `.kekkai.yaml`, run `kekkai up`; then
replace it with a comments-only file and run again. Both start with defaults
and no parse error.

**Acceptance Scenarios**:

1. **Given** a zero-byte `.kekkai.yaml`, **When** the user runs `kekkai up`,
   **Then** the sandbox starts with all-default settings and no error.
2. **Given** a `.kekkai.yaml` containing only comments, **When** the user runs
   `kekkai up`, **Then** the sandbox starts with all-default settings and no
   error.
3. **Given** an empty or comments-only file is present, **When** the user runs
   `kekkai up`, **Then** the missing-file warning does NOT appear (the file is
   the explicit opt-in; its emptiness means "defaults, please").

---

### User Story 3 - Starter template is fully commented out (Priority: P2)

A user runs `kekkai init` and opens the generated `.kekkai.yaml`. Every setting,
including the ones that previously appeared active (node version, claude
version, claude args), is a commented-out line showing its default value. The
file documents all options without activating any of them.

**Why this priority**: Aligns the starter file with "omit for default"
everywhere: uncommenting a line is the single gesture for changing behavior,
and the file never pins a default that later drifts from the built-in.

**Independent Test**: Run `kekkai init` in a fresh directory, verify the
generated file contains no active (uncommented) configuration key, then run
`kekkai up` and verify default behavior.

**Acceptance Scenarios**:

1. **Given** a fresh directory, **When** the user runs `kekkai init`, **Then**
   the generated `.kekkai.yaml` contains only comments and blank lines — no
   active keys.
2. **Given** that generated file, **When** the user runs `kekkai up`, **Then**
   the sandbox starts with all-default settings.
3. **Given** that generated file, **When** the user uncomments a documented
   setting and runs `kekkai up`, **Then** that setting takes effect.

---

### Edge Cases

- Both `.kekkai.yml` and `.kekkai.yaml` present: still an error ("remove one"),
  unchanged from today.
- Configuration file present but malformed (real syntax error, not empty):
  still a validation/parse error — absence is fine, brokenness is not.
- Warning must not interfere with machine-readable or piped output: it goes to
  the error stream, and the yellow color is disabled when that stream is not a
  terminal (plain text remains).
- `kekkai init` when a configuration file already exists: still refuses,
  unchanged.
- Other subcommands (`down`, `shell`, `ps`, `prune`): unaffected — they do not
  read the configuration file today and gain no new requirement.
- The removed hard error ("no .kekkai.yaml found, run 'kekkai init'") must not
  linger anywhere in documentation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `kekkai up` in a directory without a configuration file MUST
  start a sandbox using the built-in defaults for every setting.
- **FR-002**: In that case, `kekkai up` MUST print exactly one warning line
  before startup output: it states no configuration file was found, defaults
  are in use, and names `kekkai init` as the way to customize. The warning goes
  to the error stream and is rendered in yellow when that stream is a terminal
  (plain text otherwise).
- **FR-003**: A present configuration file that is empty or contains only
  comments MUST be treated as "all defaults": no error, no missing-file
  warning.
- **FR-004**: When a configuration file with at least one active key is
  present, behavior MUST be unchanged from today (validation, legacy-key
  errors, no warning).
- **FR-005**: The conflict error when both `.kekkai.yml` and `.kekkai.yaml`
  exist MUST be preserved.
- **FR-006**: `kekkai init` MUST generate a starter file in which every
  configuration key is commented out, each showing its default or an example
  value; the generated file MUST parse cleanly and produce default behavior
  when used as-is.
- **FR-007**: The exit status of `kekkai up` MUST NOT change because the
  configuration file is absent (absence is not an error).
- **FR-008**: README.md and SPECIFICATION.md MUST be updated wherever they
  present the configuration file as required or show `kekkai init` as a
  mandatory first step: describe the file as optional, document the
  missing-file warning, and reflect the fully commented-out starter template.
  No stale "no .kekkai.yaml found" hard-error reference may remain.

### Key Entities

- **Configuration file** (`.kekkai.yaml` / `.kekkai.yml`): now optional;
  absent, empty, and comments-only all mean "built-in defaults". Its presence
  with active keys is the only way behavior deviates from defaults.
- **Missing-configuration warning**: one line, error stream, yellow on
  terminals; appears only when no configuration file exists at all.
- **Starter template** (written by `kekkai init`): fully commented-out
  documentation of every available setting and its default.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new user can go from "installed" to "sandbox running" with a
  single command in an existing project directory (zero setup steps).
- **SC-002**: Running without a configuration file yields a sandbox whose
  behavior is indistinguishable from one started with an explicit all-defaults
  file, verified across image, network, git, and mount behavior.
- **SC-003**: The missing-configuration warning appears in 100% of runs without
  a configuration file and never when one exists (empty included).
- **SC-004**: `kekkai init` immediately followed by `kekkai up` succeeds with
  default behavior and no warning or error.
- **SC-005**: All previously valid configuration files keep working with
  byte-identical behavior (no regression for configured projects).

## Assumptions

- Defaults are the safest posture kekkai has (egress firewall on, nothing
  granted), so allowing unconfigured runs does not weaken the threat model;
  the container remains the security boundary.
- The warning on every unconfigured run is acceptable (no suppression flag or
  "don't show again" state; kekkai keeps zero persistent state).
- A misnamed file (e.g. `kekkai.yaml` without the leading dot) is out of scope;
  it is treated as absent, same as today's error path would not find it.
- The warning color follows the common convention: colored only when the output
  stream is a terminal, plain otherwise; no new configuration to control it.
