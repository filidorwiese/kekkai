# Feature Specification: Verify Claude Binary at Image Build Time

**Feature Branch**: `023-verify-claude-binary`

**Created**: 2026-08-24

**Status**: Draft

**Input**: User description: "Verify the claude native binary at image build time. Since claude-code 2.x, the npm package is a thin wrapper whose ~342MB native binary comes from a platform-specific optionalDependency (e.g. @anthropic-ai/claude-code-linux-x64); npm treats a failed optional-dependency download as non-fatal (warning, exit 0), and the wrapper's postinstall also exits 0 when the platform package is absent. A transient registry/CDN failure during docker build therefore produces a 'successful' image whose claude command aborts at sandbox startup with 'claude native binary not installed' — and because the image is content-hash tagged, every subsequent kekkai up reuses the broken image without rebuilding. The build must instead fail loudly when the installed claude cannot actually run: after npm install of claude-code in the image build, verify the claude installation is functional, so a silently-skipped optional dependency aborts the build instead of baking a broken image. A failed verification should surface a clear, actionable error to the user running kekkai up, and ideally the build step should retry the install once before giving up, since the root cause is transient."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Broken install aborts the build (Priority: P1)

A user runs `kekkai up` in their project, triggering an image build. During the build, the download of Claude's platform-native binary fails transiently (registry/CDN hiccup) while everything else succeeds. Instead of producing an image that only fails at sandbox startup, the build itself fails with a clear message telling the user the Claude installation is incomplete and that retrying usually fixes it.

**Why this priority**: This is the defect itself. Today the failure is silent: the build "succeeds", the broken image is tagged by content hash, and every subsequent `kekkai up` reuses it — the user is stuck with a sandbox that aborts at startup and no hint that a rebuild would help. Failing the build converts a persistent broken state into a visible, retryable one.

**Independent Test**: Simulate the missing native binary during a build (e.g. force the platform package to be absent after install) and verify the build aborts with the actionable message; verify no image is tagged for that build.

**Acceptance Scenarios**:

1. **Given** an image build in which the Claude wrapper package installs but its platform-native binary is absent, **When** the build reaches the verification step, **Then** the build fails and no image with that tag exists afterwards.
2. **Given** such a failed build, **When** the user reads the `kekkai up` output, **Then** it states that the Claude native binary failed to install, that the cause is usually a transient network failure, and that rerunning `kekkai up` retries the build.
3. **Given** a build in which the Claude installation is complete and functional, **When** the verification step runs, **Then** the build succeeds exactly as before with no user-visible change.

---

### User Story 2 - Transient failure self-heals via retry (Priority: P2)

A user runs `kekkai up` and the first attempt to install Claude inside the build hits a transient download failure. The build retries the installation once before giving up, so in the common case the user never sees a failure at all.

**Why this priority**: The root cause is transient by nature; a single in-build retry resolves most occurrences without user involvement. Valuable, but secondary to no longer baking broken images.

**Independent Test**: Can be tested by observing that the build's install step performs a second install attempt when the first verification fails, and that the build succeeds when the second attempt completes.

**Acceptance Scenarios**:

1. **Given** a first install attempt that leaves the native binary missing, **When** the build continues, **Then** a second install attempt is made before the build is declared failed.
2. **Given** both attempts leave the native binary missing, **When** the build finishes, **Then** it fails per User Story 1.

---

### User Story 3 - Existing users rebuild cleanly, offline path unchanged (Priority: P3)

A user with previously built kekkai images upgrades kekkai. On the next `kekkai up`, the changed build recipe produces a new image hash, triggering one normal rebuild; nothing else about their workflow changes. A user whose registry lookup fails (offline-fallback path) continues to reuse their newest matching existing image with no new checks or failures.

**Why this priority**: Compatibility guardrail — the fix must not disturb image-hash semantics beyond the expected one-time rebuild, and must not add behavior to the path that never builds.

