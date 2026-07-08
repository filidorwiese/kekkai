# Feature Specification: Yellow Update Notice

**Feature Branch**: `008-yellow-update-notice`

**Created**: 2026-07-08

**Status**: Draft

**Input**: User description: "self-update message on `kekkai up` should also be yellow"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Update notice stands out in the startup output (Priority: P1)

A user with an older kekkai runs `kekkai up` in a terminal. Among the
startup lines (image reuse, firewall init), the update notice is rendered
in yellow — the same advisory color the missing-config warning already
uses — so it catches the eye before the screen hands over to Claude.

**Why this priority**: This is the whole feature: the notice exists to be
seen, and today it drowns in same-colored startup output. "Also yellow"
aligns it with the established advisory color.

**Independent Test**: Run an outdated build's `up` in a terminal and
observe the notice in yellow; the notice text itself is unchanged.

**Acceptance Scenarios**:

1. **Given** an installed version older than the latest release and output
   going to a terminal, **When** the user runs `kekkai up`, **Then** the
   update notice line renders in yellow and its text is exactly the same
   as before.
2. **Given** the same situation but with the no-color convention active
   (`NO_COLOR` set), **When** the user runs `kekkai up`, **Then** the
   notice appears as plain text.
3. **Given** the same situation but output piped or redirected, **When**
   the user runs `kekkai up`, **Then** the notice appears as plain text
   with no color codes.

---

### Edge Cases

- Output captured by scripts/CI (not a terminal): plain text, no escape
  codes — same gating as the missing-config warning.
- All silence conditions from the update-notice feature (current version,
  dev build, endpoint unreachable, check unfinished, failed `up`) are
  untouched: color changes rendering, never when the notice appears.
- Both advisories in one run (no config file + update available): each
  renders yellow by its own stream's rules; they remain separate lines.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `kekkai up` update notice MUST render wrapped in yellow
  when its output stream is a terminal and `NO_COLOR` is unset — the same
  conditions the missing-config warning uses.
- **FR-002**: The notice MUST remain plain text when the stream is not a
  terminal or `NO_COLOR` is set.
- **FR-003**: The notice's text, stream, position in the startup sequence,
  silence conditions, and effect on exit status MUST be unchanged.
- **FR-004**: The two advisory messages (update notice, missing-config
  warning) MUST use the identical yellow so the advisory color reads as
  one convention.
- **FR-005**: The living specification's update-notice description MUST be
  amended for the yellow rendering in the same change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a terminal session with an update available, the notice
  is visually distinct (yellow) from the surrounding default-colored
  startup lines.
- **SC-002**: Piped/redirected output of the same run contains the notice
  with zero escape sequences.
- **SC-003**: A byte-for-byte comparison of the notice text (color
  stripped) before and after the change shows no difference.
- **SC-004**: All silence conditions of the update notice behave
  identically before and after (no new appearances, no lost ones).

## Assumptions

- "Also yellow" refers to the advisory yellow introduced for the
  missing-config warning (feature 006); consistency between the two is the
  intent.
- The notice keeps its current output stream; only rendering changes. The
  terminal check therefore applies to that stream, mirroring how the
  warning checks its own stream.
- The `NO_COLOR` convention adopted in feature 006 extends to every
  colored line kekkai prints, including this one.
- The historical feature 005 contract stays frozen; the living
  specification is where the rendering note lands (precedent: features
  004-007).
