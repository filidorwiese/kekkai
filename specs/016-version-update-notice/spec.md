# Feature Specification: Update Notice on `kekkai version`

**Feature Branch**: `016-version-update-notice`

**Created**: 2026-07-10

**Status**: Draft

**Input**: User description: "on `kekkai version` also mention when there is a new update available and how to update (in yellow)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Version check tells me I'm outdated (Priority: P1)

A user runs `kekkai version` to see what they have installed. Besides the version number, kekkai tells them — in the same yellow advisory style used elsewhere — when a newer release exists and exactly how to get it (`kekkai self-update`). Today this hint only appears during `kekkai up`; a user who checks their version explicitly is the one most interested in knowing it is stale, and currently gets nothing.

**Why this priority**: This is the feature. `kekkai version` is the natural "am I up to date?" gesture; answering it fully closes the loop with the existing self-update machinery.

**Independent Test**: Run a binary whose version is older than the latest published release: `kekkai version` prints the version plus a yellow one-liner naming the newer release and the update command. Run a current binary: only the version prints.

**Acceptance Scenarios**:

1. **Given** an installed release older than the latest published release, **When** the user runs `kekkai version`, **Then** the version number prints followed by one advisory line stating a newer version exists (naming both versions) and that `kekkai self-update` upgrades.
2. **Given** the advisory prints to a color-capable terminal with color output enabled, **When** it appears, **Then** it is rendered in the same yellow used by the existing `up` update notice; with color disabled (NO_COLOR or non-terminal), the same text appears unstyled.
3. **Given** the installed version equals (or is ahead of) the latest release, **When** the user runs `kekkai version`, **Then** exactly the version number prints — no advisory, no error.
4. **Given** the same stale binary, **When** the user runs `kekkai up`, **Then** the notice text shown there and the one from `kekkai version` are identical (one message, two surfaces).

---

### User Story 2 - Scripts and offline use are unaffected (Priority: P2)

A user pipes `kekkai version` into a script, or runs it without network access. The machine-readable output (the version number) stays exactly as before on standard output; the advisory never contaminates it. Offline or on any lookup failure, the command behaves as if no update exists — same output as today, no error, no long hang.

**Why this priority**: `kekkai version` is the one command whose output gets parsed. Breaking `$(kekkai version)` or making it hang offline would turn a courtesy into a regression.

**Independent Test**: `kekkai version | cat` (non-terminal) yields the version as the only stdout content even when an update exists; with network blocked the command completes without error in bounded time.

**Acceptance Scenarios**:

1. **Given** an update exists, **When** stdout is captured (`v=$(kekkai version)`), **Then** the captured value is exactly the version string — the advisory travels on the diagnostic stream, not stdout.
2. **Given** no network connectivity (or the release lookup fails for any reason), **When** `kekkai version` runs, **Then** it prints the version and exits successfully within a bounded time — no error text, no advisory.
3. **Given** a development build (no release version stamped), **When** `kekkai version` runs, **Then** no lookup happens and no advisory appears.

---

### Edge Cases

- Lookup succeeds but is slow: the command must not feel broken — the version itself prints immediately; the total run stays within the bounded lookup time already used by the existing notice machinery.
- Rate-limited or malformed release data: treated as "no update known" — silent, exit code unchanged.
- User is ahead of the latest release (local build of a newer tag): no advisory (consistent with the existing notice, which only speaks when the release is strictly newer).
- `kekkai self-update` advertised while the binary location is not writable: out of scope — `self-update` itself already explains that situation when run.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `kekkai version` MUST print the installed version on standard output exactly as today, as the first output, unconditionally.
- **FR-002**: When a strictly newer release exists, `kekkai version` MUST additionally print one advisory line naming the installed and latest versions and the upgrade command (`kekkai self-update`).
- **FR-003**: The advisory text MUST be identical to the update notice shown at `kekkai up` — one message definition, shared by both surfaces, so the advertised version is always what self-update would install.
- **FR-004**: The advisory MUST be styled in the established yellow advisory convention: yellow only when the destination stream is a terminal and color is not disabled (NO_COLOR); plain text otherwise.
- **FR-005**: The advisory MUST NOT be written to standard output; captured/piped stdout contains only the version string.
- **FR-006**: On dev builds, lookup failure, network absence, equal or ahead versions: no advisory, no error message, unchanged exit code, completion within the bounded lookup time already used by the existing notice (no new timeouts introduced).
- **FR-007**: The release lookup MUST reuse the same source of truth as `kekkai self-update` and the `up` notice (no second lookup mechanism).

### Key Entities

- **Update notice (existing)**: the single message stating current → latest version and the upgrade command; gains a second display surface.
- **`kekkai version` output contract**: stdout = version string only; advisory (when any) on the diagnostic stream, yellow per the advisory convention.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With a stale binary and working network, 100% of `kekkai version` runs show the version plus the yellow advisory naming the newer release and `kekkai self-update`.
- **SC-002**: `$(kekkai version)` captures exactly the version string in all cases (update available, current, offline).
- **SC-003**: With no network, `kekkai version` completes without error within the existing bounded lookup time (≤ the 10-second cap the notice machinery already uses; typically instantly when the failure is immediate).
- **SC-004**: The notice text shown by `kekkai version` and by `kekkai up` for the same binary/release pair is byte-identical.
- **SC-005**: Current or dev builds produce byte-identical `kekkai version` output to the pre-feature behavior.

## Assumptions

- The existing notice machinery (specs/005 update notice, specs/008 yellow styling) is reused as-is: same message, same yellow convention, same silent-failure policy; this feature only adds the second surface.
- Stderr is the diagnostic stream for the advisory (matching the missing-config advisory convention), keeping stdout parseable.
- The version number itself stays plain (never yellow), and remains the first line regardless of whether the advisory appears.
- A bounded wait (up to the existing lookup timeout) on `kekkai version` is acceptable; no async/cache layer is introduced for this courtesy check.
- `SPECIFICATION.md` §3 (CLI) gains the version-command advisory in the same commit as the change (constitution I).
- Out of scope: update checks on any other subcommand, notice caching/throttling, changing `self-update` behavior.
