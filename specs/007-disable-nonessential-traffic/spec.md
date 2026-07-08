# Feature Specification: Disable Nonessential Traffic

**Feature Branch**: `007-disable-nonessential-traffic`

**Created**: 2026-07-08

**Status**: Draft

**Input**: User description: "Set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 inside the sandbox and remove any mention of statsig.anthropic.com"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Sandbox sends no nonessential traffic (Priority: P1)

A privacy-conscious user runs `kekkai up`. Claude Code inside the sandbox
operates with nonessential traffic disabled: no telemetry, no error
reporting, no auto-update attempts. Only traffic essential for Claude to
function (the Anthropic API) leaves the sandbox on the builtin allowance.

**Why this priority**: This is the feature's core value: the sandbox
becomes quieter and more private by default, and the built-in egress
surface shrinks to the single destination Claude actually needs.

**Independent Test**: Start a sandbox and inspect the container
environment: the nonessential-traffic switch is present and active.
Claude Code starts and operates normally.

**Acceptance Scenarios**:

1. **Given** any project, **When** the user runs `kekkai up`, **Then** the
   sandbox environment contains `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
   and Claude Code starts and functions normally.
2. **Given** a running sandbox, **When** the user inspects its
   configuration from the host, **Then** the switch is visible in the
   container's environment.

---

### User Story 2 - Telemetry endpoint leaves the builtin firewall allowance (Priority: P1)

With telemetry disabled, the sandbox firewall no longer needs to allow the
telemetry endpoint (statsig.anthropic.com). The builtin always-allowed set
shrinks accordingly; everything else about the firewall is unchanged.

**Why this priority**: Defense in depth. An allowed destination that
nothing legitimate uses is pure exfiltration surface; removing it
completes the privacy story of US1.

**Independent Test**: Start a sandbox and verify the firewall's allowed
set no longer contains the telemetry endpoint while the Anthropic API
endpoint remains allowed and the startup verification probes still pass.

**Acceptance Scenarios**:

1. **Given** a default sandbox, **When** the firewall initializes, **Then**
   its builtin allowed set contains the Anthropic API endpoint but not the
   telemetry endpoint, and the startup firewall verification passes.
2. **Given** a default sandbox, **When** a connection to the telemetry
   endpoint is attempted from inside, **Then** it is blocked like any
   other non-allowed destination.

---

### User Story 3 - No stale telemetry references in docs or starter config (Priority: P2)

A user reading the README, the specification, or a freshly generated
starter config finds no mention of the telemetry endpoint. The documented
builtin allowance is the Anthropic API only.

**Why this priority**: Docs describing an allowance that no longer exists
would misinform security-sensitive users; the project treats its
specification as the source of truth, so it must be amended in the same
change.

**Independent Test**: Search the user-facing documentation and a freshly
generated starter config for the telemetry endpoint's name; expect zero
matches.

**Acceptance Scenarios**:

1. **Given** the updated project, **When** searching README, the
   specification, and `kekkai init` output for "statsig", **Then** there
   are no matches.
2. **Given** a fresh starter config, **When** the user reads the network
   section comments, **Then** the always-allowed destination listed is the
   Anthropic API only.

---

### Edge Cases

- A user pins an older Claude Code version that predates or ignores the
  nonessential-traffic switch: its telemetry attempts are simply blocked
  by the firewall (US2); Claude continues to function.
- A user explicitly sets `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` (or a
  related telemetry variable) in their config's `env` section: the user's
  explicit value wins, consistent with existing env layering. Re-enabled
  telemetry traffic is still subject to the firewall.
- A user who wants telemetry back can combine their env override with a
  network allowance for the telemetry endpoint (`allowed_domains`); no
  builtin re-enables it.
- Existing sandboxes/images built before this change: behavior updates on
  the next sandbox start/image rebuild; no migration needed (kekkai is
  stateless).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Every sandbox MUST start with
  `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` present in its environment
  by default.
- **FR-002**: A user-supplied value for the same variable in the config's
  `env` section MUST take precedence over the builtin default.
- **FR-003**: The firewall's builtin always-allowed destination set MUST
  no longer include statsig.anthropic.com; the Anthropic API endpoint
  remains allowed and required.
- **FR-004**: Firewall startup verification MUST continue to pass with the
  reduced builtin set.
- **FR-005**: All user-facing mentions of statsig.anthropic.com MUST be
  removed: README, the project specification, and the starter config
  generated by `kekkai init`.
- **FR-006**: The specification's builtin destination section MUST be
  amended in the same change as the code (constitution I and II: builtin
  destination set changes require a spec amendment).
- **FR-007**: All other sandbox behavior (mounts, user network config,
  limits, exit codes) MUST remain unchanged.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A default sandbox makes zero connections to the telemetry
  endpoint during a normal Claude Code session.
- **SC-002**: The builtin always-allowed destination set contains exactly
  one external destination: the Anthropic API endpoint.
- **SC-003**: A search for "statsig" across user-facing docs and freshly
  generated starter configs returns zero matches.
- **SC-004**: Claude Code starts and completes a normal interactive
  session in the sandbox with no user-visible errors attributable to
  disabled telemetry.
- **SC-005**: Sandbox startup succeeds at the same rate as before the
  change (firewall verification unaffected).

## Assumptions

- `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` is the documented Claude
  Code switch that disables telemetry, error reporting, auto-update
  checks, and similar non-critical traffic; the Anthropic API remains the
  only endpoint Claude needs to function.
- statsig.anthropic.com exists in the builtin allowance solely for Claude
  Code telemetry; nothing else in the sandbox depends on it.
- Disabling the auto-updater inside the sandbox is desirable, not a
  regression: the Claude Code version is baked into the image and managed
  by kekkai (`claude.version`), so in-container self-updates were never
  meaningful.
- The user's env section already layers over builtin env values, so
  FR-002 follows the existing precedence model rather than introducing a
  new one.
- Historical feature specs under `specs/` are frozen records and keep
  their statsig mentions; "user-facing docs" means README, the living
  specification, and generated starter configs.
