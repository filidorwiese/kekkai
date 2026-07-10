# Feature Specification: Pre-Build Node Version Existence Check

**Feature Branch**: `017-prebuild-node-check`

**Created**: 2026-07-10

**Status**: Draft

**Input**: User description: "validate image.node_version existence before docker build against nvm's source of truth (nodejs.org dist index): lts always valid without lookup; numeric selectors checked remotely with tri-state semantics (confirmed absent aborts pre-build naming image.node_version, confirmed present builds, inconclusive proceeds). Also remove the in-Dockerfile ERROR wrap from the nvm install step - it causes confusion; the raw nvm error suffices as the fallback when the probe was inconclusive"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A typo'd Node version fails in seconds, not minutes (Priority: P1)

A user sets `image.node_version: "99"` (well-formed, nonexistent) and runs `kekkai up`. Instead of watching a multi-minute image build churn through apt and the Node installer only to die mid-build, kekkai checks the version against the same published Node release index the installer itself consults, and aborts before any build starts — with an error naming `image.node_version` as the knob and where to see available versions.

**Why this priority**: This is the feature: it converts the worst failure experience left after feature 014 (long build, then an error buried in build output) into an instant, config-shaped error — using the installer's own source of truth, so the check can never disagree with what the build would do.

**Independent Test**: Set `node_version: "99"`, run `kekkai up` with network available: it exits within seconds, before any docker build activity, naming `image.node_version`. Set `"22"`: the build proceeds.

**Acceptance Scenarios**:

1. **Given** `image.node_version: "99"` (or `"22.99"` / `"22.99.0"`) and a reachable release index, **When** `kekkai up` needs to build, **Then** it aborts before any docker build starts with an error naming `image.node_version`, the offending value, and pointing at the list of available versions.
2. **Given** `image.node_version: "22"` (existing major) and a reachable index, **When** `kekkai up` needs to build, **Then** the check passes and the build starts normally.
3. **Given** `image.node_version: lts`, **When** `kekkai up` needs to build, **Then** no remote check happens at all — `lts` is valid by construction.
4. **Given** a full pin like `"22.11.0"` that exists, **When** the check runs, **Then** it passes; partial selectors (`"22"`, `"22.11"`) pass when any published version matches them — the same matching rule the installer applies.

---

### User Story 2 - The check never blocks a build it can't judge (Priority: P2)

A user runs `kekkai up` while the release index host is slow, unreachable, or returns garbage. The check silently steps aside and the build proceeds exactly as today — where an actually-bad version still fails with the installer's own "version not found" message. A cached image (`up` without a pending build) never triggers the check at all.

**Why this priority**: The check is a courtesy accelerator, not a gate. If it could block builds on flaky networks it would be a reliability regression worse than the problem it solves.

**Independent Test**: Point the check at an unreachable index host; `kekkai up` with a valid version proceeds to build with no error and no noticeable extra delay beyond the bounded probe. Run `up` with a cached image: zero requests to the index.

**Acceptance Scenarios**:

1. **Given** the index is unreachable (timeout, DNS failure, non-success response, malformed data), **When** `kekkai up` needs to build, **Then** the build proceeds; no error or warning about the check is shown.
2. **Given** the image for the current config already exists, **When** `kekkai up` runs, **Then** no index request is made (check only runs when a build is imminent).
3. **Given** an inconclusive check AND a nonexistent version, **When** the build runs, **Then** it fails at the Node install step with the installer's own error message (e.g. "Version '99' not found - try `nvm ls-remote`") — no kekkai-added wrapper text.

---

### User Story 3 - The confusing in-build error wrapper is gone (Priority: P3)

A user reading build output (or the Dockerfile) no longer sees kekkai's bolted-on `ERROR: Node version ... not found - check image.node_version` echo around the Node install step. When the install fails (now only reachable when the pre-build check was inconclusive), the output shows the installer's native error once, not two stacked errors saying different things.

**Why this priority**: Cleanup rider on US1 — the wrapper was the stopgap for exactly the failure US1 now catches earlier; keeping both would double-report. It has caused confusion and its removal simplifies the build step.

**Independent Test**: Inspect the rendered image recipe: no ERROR-wrap around the install step. Force a build with a nonexistent version and the index unreachable: build output contains the installer's single native error.

**Acceptance Scenarios**:

