# Feature Specification: Sandbox-awareness context for Claude Code

**Feature Branch**: `011-sandbox-awareness`

**Created**: 2026-07-09

**Status**: Draft

**Input**: User description: "Inject sandbox-awareness context into Claude Code at startup — Claude should know it is running in a restricted environment, recognize sandbox-caused failures, and guide the user to the correct .kekkai.yaml change instead of attempting workarounds." (Full description with pinned requirements, verbatim prompt text, and acceptance criteria provided; encoded below.)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Claude diagnoses sandbox-caused failures correctly (Priority: P1)

A developer's sandboxed agent hits a failure that is actually a sandbox restriction — an unreachable host, a missing tool, an unexpectedly empty (shadowed) file. Because Claude was told at startup that it runs inside Kekkai and what the restriction categories are, it recognizes the pattern, names the exact `.kekkai.yaml` addition (`network.allowed_domains`, `image.apt_packages`, or `disk.mounts`), and tells the user to restart with `kekkai up` — instead of burning time on workarounds (proxies, mirrors, retries) or trying to weaken the sandbox.

**Why this priority**: This is the feature's purpose: turning confusing sandbox friction into a one-step config fix, and keeping the agent from fighting the boundary.

**Independent Test**: In a sandbox whose allowlist lacks some domain, ask Claude to fetch that domain; verify the answer proposes the specific `network.allowed_domains` entry plus a `kekkai up` restart, and proposes no bypass.

**Acceptance Scenarios**:

1. **Given** a running sandbox, **When** the user asks Claude "are you running in a sandbox?", **Then** Claude answers correctly, referencing Kekkai.
2. **Given** a domain absent from the allowlist, **When** the user asks Claude to fetch it, **Then** Claude suggests adding that domain under `network.allowed_domains` and restarting via `kekkai up` — not a proxy/mirror workaround.
3. **Given** the injected context, **When** a command fails for an ordinary (non-sandbox) reason, **Then** Claude still considers normal causes first (the context instructs this ordering).

---

### User Story 2 - Scripts and tools can detect the sandbox (Priority: P2)

A developer's scripts, hooks, or Claude itself check programmatically whether they are running inside a kekkai sandbox via the `KEKKAI_SANDBOX=1` environment variable, and adjust behavior (e.g. skip host-only steps).

**Why this priority**: Cheap, explicitly requested, and the only machine-readable sandbox signal; the prose prompt is for the model, the env var is for code.

**Independent Test**: `kekkai exec sh -c 'echo $KEKKAI_SANDBOX'` prints `1`.

**Acceptance Scenarios**:

1. **Given** a running sandbox, **When** any process inside it reads `KEKKAI_SANDBOX`, **Then** the value is `1`.

---

### User Story 3 - Graceful degradation on unsupported Claude versions (Priority: P3)

A developer pins an old Claude Code version (`claude.version`) that does not support appending to the system prompt in interactive mode. The sandbox still starts normally: kekkai logs one warning that sandbox-awareness context was skipped, and everything else works as before.

**Why this priority**: Protects the existing contract (any pinnable version keeps working); failure of an advisory feature must never block startup.

**Independent Test**: Pin a Claude Code version older than interactive append support, run `kekkai up`; sandbox starts, a single warning mentions the skipped context, Claude runs without it.

**Acceptance Scenarios**:

1. **Given** a resolved Claude Code version that does not support the append mechanism interactively, **When** the sandbox starts, **Then** startup succeeds, one warning is printed, and no context is injected.
2. **Given** a supported version, **When** the sandbox starts, **Then** the context is injected and no warning appears.

---

### Edge Cases

