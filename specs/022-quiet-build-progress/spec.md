# Feature Specification: Quiet Interactive Build Progress Without Custom Apt Repos

**Feature Branch**: `022-quiet-build-progress`

**Created**: 2026-08-20

**Status**: Draft

**Input**: User description: "Restore quiet interactive build output when no custom apt repos are configured — fix the UX regression from feature 020 where the build-output capture makes every build show step-by-step plain progress; capture only when `image.apt_repos` is non-empty, since the captured output feeds only the apt signature-failure hint."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Default builds show the compact progress display again (Priority: P1)

A user without `image.apt_repos` in their config (the default and overwhelmingly common case) runs kekkai from an interactive terminal and the image needs building. The build shows the container tooling's compact, auto-collapsing progress display — as it did before feature 020 — instead of printing every build step line-by-line.

**Why this priority**: This is the regression being fixed. Since 020, every user's 2–4 minute build scrolls hundreds of plain-progress lines, regardless of whether they use the feature that needed output capture.

**Independent Test**: From an interactive terminal, trigger a build with a repo-less config and observe the compact progress display; compare against a pre-020 release behaving identically.

**Acceptance Scenarios**:

1. **Given** a config without `apt_repos` (or no config) and an interactive terminal, **When** a build is triggered, **Then** the build renders the compact progress display, not step-by-step plain output.
2. **Given** the same setup, **When** the build succeeds or fails, **Then** the output shown is exactly what the container tooling itself would print — kekkai adds nothing and hides nothing.
3. **Given** a repo-less config and the verbose flag, **When** a build is triggered, **Then** step-by-step plain progress is shown (verbose keeps its meaning).

---

### User Story 2 - Repo users keep the signature-failure hint (Priority: P1)

A user with `image.apt_repos` configured triggers a build. Build output continues to be captured so that, on an apt signature failure, the hint naming the offending repo entry and pointing at `key_url` still appears — the feature 020 contract, byte-for-byte unchanged.

**Why this priority**: The fix must not trade away the diagnostic the capture exists for; both stories together are the whole feature and must ship as one.

**Independent Test**: Re-run feature 020's signature-failure scenario (repo needing a key, `key_url` omitted) and verify the hint still appears after the build error with unchanged wording and exit status.

**Acceptance Scenarios**:

1. **Given** a config with at least one `apt_repos` entry whose key cannot be verified, **When** the build fails with an apt signature error, **Then** the hint naming the entry appears after the build error, exactly per the 020 contract (never replacing the error; exit status unchanged).
2. **Given** a config with `apt_repos` and a build that fails for a non-signature reason, **When** the build fails, **Then** no hint is printed (unchanged from 020).
3. **Given** a config with `apt_repos`, **When** a build runs, **Then** build output remains visible on the terminal while being captured (step-by-step display in this mode is accepted).

---

### Edge Cases

- `apt_repos: []` (empty list): equivalent to absent — quiet mode, no capture (consistent with 020's `[]` ≡ absent semantics).
- Non-interactive invocation (output piped/redirected) without repos: the container tooling applies its own non-TTY fallback, exactly as it would when invoked directly; kekkai imposes nothing.
- Signature-type apt failure on a repo-less config: no hint — accepted trade-off; such a config got no hint before 020 either, and an entry-naming hint has no entry to name.
- Existing built images: this is a behavior-of-output fix only; no image identity input changes, so no config rebuilds and the offline-fallback labels stay valid.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When `image.apt_repos` is empty or absent, the image build MUST attach directly to the user's terminal so the container tooling selects its interactive compact progress display; no build-output capture occurs.
- **FR-002**: When `image.apt_repos` has at least one entry, build output MUST continue to be captured (while still streaming to the terminal) so the apt signature-failure hint can be produced per the feature 020 contract.
- **FR-003**: The feature 020 hint contract is unchanged in repo mode: same trigger markers, same attribution, same wording, printed to stderr after the build error, never replacing it, exit status unchanged. Builds without repos never print the hint.
- **FR-004**: The verbose flag MUST force step-by-step plain progress in both modes, as today.
- **FR-005**: The fix MUST NOT change any image identity input: no rendered-template change, no identity/label formula change — existing images are reused with zero rebuilds after upgrading to this fix.
- **FR-006**: No new config keys, flags, or schema changes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For configs without `apt_repos`, an interactive build's scrollback is the compact progress display (a handful of lines on screen), not the hundreds of step lines of plain progress — matching pre-020 behavior exactly.
- **SC-002**: Feature 020's signature-failure validation scenario passes unchanged: 100% of the hint wording, placement, and exit-status expectations hold.
- **SC-003**: After upgrading, 100% of previously built images are reused — zero rebuilds attributable to this fix.
- **SC-004**: Verbose builds show step-by-step progress in both modes.

## Assumptions

- The compact-vs-plain rendering choice belongs to the container tooling's own terminal detection; kekkai's obligation is only to not stand between the tooling and the terminal when it has no need to (no repos), and to accept plain rendering as the cost of capture when it does (repos configured).
- Forcing the compact display while capturing (pseudo-terminal tricks or forcing the interactive renderer through a pipe) was considered and rejected: it either requires non-standard-library machinery or fills the capture with control codes that can truncate the very error text the hint scans for.
- The trade-off in Edge Cases (no hint for repo-less signature failures) is accepted by the feature owner; it restores the exact pre-020 behavior for that case.
- Constitution alignment: no new surface (Principle III); image hash from bake-time inputs only — untouched (Constraint); container boundary untouched (Principle II); validation end-to-end against a real daemon including a rebuilt 020 scenario (Principle IV).
