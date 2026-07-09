# Feature Specification: Inspect egress traffic

**Feature Branch**: `010-inspect-egress`

**Created**: 2026-07-08

**Status**: Draft

**Input**: User description: "I would like to a command to inspect iptables egress traffic, this can be useful for developers who want to add additional network exceptions but dont know which endpoints are being called (for a example, a mcp server, but which?). If possible also add DNS lookups to the same trailing log so that the hostnames can be seens as well. Advise what options there are"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover which endpoints a tool needs (Priority: P1)

A developer's sandboxed agent uses a tool (for example an MCP server) that fails because the firewall blocks its traffic. The developer doesn't know which endpoints the tool calls. From a second terminal they start the traffic inspection command, reproduce the failure in the sandbox, and watch a live log of outbound connection attempts — each line showing destination and whether it was allowed or blocked — so they can copy the right hostnames into `network.allowed_domains`.

**Why this priority**: This is the feature's reason to exist: today the only way to find missing exceptions is trial-and-error or reading third-party source code.

**Independent Test**: With a sandbox running, start the inspection command, run a command in the sandbox that calls a non-allowlisted endpoint, and verify a log line appears identifying that destination as blocked.

**Acceptance Scenarios**:

1. **Given** a running sandbox and the inspection command active, **When** a process inside the sandbox attempts a connection to a blocked destination, **Then** a log line appears showing the destination and a "blocked" verdict.
2. **Given** the inspection command active, **When** a process connects to an allowed destination, **Then** a log line appears showing the destination with an "allowed" verdict — all egress is shown, labeled by verdict.
3. **Given** the developer has seen the blocked destinations, **When** they add the hostnames to their config and restart the sandbox, **Then** the tool works and the inspection log shows the same destinations as allowed.

---

### User Story 2 - See hostnames, not just IP addresses (Priority: P2)

While inspecting, the developer sees DNS lookups in the same trailing log, so blocked connection attempts can be matched to the hostname that was just resolved — config exceptions are declared as domain names, so raw IPs alone are not actionable.

**Why this priority**: Without hostnames the log answers "something was blocked" but not "what to put in the config". Explicitly requested ("add DNS lookups to the same trailing log").

**Independent Test**: With inspection active, run a lookup+connect to a non-allowlisted domain inside the sandbox; verify the log shows the DNS query with the domain name and the subsequent blocked connection, correlatable by the developer.

**Acceptance Scenarios**:

1. **Given** inspection active, **When** a process inside the sandbox resolves a domain, **Then** the log shows the queried domain name.
2. **Given** a DNS answer followed by a connection to the answered address, **When** both appear in the log, **Then** the developer can tell which hostname the blocked connection belongs to.

---

### User Story 3 - Inspect without preparation (Priority: P3)

The developer can inspect a sandbox that is already running, without having planned ahead — no special startup mode, no config change, no sandbox restart. Every sandbox is inspectable by default.

**Why this priority**: The need to inspect usually appears mid-session, right after something failed. Requiring a restart loses the failing state.

**Independent Test**: Start a sandbox with default config, then start the inspection command; verify blocked-traffic lines appear without any restart.

**Acceptance Scenarios**:

1. **Given** a sandbox started with default config, **When** the developer runs the inspection command, **Then** traffic lines appear without restarting the sandbox.
2. **Given** no running sandbox for the current directory, **When** the developer runs the inspection command, **Then** a clear error points at `kekkai up`.

---

### Edge Cases

- Sandbox running with the firewall disabled (`network.allow_all`) → inspection still works; everything shows as allowed (still useful to enumerate endpoints before tightening config).
- High-volume traffic (package installs) → the log must remain readable; repeated identical destinations should not drown the signal.
- Connections to raw IPs that were never resolved via DNS (hardcoded IPs) → shown with IP only; no hostname is available.
- The developer interrupts the inspection (Ctrl+C) → command exits cleanly; sandbox and its traffic are unaffected.
- Sandbox stops while inspection is running → inspection ends with a clear message rather than hanging.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST provide a command that streams a live, human-readable log of outbound connection attempts from the running sandbox for the current directory.
- **FR-002**: Each connection line MUST include at least: destination address, destination port, and the firewall verdict (allowed/blocked).
- **FR-003**: DNS queries made inside the sandbox MUST appear in the same log stream, showing the queried hostname, interleaved chronologically with connection lines.
- **FR-004**: The log MUST make it possible to associate a blocked connection with the hostname that resolved to that address, at minimum by temporal proximity of the DNS line and the connection line.
- **FR-005**: Inspection MUST be read-only with respect to the security boundary: it MUST NOT relax the firewall, alter verdicts, or open new destinations.
- **FR-006**: Ending the inspection MUST leave the sandbox exactly as it was; inspection MUST be repeatable in the same session.
- **FR-007**: With no running sandbox for the current directory, the command MUST print an actionable error and exit nonzero.
- **FR-008**: Output MUST be plain text suitable for piping to files or filters (`grep`), one event per line.
- **FR-009**: Inspection MUST attach to any already-running sandbox without restart, config change, or startup flag — every sandbox is inspectable by default.
- **FR-010**: With the firewall disabled (`network.allow_all`), inspection MUST still stream traffic and DNS events, with everything showing as allowed.

### Key Entities

- **Traffic event**: one observed outbound connection attempt — timestamp, destination (address, port, protocol), verdict (allowed/blocked), best-known hostname.
- **DNS event**: one observed lookup — timestamp, queried hostname (and answered addresses when available).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer who does not know a tool's endpoints can identify every hostname the tool needs in one inspection session (reproduce failure once, read the log), without consulting external documentation or source code.
- **SC-002**: For a blocked connection preceded by a DNS lookup, the hostname is identifiable from the log in 100% of cases within the same session.
- **SC-003**: Starting inspection requires a single command and shows the first traffic line within seconds of traffic occurring.
- **SC-004**: After adding the discovered hostnames to config, the tool's traffic shows as allowed and the tool works — zero further trial-and-error rounds.

## Assumptions

- Inspection is per-sandbox (current directory), consistent with every other kekkai command.
- The log is ephemeral (streamed to the terminal, not persisted by kekkai); developers redirect to a file if they want a record.
- Repeated identical events may be summarized or rate-limited for readability, provided no *new* destination is ever omitted.
- Sandbox lifetime traffic before the inspection started need not be shown; only traffic from inspection start onward.