- User-configured `claude.args` (which replaces the default argument string) → the sandbox context is still injected in addition; user args keep working alongside it.
- User's own `claude.args` already contains an append-system-prompt flag of their own → both texts are delivered; kekkai's injection must not swallow or replace the user's.
- Prompt text contains spaces, newlines, quotes → must arrive intact through the container startup chain (no word-splitting corruption).
- No config file (defaults run) → context still injected; templated values (if rendered) reflect defaults.
- `network.allow_all` sandbox → context still injected (filesystem/tools restrictions still apply even with the firewall off).
- Very long allowlists/secret lists (optional enhancement) → rendered section stays within ~10 lines, truncated with a count ("...and 12 more").

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When starting Claude Code in the sandbox, kekkai MUST deliver a sandbox-awareness context by *appending* to Claude's system prompt via the CLI's append mechanism (`--append-system-prompt`) — never via a flag that replaces the system prompt wholesale.
- **FR-002**: The appended context MUST be the pinned prompt text (see "Prompt text" below), verbatim apart from the optional templated section (FR-008).
- **FR-003**: The container environment MUST contain `KEKKAI_SANDBOX=1`, visible to every process in the sandbox.
- **FR-004**: The feature MUST NOT create or modify `CLAUDE.md` in the workspace and MUST NOT write anything into the mounted `~/.claude/` directory (that directory is shared with the host).
- **FR-005**: The injected argument MUST compose with the existing argument list: the default `--dangerously-skip-permissions`, user-configured `claude.args` (which replace the default), and `up`'s after-`--` extras all keep working.
- **FR-006**: If the resolved Claude Code version does not support the append mechanism in interactive mode, kekkai MUST print exactly one warning and start the sandbox *without* the context — never fail startup over it.
- **FR-007**: The prompt text MUST live in the kekkai codebase as a named constant or embedded template, not inline at the point of process assembly.
- **FR-008** *(optional enhancement — implement only if straightforward)*: The prompt MAY be extended with a short section rendered from the parsed config: currently allowed domains and CIDRs, and names of shadowed files; at most ~10 rendered lines, long lists truncated with a count.

### Key Entities

- **Sandbox-awareness prompt**: the pinned advisory text delivered to Claude at startup; optionally suffixed with a config-derived summary (allowed domains/CIDRs, shadowed file names).
- **Sandbox marker**: the `KEKKAI_SANDBOX=1` environment variable — the machine-readable counterpart of the prompt.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Asked "are you running in a sandbox?", Claude answers correctly referencing Kekkai on the first try.
- **SC-002**: For a fetch of a non-allowlisted domain, Claude's first response names the exact `network.allowed_domains` addition and the `kekkai up` restart, with zero bypass suggestions.
- **SC-003**: `echo $KEKKAI_SANDBOX` inside the sandbox prints `1` in 100% of sandboxes started by this version.
- **SC-004**: After a session using this feature, the workspace contains no new/modified `CLAUDE.md` and `~/.claude/` contains nothing written by kekkai.
- **SC-005**: Sandboxes with user `claude.args`, extra `up -- ...` args, or an unsupported pinned version all start exactly as before (unsupported version: plus one warning line).

## Prompt text (pinned, verbatim)

> You are running inside Kekkai, a security sandbox (docs:
> https://github.com/filidorwiese/kekkai). The environment is intentionally
> restricted:
>
> - Filesystem: only the workspace and explicitly configured mounts are visible.
>   Some files may be shadowed (present but empty) because they contain secrets.
> - Network: outbound traffic is limited to an allowlist. Blocked destinations
>   typically fail as connection timeouts or refused connections.
> - Tools: only packages installed in the sandbox image are available.
>
> When a command fails, first consider normal causes. If the failure pattern
> matches a sandbox restriction (unreachable host, missing tool, unexpectedly
> empty file), do not attempt to bypass or disable the sandbox. Instead, tell the
> user exactly what to add to .kekkai.yaml in the workspace root - for example a
> domain under network.allowed_domains, a package under image.apt_packages, or a
> mount under disk.mounts - and mention that changes take effect after restarting
> with `kekkai up`. Configuration reference:
> https://github.com/filidorwiese/kekkai#configure
>
> These restrictions are chosen by the user. Work within them by default.

## Assumptions

- The append mechanism and the interactive-mode version threshold are verifiable at plan time; the threshold becomes a code constant compared against the resolved `claude.version`.
- "Resolved version" means the version the image was built with (including `latest` resolution), so the support check needs no network call of its own.
- The env var is baked as a builtin (§5.3-style), not user-overridable config; users who want it gone can be pointed at future config if ever requested.
- The optional templated section (FR-008) is judged "straightforward" at plan time; if it complicates argument assembly or quoting, it is skipped without renegotiating this spec.
- Model behavior criteria (SC-001/SC-002) are validated manually with one prompt each in the e2e run; they depend on the model honoring instructions, which the pinned text maximizes but cannot guarantee — the testable contract is that the text is delivered intact.
