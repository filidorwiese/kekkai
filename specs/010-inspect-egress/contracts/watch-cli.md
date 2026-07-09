# Contract: `kekkai watch` CLI

## Invocation

```
kekkai watch
```

No flags. Any argument → usage error on stderr, exit 1:
`usage: kekkai watch (no arguments)`.

## Stream contract (stdout, one event per line)

| Event | Format |
|---|---|
| Allowed connection | `HH:MM:SS ALLOW <proto> <ip>:<port>` or `... <ip>:<port> (<hostname>)` |
| Blocked connection | `HH:MM:SS BLOCK <proto> <ip>:<port>` or `... <ip>:<port> (<hostname>)` |
| DNS query | `HH:MM:SS DNS   query <hostname>` |
| DNS answer | `HH:MM:SS DNS   answer <hostname> -> <ip>[ <ip>...]` |
| Unparseable reader line | passed through raw, never dropped |

- Verdict column is exactly `ALLOW`, `BLOCK`, or `DNS` (fixed width, grep-able: `grep BLOCK`).
- `(<hostname>)` appears when a prior DNS answer in this session mapped the IP (FR-004).
- Repeated identical `(verdict, proto, ip, port)` tuples within 5s are suppressed
  after the first occurrence; a new destination always prints immediately.
- Startup banner (stderr, one line): `watching egress of sandbox for <pwd> (Ctrl+C to stop)`.
- Under `network.allow_all`: stream works, only `ALLOW`/`DNS` lines occur (FR-010).

## Exit-code contract

| Outcome | Exit | Notes |
|---|---|---|
| User interrupt (Ctrl+C / SIGTERM) | 0 | normal way to end a stream; readers cleaned up |
| Sandbox stopped while watching | 1 | stderr: `sandbox stopped` |
| No running sandbox for `$PWD` | 1 | stderr: `no running sandbox for <pwd>, run 'kekkai up'` |
| Sandbox image predates watch | 1 | stderr: `sandbox image predates 'kekkai watch'; run 'kekkai down' and 'kekkai up' to rebuild` |
| Capture unavailable (kernel lacks nflog, e.g. some macOS runtimes) | 1 | stderr names the failing capability; runtime name decorates the message when recognized |

## Guarantees

- Observe-only: watch never modifies firewall rules, verdicts, or container state (FR-005).
- Ending watch leaves no reader processes in the sandbox (explicit cleanup; FR-006).
- Repeatable: consecutive watch sessions against the same sandbox behave identically.

## Firewall-script contract (§9 amendment)

- NFLOG rules are non-terminating and adjacent to the verdict rule they mirror:
  group 1 = DNS queries (before the `lo` ACCEPT), DNS answers (INPUT), NEW
  connections before the ipset/bridge ACCEPTs; group 2 = NEW connections
  immediately before the REJECT.
- `ALLOW_ALL=1` path installs group-1 observe-only rules with ACCEPT policies, no group 2.
- Default DROP policies, ACCEPT/REJECT lines, and the §9.6 verification probes
  are byte-for-byte unaffected by this feature.

## Help text

`kekkai help` gains:

```
  watch       stream egress traffic of the running sandbox for $PWD
              connections labeled ALLOW/BLOCK, DNS queries inline
```
