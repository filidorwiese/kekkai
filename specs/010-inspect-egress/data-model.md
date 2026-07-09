# Data Model: Inspect egress traffic (`kekkai watch`)

Nothing is persisted. All state is in-memory for one watch session; the sandbox
container is read-only input (resolved by the existing `kekkai.cwd` label).

## Entities (in-memory, session-scoped)

### Traffic event

One observed outbound connection attempt (from an NFLOG group reader).

| Field | Source | Notes |
|---|---|---|
| Timestamp | tcpdump `-tt` epoch | rendered `HH:MM:SS` local time |
| Verdict | which reader emitted it | group 1 → ALLOW, group 2 → BLOCK |
| Protocol | tcpdump decode | tcp / udp / other passthrough |
| Destination IP:port | tcpdump decode | |
| Hostname | lookup in Hostname cache | optional; omitted when unknown |

### DNS event

One observed lookup or answer (group 1, port-53 packets).

| Field | Source | Notes |
|---|---|---|
| Timestamp | tcpdump `-tt` epoch | |
| Kind | query / answer | from tcpdump DNS decode (`A?` vs `A`) |
| Hostname | DNS payload | trailing dot stripped |
| Answered IPs | answer payload | feed the Hostname cache |

### Hostname cache

`map[ip]hostname`, fed by DNS answers, read by traffic-event annotation.
Last-writer-wins (an IP re-answered under a new name adopts it). Lifetime: the
watch session; never written to disk.

### Repeat-suppression window

`map[(verdict,proto,ip,port)]lastPrinted`; identical tuples within 5s are
counted, not printed (validation rule: the FIRST occurrence of any tuple is
always printed — no new destination may be omitted, per spec assumption).

## State transitions

Watch session: `resolving → streaming → (interrupted → cleanup → exit 0 | sandbox-stopped → exit 1)`.
The sandbox container's state is never modified in any transition (FR-005/006);
cleanup only kills the reader processes watch itself started.