1. **Given** the rendered image build recipe, **When** inspected, **Then** the Node install step contains no kekkai-added failure echo/wrapper.
2. **Given** a failing install (inconclusive-check path), **When** the build errors, **Then** exactly one error message appears — the installer's native one.

---

### Edge Cases

- Selector exists in the index but its download artifacts are missing for the build architecture: the check passes (it validates version existence, not per-arch artifacts); the build fails with the installer's message — same as today, unchanged and acceptable.
- Very slow index responses: the check has a bounded time budget (same order as kekkai's other best-effort lookups); when exceeded it counts as inconclusive and the build proceeds.
- The check runs on the host network, the build on the docker daemon's network: a split where one side is reachable and the other isn't resolves safely in both directions (inconclusive → proceed; confirmed-present → build may still fail with the installer's network error, which is honest).
- `lts` newly released between check and install: irrelevant — `lts` never hits the check and always resolves at install time.
- New Node release published moments before the check: a just-published version might be present in the index and installable — no staleness problem in the abort direction as long as absence is judged against fresh index data.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When (and only when) `kekkai up` is about to build an image, numeric `image.node_version` selectors MUST be checked for existence against the Node release index that the in-image installer itself consults (its configured source of truth, not a proxy).
- **FR-002**: The literal `lts` selector MUST NOT trigger any remote check (always valid by construction).
- **FR-003**: Matching MUST mirror the installer's resolution rule: a full version matches exactly; major and major.minor selectors match when any published version falls under them.
- **FR-004**: Confirmed absence (index retrieved successfully, no match) MUST abort before any docker build activity, with an error naming `image.node_version`, the configured value, and pointing the user at the list of available Node versions.
- **FR-005**: Any inconclusive outcome (network failure, timeout, non-success response, unparseable data) MUST let the build proceed silently — the check never blocks offline or degraded use, and never prints noise on the happy path.
- **FR-006**: The check MUST have a bounded time budget consistent with kekkai's existing best-effort remote lookups (single-digit seconds); no retries.
- **FR-007**: `kekkai up` runs that reuse a cached image MUST NOT contact the index at all.
- **FR-008**: The in-image build recipe MUST NOT contain the kekkai-added error wrapper around the Node install step; a failing install surfaces the installer's native error only. (Note: removing the wrapper changes the rendered recipe and therefore triggers a one-time image rebuild on next `up` — expected.)
- **FR-009**: The pre-build check failure is a host-side error before docker involvement; its message is the single kekkai-owned surface for "version does not exist" (replacing the in-build wrapper's role from feature 014).

### Key Entities

- **Node release index**: the published list of all Node versions (the installer's source of truth); consulted read-only, at most once per build-triggering `up`.
- **Existence check outcome (tri-state)**: confirmed-present → build; confirmed-absent → abort with config-shaped error; inconclusive → build (installer error remains the fallback).
- **Node install build step**: loses the kekkai error wrapper; otherwise unchanged.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With a reachable index, a nonexistent version (`99`, `22.99`, `22.99.0`) aborts `kekkai up` in under ~15 seconds with zero docker build activity and an error naming `image.node_version`.
- **SC-002**: Existing selectors (`lts`, a real major, a real full pin) all proceed to build; `lts` provably makes no index request.
- **SC-003**: With the index unreachable, a build-triggering `up` proceeds with no check-related output; total added latency stays within the bounded probe budget.
- **SC-004**: Cached-image `up` runs make zero index requests.
- **SC-005**: The rendered build recipe contains no ERROR wrapper; a forced in-build install failure shows exactly one (native) error message.

## Assumptions

- The installer's default release index (nodejs.org dist index, as consulted by the pinned nvm's `ls-remote`) is the single source checked; custom mirrors are out of scope (kekkai never configures one).
- Tri-state best-effort semantics deliberately mirror the deleted node:* Docker Hub probe from feature 014 — same constitution rationale: an unreachable registry must never block use.
- Checking version existence only (not per-architecture artifact availability) is sufficient; arch-specific gaps remain an in-build installer error.
- The check lives at the build-imminent moment, not in config validation: validation stays offline and one-pass (constitution), and cached-image runs stay network-free.
- `SPECIFICATION.md` §6.1 (bake/build pre-check paragraph) and the feature-014 Dockerfile contract note are amended in the same commit (constitution I).
- Out of scope: caching index responses, validating `claude.version` the same way, offline version lists.
