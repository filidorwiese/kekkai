# Feature Specification: Rename `kekkai watch` to `kekkai traffic`

**Feature Branch**: `013-rename-watch-traffic`

**Created**: 2026-07-09

**Status**: Draft

**Input**: User description: "rename `kekkai watch` to `kekkai egress-log`"

## Clarifications

### Session 2026-07-09

- Q: Final command name — `egress-log` as originally described? → A: `kekkai traffic` (supersedes `egress-log`; spec updated throughout)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect egress under the new name (Priority: P1)

A developer wants to see the live egress log of their running sandbox. They run `kekkai traffic` and get exactly the experience `kekkai watch` gave them: the labeled ALLOW/BLOCK stream with DNS lines, the same errors, the same exit behavior. Every place the tool presents its own commands (help output, error hints, documentation) says `traffic`; nothing anywhere still says `watch`.

**Why this priority**: This is the whole feature — the command must work under its new name with zero behavior change.

**Independent Test**: With a sandbox running, run `kekkai traffic`, generate traffic, and verify the stream behaves per the feature 010 contract; check `kekkai help` and user docs mention only `traffic`.

**Acceptance Scenarios**:

1. **Given** a running sandbox, **When** the developer runs `kekkai traffic`, **Then** the live labeled egress stream appears exactly as specified for feature 010 (same line formats, banner, exit codes, error strings).
2. **Given** any kekkai-produced text (help output, usage line, stale-image hint, docs), **When** the developer reads it, **Then** the command is called `traffic` and no reference to a `watch` command remains.
3. **Given** the new name, **When** the developer runs `kekkai traffic extra-arg`, **Then** the usage error names the new command and exits nonzero.

---

### User Story 2 - Old name fails loudly, not silently (Priority: P2)

A developer who remembers the old name runs `kekkai watch`. The tool treats it as an unknown command — same as any other typo — showing the standard unknown-command error plus the usage overview, where the developer immediately sees `traffic`.

**Why this priority**: The rename ships before any release containing `watch`, so no user base depends on the old name; a plain unknown-command error with visible help is sufficient recovery.

**Independent Test**: Run `kekkai watch` and verify the standard unknown-command error and nonzero exit, with the usage text listing `traffic`.

**Acceptance Scenarios**:

1. **Given** any directory, **When** the developer runs `kekkai watch`, **Then** kekkai reports an unknown command, prints usage (which lists `traffic`), and exits nonzero.

---

### Edge Cases

- Internal artifacts named after the old command (e.g. historical feature 010 documents) are records of past decisions and stay untouched; only living design/user documents change.
- Error messages that mention the command by name (e.g. the stale-image hint "sandbox image predates ...") must carry the new name.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST expose the feature 010 egress-inspection command as `traffic`; behavior, output formats, exit codes, and guarantees remain exactly as specified there.
- **FR-002**: `watch` MUST no longer be a recognized command; invoking it yields the standard unknown-command error and nonzero exit.
- **FR-003**: All user-facing text produced by the tool (help/usage, error hints that name the command) MUST refer to `traffic`.
- **FR-004**: All living documentation (design spec, user README, the feature 010 CLI contract's invocation name) MUST be updated to `traffic` in the same change.
- **FR-005**: The rename MUST NOT alter the security posture: no firewall, sandbox, or container behavior changes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every feature 010 acceptance scenario passes unchanged when driven via `kekkai traffic` instead of `kekkai watch`.
- **SC-002**: `kekkai watch` fails with the standard unknown-command error, and the accompanying usage text shows `traffic` — 100% of invocations.
- **SC-003**: A text search of the tool's output surfaces and living docs finds zero remaining references to a `watch` command.

## Assumptions

- Hard rename, no alias or deprecation period: feature 010 has not shipped in any release, so nobody depends on `watch`. Minimal-surface principle favors one name over two.
- `traffic` is the noun alternative that feature 010 research (R7) explicitly considered and rejected for verb-style consistency (`up`, `shell`, `exec`); the user's clarified choice supersedes that rejection. Recorded here so the naming decision has a home.
- Historical spec-kit artifacts under `specs/010-inspect-egress/` remain as-is except the CLI contract, which downstream work treats as the living contract for exact strings.
- One-time image rebuild accepted: firewall script comments naming the command are updated for accuracy (comment-only diff, no rule changes), and the script is a bake-time input — next `kekkai up` rebuilds cheaply (plan research R2).
