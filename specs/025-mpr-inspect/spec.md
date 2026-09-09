# Feature Specification: Inspect model-provider requests (`kekkai mpr`)

**Feature Branch**: `025-mpr-inspect`

**Created**: 2026-09-08

**Status**: Draft

## Clarifications

### Session 2026-09-08

- Q: What does one exchange display, given each request re-sends the full conversation? → A: Full request every time (faithful audit record; readability via rendering, not omission).
- Q: Output format? → A: Rendered human-readable by default, colored when stdout is a terminal; `--raw` flag emits provider JSON lines.
- Q: History on attach? → A: Live only, from attach onward (pipe-like, consistent with `kekkai traffic`).
- Q: User sets `ANTHROPIC_BASE_URL` in config `env`: chain through the built-in interception, or bypass it? → A: Bypass. The user's URL overrules the built-in interception entirely; Claude talks to the user's URL directly, nothing is captured.
- Q: Visual separation between exchanges in the stream? → A: A clear separator line MUST precede each request and each response block, carrying the exchange number, direction, and timestamp.
- Q: When does an exchange appear in the stream? → A: Request block printed when sent; response block printed when the response completes (reassembled). Concurrent exchanges may alternate blocks, linked by exchange number.
- Q: Interception point dies mid-session? → A: Automatic restart inside the sandbox; in-flight exchanges fail once, later ones work; `kekkai mpr` stays attached and prints a notice.

**Input**: User description: "implement a new feature, similar to `kekkai traffic` but then named `kekkai mpr` (model-provider-request). When run in a separate terminal, it outputs the contents of the messages going from/to claude. The mechanism is always using ANTHROPIC_BASE_URL to proxy requests going to claude api (whether the user is listening with `kekkai mpr` or not, output the logs to a temporary in-memory file/pipe, which a user can hook into."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See what leaves the sandbox toward the model provider (Priority: P1)

A developer running an autonomous Claude session wants to know exactly what Claude is sending to the model provider and what comes back: the prompts, the file contents that were included, the tool calls and their results, the model's replies. From a second terminal they run `kekkai mpr` and watch a live, readable transcript of every exchange as it happens. `kekkai traffic` answers *where* the sandbox talks; `kekkai mpr` answers *what* it says to the one destination that is always allowed.

**Why this priority**: This is the feature's reason to exist. The firewall makes egress destinations auditable but the content sent to the provider is opaque today; for a security-focused sandbox, seeing that content (did a secret get pasted into a prompt? what context did Claude actually receive?) is the missing half of the audit story.

**Independent Test**: With a sandbox running, start `kekkai mpr`, type a prompt into Claude in the sandbox, and verify the prompt text and the model's reply appear in the `mpr` terminal within seconds.

**Acceptance Scenarios**:

1. **Given** a running sandbox and `kekkai mpr` active, **When** Claude sends a request to the model provider, **Then** the request's message content appears in the `mpr` stream, attributable to that request.
2. **Given** `kekkai mpr` active, **When** the provider's response arrives (including responses delivered incrementally as a stream), **Then** the complete response content appears in the `mpr` stream, paired with its request.
3. **Given** Claude issues tool calls and receives tool results, **When** these are exchanged with the provider, **Then** the stream shows the tool name and its input/result content.
4. **Given** `kekkai mpr` active, **When** the provider returns an error (rate limit, overload, bad request), **Then** the stream shows the error status and message instead of silently omitting the exchange.

---

### User Story 2 - Inspect without preparation, and without changing Claude's behavior (Priority: P1)

The need to inspect appears mid-session. The developer attaches `kekkai mpr` to a sandbox that was started with default config, with no startup flag and no restart, and Claude keeps working exactly as before: same replies, same latency as far as the developer can tell, same authentication. Detaching leaves the sandbox untouched, and the developer can attach again later.

**Why this priority**: If capture had to be planned ahead the failing state would be lost, and if capture could alter Claude's behavior the observations would be worthless. Both are prerequisites for story 1 being trustworthy.

**Independent Test**: Start a sandbox with an empty config, run Claude for a few turns, then start `kekkai mpr`; verify subsequent exchanges appear, Claude keeps functioning, Ctrl+C in the `mpr` terminal leaves Claude running, and a second `kekkai mpr` works identically.

