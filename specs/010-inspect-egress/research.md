# Research: Inspect egress traffic (`kekkai watch`)

Capture-mechanism options were presented to the user at specify time; the chosen
direction (NFLOG + in-container reader) is refined here against the actual
firewall script (`embed/init-firewall.sh`) and container setup.

## R1 — Observation mechanism: iptables NFLOG groups

**Decision**: Add passive `-j NFLOG` rules to `init-firewall.sh`: group 1 for
allowed traffic + DNS, group 2 for blocked traffic. NFLOG is a non-terminating
target — the packet continues to the next rule, so verdicts are decided exactly
where they are today (FR-005).

**Rationale**: Verdict-exact (a packet logged in group 2 is by construction one
that reaches the REJECT line), kernel-native, no daemon, and readable by any
libpcap tool via the `nflog:N` pseudo-interface. Rules live in the sanctioned
script (constitution II: firewall changes happen there, and these open nothing).

**Alternatives considered**:
- iptables `LOG` target — writes to the *host* kernel ring buffer; unreadable
  from the container with `kernel.dmesg_restrict`, pollutes the host journal — rejected.
- Plain tcpdump on `eth0` — one tool, no firewall change, but verdicts must be
  inferred from icmp-admin-prohibited echoes and docker's embedded-DNS NAT hides
  lo-path DNS; indirect and fragile — rejected.
- `conntrack -E` — flow events exist for dropped packets too (unreplied entries);
  verdict distinction unreliable — rejected.
- dnsmasq DNS proxy — best hostnames, disproportionate architecture change — rejected.

## R2 — Reader: tcpdump on `nflog:<group>` via `docker exec -u root`

**Decision**: `kekkai watch` spawns `docker exec -u root <id> tcpdump -l -n -tt -i nflog:1`
(and `nflog:2`), one process per group. Add `tcpdump` to `builtinAptPackages`
(§5.1, subcommands tier, like `zsh` for `shell`).

**Rationale**: tcpdump decodes NFLOG-encapsulated packets including DNS payloads
(`A? host.` / `A host. 1.2.3.4`) — connection lines and hostname data from one
tool. Root is required for nflog socket + CAP_NET_ADMIN; `docker exec -u root`
is host-side authority the user already has (they own the docker daemon), so the
in-container sudoers grant stays firewall-only (constitution II). Two processes
instead of NFLOG prefix parsing: group membership IS the verdict — no fragile
prefix extraction from tcpdump output.

**Alternatives considered**: `ulogd2` (daemon + config file in image — heavier);
reading nfnetlink from Go on the host (impossible — netlink is per-netns, kekkai
would need to enter the container's netns; that's what docker exec is for);
single group with `--nflog-prefix` verdict tags (tcpdump does not reliably print
nflog prefixes — rejected).

## R3 — Output shaping on the host

**Decision**: Merge both reader streams line-by-line in Go. Parse tcpdump's
stable `-n -tt` fields into the contract's line formats; keep an in-memory
IP→hostname map fed by DNS *answer* lines (captured via an INPUT `udp sport 53`
NFLOG rule in group 1) and annotate ALLOW/BLOCK lines with the best-known
hostname. Suppress repeats of the same (verdict, proto, ip, port) tuple within a
5s window — first occurrence always printed. Unparseable lines pass through raw.

**Rationale**: FR-004's minimum is temporal proximity, but the answer-fed map
upgrades that to inline `(hostname)` annotations for the common case, which is
what makes SC-001 (one session, no external docs) realistic. Raw passthrough
guarantees no new destination is ever silently omitted (spec assumption).

**Alternatives considered**: printing raw tcpdump lines with an ALLOW/BLOCK
prefix only (grep-able but leaves IP→hostname correlation entirely to the user
— fails SC-001's spirit); resolving IPs via reverse DNS (PTR records are wrong
or missing for CDNs — rejected).

## R4 — Lifecycle and cleanup

**Decision**: Reuse the signal pattern from `docker.Interactive` but with
explicit teardown: on SIGINT/SIGTERM kill both reader `docker exec` processes,
then best-effort `docker exec -u root <id> pkill -x tcpdump`, exit 0 (interrupt
is the normal way to end a stream). Readers ending on their own (sandbox
stopped) → stderr message, exit 1.

**Rationale**: Feature 009 established that the docker CLI does not proxy
signals to exec'd processes — without the explicit pkill, root tcpdump processes
would linger in the sandbox, violating FR-006.

## R5 — Old sandboxes (image predates watch)

**Decision**: A container from a pre-tcpdump image makes the reader exec fail
fast (126/127); watch prints
`sandbox image predates 'kekkai watch'; run 'kekkai down' and 'kekkai up' to rebuild`
and exits 1. Adding `tcpdump` to §5.1 changes the bake-time inputs (§6.1), so
the next `kekkai up` rebuilds automatically — no manual image management.

**Rationale**: Same-vintage detection: an image without tcpdump is also an image
whose firewall script has no NFLOG rules, so tool presence is a sufficient probe.

## R6 — `allow_all` and rule placement details

**Decision**: In the `ALLOW_ALL=1` early-exit path, install observe-only group-1
rules (DNS query/answer + `--state NEW` OUTPUT log) before `exit 0`; policies
remain ACCEPT, no group 2 exists. In the normal path: DNS-query NFLOG precedes
the loopback ACCEPT (docker's embedded DNS rides `lo` post-NAT, so a rule after
the `lo` ACCEPT would miss it — exact placement asserted by the quickstart
hostname scenario, per constitution IV); allowed-NEW NFLOG rules sit immediately
before the ipset and bridge ACCEPTs; blocked NFLOG (`--state NEW`) sits
immediately before the REJECT.

**Rationale**: FR-009/FR-010 (attach anytime, allow_all still streams) with one
uniform reader path; verdict labels stay truthful because each log rule is
adjacent to the verdict rule it mirrors. NEW-state matching keeps volume at
connections, not packets (performance goal).

## R7 — Command shape and name

**Decision**: `kekkai watch`, no flags, no arguments (extra args → usage error).

**Rationale**: Matches the CLI's terse verb style (`up`, `down`, `shell`, `exec`);
"watch" says live-stream. Q1 decision (all traffic, labeled) removes the need
for any filter flag; `grep` covers the rest (FR-008).

**Alternatives considered**: `kekkai traffic` (noun, inconsistent with verb
style); `kekkai inspect` (collides with `docker inspect` static-metadata
connotation); an `--all`/`--blocked` flag pair (Q1 chose all-labeled — rejected).