**Independent Test**: Run `kekkai up` with a pre-existing image built by the previous recipe and confirm exactly one rebuild is triggered by the hash change; run the offline-fallback path and confirm it behaves as today.

**Acceptance Scenarios**:

1. **Given** an image built with the previous recipe, **When** the user runs `kekkai up` after upgrading, **Then** a rebuild is triggered once by the recipe change and subsequent runs reuse the new image.
2. **Given** the offline-fallback path (registry unreachable, matching image exists), **When** the user runs `kekkai up`, **Then** the existing image is reused with no verification-related change in behavior.

---

### Edge Cases

- Verification must exercise the actual native binary, not just the wrapper: the wrapper resolves and exits 0 even when the binary is missing, which is exactly the false success being eliminated.
- Verification must not require network access: the build's later steps may run in environments where only the install step's downloads are expected, and a functional install must verify offline.
- The retry must re-attempt the actual download: if the first attempt cached a partial/failed state, the second attempt must still be able to succeed (a retry that trivially "succeeds" without fetching the binary would reintroduce the silent failure).
- If Claude changes its distribution mechanism again, the verification must fail safe: any state in which the installed `claude` command cannot run successfully fails the build, regardless of why.
- Broken images built before this feature are not detected retroactively; they are naturally replaced by the one-time rebuild the recipe change forces (User Story 3).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The image build MUST verify, after installing Claude, that the installed `claude` command is actually runnable (native binary present and executing successfully), and MUST fail the build when it is not.
- **FR-002**: A failed verification MUST NOT produce a tagged image; the next `kekkai up` MUST attempt a fresh build rather than reuse a broken image.
- **FR-003**: When a build fails due to Claude installation verification, the user-facing output of `kekkai up` MUST include an actionable explanation: the Claude native binary failed to install, the cause is usually transient (registry/CDN), and rerunning `kekkai up` retries the build.
- **FR-004**: The build MUST retry the Claude installation once when the first attempt fails verification, before declaring the build failed.
- **FR-005**: The verification step MUST NOT require network access beyond what the installation itself already uses; verifying an already-complete installation MUST work without any network access.
- **FR-006**: Image hash semantics MUST be preserved: the only hash impact is the normal content change of the build recipe (one expected rebuild for existing users); runtime configuration MUST NOT gain any influence on the hash.
- **FR-007**: The offline-fallback path (reuse of an existing image when the registry is unreachable) MUST be unchanged: it never builds, so it MUST NOT gain any new checks, warnings, or failure modes.
- **FR-008**: On a successful build, the verification MUST be invisible to the user apart from negligible build-time overhead.

### Key Entities

- **Sandbox image**: The content-hash-tagged image baked per configuration; gains a build-time guarantee that its Claude installation is functional.
- **Claude installation**: The wrapper package plus its platform-native binary; "functional" means the `claude` command runs successfully, not merely that installation reported success.
- **Build verification step**: The new gate between installation and image completion; its failure is a build failure.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero sandboxes start with a "claude native binary not installed" abort from images built after this feature: every image that completes a build contains a runnable Claude.
- **SC-002**: A user hitting the transient download failure recovers by rerunning `kekkai up` at most once (and in most cases not at all, thanks to the in-build retry) — versus today's unrecoverable state that requires manually deleting the image.
- **SC-003**: 100% of verification-failure build outputs name the cause and the retry remedy.
- **SC-004**: Build duration for successful builds increases by no more than a few seconds compared to the previous recipe.

## Assumptions

- A single in-build retry is sufficient for the observed failure mode; persistent registry outages are expected to fail the build (correctly) and be retried by the user.
- The one-time rebuild forced on existing users by the recipe change is acceptable (normal kekkai behavior on any recipe update).
- Verifying via actually running the installed `claude` command (e.g. asking it for its version) is an acceptable functional check and requires no network access.
- Images built before this feature that are already broken are out of scope for detection; the forced rebuild replaces them.
- The build environment has enough disk headroom that a retried download does not systematically fail for space reasons; disk exhaustion failing the build loudly is acceptable behavior.