**Acceptance Scenarios**:

1. **Given** a sandbox started with default config, **When** the developer runs `kekkai mpr`, **Then** exchanges stream without any restart, flag, or config change.
2. **Given** `kekkai mpr` was never run during a session, **When** Claude operates, **Then** Claude's behavior and authentication are indistinguishable from a sandbox without this feature.
3. **Given** `kekkai mpr` is streaming, **When** the developer presses Ctrl+C, **Then** `mpr` exits cleanly and the sandbox and Claude are unaffected.
4. **Given** no running sandbox for the current directory, **When** the developer runs `kekkai mpr`, **Then** a clear error points at `kekkai up` and the command exits nonzero.
5. **Given** the sandbox stops while `kekkai mpr` is streaming, **When** that happens, **Then** `mpr` ends with a clear message rather than hanging.

---

### User Story 3 - Nothing sensitive persists (Priority: P2)

The captured exchanges contain everything Claude saw: source code, prompts, possibly credentials the developer accidentally exposed. The developer expects that this record lives only in memory inside the sandbox, is bounded in size, is never written to host disk or into the image, and vanishes when the sandbox stops. The developer's own provider credentials (API key, session tokens) never appear in the stream.

**Why this priority**: The feature is only acceptable in a security sandbox if it does not create a new leak. Persisting transcripts or echoing credentials would turn an audit tool into a liability.

**Independent Test**: Run a session with `kekkai mpr`, stop the sandbox, and verify no transcript data exists on the host or in any image layer; during the session, verify no line of `mpr` output contains the credential used to authenticate to the provider.

**Acceptance Scenarios**:

1. **Given** a long session with no `kekkai mpr` attached, **When** the developer inspects the sandbox's memory use, **Then** captured content does not accumulate; memory stays within a fixed cap regardless of session length.
2. **Given** a sandbox that has been stopped, **When** the developer searches the host filesystem and the image, **Then** no captured exchange content is found.
3. **Given** any `mpr` output, **When** searched for the API key or auth token in use, **Then** zero matches occur.

---

### Edge Cases

- Claude issues several provider requests concurrently (parallel subagents): the stream must keep each exchange attributable (request and its response identifiable as a pair) rather than interleaving fragments unreadably.
- Each request re-sends the entire conversation plus a large system prompt and tool definitions; output repeats tens of kilobytes per turn by design (full request every time). Rendering, delimiters, and color keep it scannable; filtering is the developer's job (`grep`, redirect to file).
- The developer attaches after many exchanges have already happened: only exchanges from attach onward appear; nothing is replayed.
- Output is piped or redirected (not a terminal): no color codes are emitted, so files and `grep` see plain text.
- The developer's own config sets `ANTHROPIC_BASE_URL` (a corporate gateway): the built-in interception is not installed at all; Claude talks to the developer's URL directly and nothing is captured. `kekkai mpr` then reports that capture is disabled by the user's config and exits 1.
- The developer sets `ANTHROPIC_BASE_URL` by a means kekkai cannot see at startup (Claude's own settings inside the sandbox, an in-session export): interception is installed but Claude never uses it; `kekkai mpr` streams nothing. Documented limitation.
- Claude is configured to use a third-party hosting route (e.g. cloud-vendor endpoints) that does not honor the base-URL setting: such traffic is not captured; `mpr` stays silent and the user-facing docs state the limitation.
- The interception point fails to start when the sandbox comes up: startup fails loudly (same posture as firewall verification), never silently falling back to uncaptured direct traffic.
- The interception point dies mid-session: it is restarted automatically inside the sandbox; exchanges in flight fail once (Claude retries on its own), later exchanges are captured again; an attached `kekkai mpr` stays attached and prints a one-line notice about the restart. Direct uncaptured fallback never happens.
- Sandbox image built before this feature: `kekkai mpr` explains that the image predates the command and how to rebuild, mirroring `kekkai traffic`.
- `network.allow_all` set: capture works identically; it is independent of firewall verdicts.
- Very large single messages (a file of several MB pasted into context): shown in full.
- Provider responses that are not message exchanges (token counting, model listing): shown compactly by endpoint and status, not as transcript content.
- Binary content blocks (images, documents) inside messages: rendered as a one-line placeholder with type and size, never as an encoded dump; `--raw` still emits them verbatim.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Every sandbox MUST route Claude's model-provider traffic through an in-sandbox interception point by default, with no flag, config key, or restart required. Every sandbox is inspectable by default, as with `kekkai traffic`.
- **FR-002**: Interception MUST be transparent: requests reach the provider with headers, body, authentication, and streaming semantics unchanged, and responses return to Claude unchanged. Claude's behavior MUST be indistinguishable from an uncaptured sandbox.
- **FR-003**: Every exchange MUST be published to a memory-backed, sandbox-scoped capture stream whether or not anyone is reading, so that `kekkai mpr` can hook into it at any moment without preparation.
- **FR-004**: The capture stream MUST be bounded: with no reader attached, content is discarded rather than accumulated, and memory use stays within a fixed cap. Publishing MUST never cause Claude's requests to fail or stall, including when a reader is slow.
- **FR-005**: The capture stream MUST live only in sandbox memory: never written to host disk, a bind mount, or an image layer, and MUST cease to exist when the sandbox stops.
- **FR-006**: `kekkai mpr` MUST attach to the running sandbox for the current directory and stream exchanges live in plain text suitable for piping and filtering. Each request block and each response block MUST begin with a full-width separator line showing the exchange number, direction (request/response), timestamp, and for responses the outcome status; the separator MUST be visually unmistakable when scrolling and greppable by a fixed marker.
- **FR-007**: The request block MUST be printed when the request is forwarded; the response block MUST be printed once the response has fully completed, with streamed responses reassembled into their final form. Blocks of concurrent exchanges may alternate; the exchange number on each separator links them. For each exchange the stream MUST show: a timestamp, the endpoint and outcome (status), the request's message content (roles, text, tool calls, tool results), and the complete response content.
- **FR-008**: Each exchange MUST display the full request as sent (system prompt, tool definitions, entire conversation) every time; nothing is collapsed, deduplicated, or elided.
- **FR-009**: Output MUST be a rendered human-readable transcript by default. When stdout is a terminal, roles, tool calls, tool results, and status lines MUST be color-distinguished; when stdout is not a terminal, output MUST be plain text with no color codes. A `--raw` flag MUST instead emit the provider's request and response JSON, one object per line, never colored; each object carries the exchange number, direction, and timestamp, and a streamed response is emitted as its reassembled final message object.
- **FR-010**: `kekkai mpr` MUST show only exchanges that begin after it attaches; earlier exchanges are never replayed.
- **FR-011**: Concurrent exchanges MUST remain attributable: a response is always identifiable with its request, and content from different exchanges is never merged into one block.
- **FR-012**: Provider credentials and any authentication or session header MUST never be recorded or displayed. Message content is shown as-is; it is the developer's own data.
- **FR-013**: Interception MUST NOT change the security boundary: no new network destinations, no host port, no listening socket reachable from outside the sandbox. The upstream is always the always-allowed provider host.
- **FR-018**: When the user's config `env` sets `ANTHROPIC_BASE_URL`, the user's value MUST overrule the built-in interception: the interception point is not installed, Claude's requests go to the user's URL directly, and nothing is captured. `kekkai mpr` MUST then exit 1 with a message stating capture is disabled because `ANTHROPIC_BASE_URL` is set in config. Reaching the user's host still requires it in `network.allowed_domains` (firewall unchanged).
- **FR-014**: Failure to establish interception at sandbox startup MUST fail startup with an actionable error; the sandbox MUST NOT silently fall back to uncaptured direct traffic.
- **FR-019**: If the interception point exits mid-session it MUST be restarted automatically inside the sandbox; exchanges in flight may fail once, subsequent exchanges MUST be captured again. An attached `kekkai mpr` MUST remain attached and print a notice line naming the restart. Exchange numbering MUST continue, not reset.
- **FR-015**: Exit behavior MUST mirror `kekkai traffic`: Ctrl+C exits 0 and cleans up in-sandbox readers; sandbox stopped mid-stream exits 1 with a clear message; no running sandbox exits 1 with a pointer to `kekkai up`; an image predating the command exits 1 with a rebuild hint. Any argument to `kekkai mpr` is a usage error.
- **FR-016**: Multiple `kekkai mpr` sessions MAY run at once against the same sandbox; each sees every exchange from its own attach point onward.
- **FR-017**: Help output and the user README MUST document `mpr` next to `traffic`, including the limitation that provider routes not governed by the base-URL setting are not captured.

### Key Entities

- **Exchange**: one request/response pair with the model provider: exchange number (monotonic per sandbox lifetime, unique), timestamps, endpoint, outcome status, request messages, response content, timing. Concurrent exchanges are distinct entities.
- **Message**: one entry in a conversation as sent to or received from the provider: role, and ordered content blocks (text, tool call with name and input, tool result, other).
- **Capture stream**: the bounded, memory-backed, sandbox-scoped live feed of exchanges that exists for the sandbox's lifetime and discards content nobody is reading; the thing `kekkai mpr` hooks into.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can see the full content of what Claude sent and received for any turn using a single command, with the exchange visible within 2 seconds of the provider's response completing.
- **SC-002**: Claude's functionality is unchanged: every workflow that works in an uncaptured sandbox (streaming replies, tool use, long sessions, authentication) works identically, and added round-trip overhead per request stays below 50 ms.
- **SC-003**: Provider credentials appear in 0% of `mpr` output lines across a session that authenticates and exchanges many messages.
- **SC-004**: Memory attributable to capture stays at or below a fixed cap for the entire lifetime of a sandbox, regardless of session length, message size, or whether anyone ever attached.
- **SC-005**: After `kekkai down`, a search of the host filesystem and image layers finds zero captured exchange content.
- **SC-006**: Attaching, detaching, and re-attaching `kekkai mpr` ten times in one session leaves zero leftover reader processes in the sandbox and never interrupts Claude.
- **SC-007**: After the interception point is killed mid-session, Claude's next successful request is captured, and an attached `kekkai mpr` needs no user action to keep streaming.

## Out of Scope

- A switch to disable capture other than setting `ANTHROPIC_BASE_URL`.
- Persisting or exporting transcripts; replaying history on attach.
- Modifying, redacting, or blocking requests (observe-only, like `traffic`).
- Capturing provider routes that do not honor `ANTHROPIC_BASE_URL`.
- Filtering or searching inside `mpr` (use `grep` / redirection).

## Assumptions

- Inspection is per-sandbox (current directory), consistent with every other kekkai command.
- Capture is always on because the value is highest exactly when it was not planned for; with live-only semantics the idle cost is negligible (unread content is discarded). A user-facing switch to disable capture is out of scope for v1 (minimal-surface principle); it can be added if a concrete need appears.
- Interception runs entirely inside the sandbox and terminates the provider's TLS only inside the sandbox; the connection from the interception point to the provider remains TLS. Traffic between Claude and the interception point never leaves the container.
- `ANTHROPIC_BASE_URL` is not a reserved config key: setting it is the supported way to opt out of capture and point Claude elsewhere, with no separate switch.
- Third-party hosting routes for Claude that bypass the base-URL setting are not captured; documented as a limitation, not handled.
- The capture cap is a code constant, not a config key, per the one-config-file principle. Its role is back-pressure safety, not history.
- `--raw` is the first flag on a kekkai subcommand; accepted because rendered and machine-readable output are both needed and a second subcommand would be worse for surface area.
- Color is a rendering convenience only and never changes content; `NO_COLOR` convention respected.
- The interception point is a bake-time input, so this feature triggers a one-time image rebuild on next `kekkai up`, mirroring feature 010/013.
- Transcript output is ephemeral and streamed to the terminal; developers redirect to a file if they want a record, and doing so is their own choice outside the sandbox.
- Kekkai's secrets-hiding mechanism (§8) is unrelated: it hides files from the sandbox; this feature reveals what the sandbox sends out.
- "Very large single messages" edge case: shown in full; the cap governs unread backlog, not message size.
