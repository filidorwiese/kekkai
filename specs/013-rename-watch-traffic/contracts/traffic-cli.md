# Contract: `kekkai traffic` CLI

Supersedes [specs/010-inspect-egress/contracts/watch-cli.md](../../010-inspect-egress/contracts/watch-cli.md)
(feature 013 rename). Identical to that contract except the three strings
that name the command: the invocation, the usage error, and the stale-image
hint. Everything else is byte-identical (FR-001).

## Invocation

```
kekkai traffic
```

No flags. Any argument → usage error on stderr, exit 1:
`usage: kekkai traffic (no arguments)`.

`kekkai watch` is NOT recognized: it takes the standard unknown-command path
(`kekkai: unknown command "watch"` + usage, exit 1) like any other typo.

## Stream contract (stdout, one event per line)

| Event | Format |
|---|---|
| Allowed connection | `HH:MM:SS ALLOW <proto> <ip>:<port>` or `... <ip>:<port> (<hostname>)` |
| Blocked connection | `HH:MM:SS BLOCK <proto> <ip>:<port>` or `... <ip>:<port> (<hostname>)` |
| DNS query | `HH:MM:SS DNS   query <hostname>` |
| DNS answer | `HH:MM:SS DNS   answer <hostname> -> <ip>[ <ip>...]` |
| Unparseable reader line | passed through raw, never dropped |

- Verdict column is exactly `ALLOW`, `BLOCK`, or `DNS` (fixed width, grep-able: `grep BLOCK`).
- `(<hostname>)` appears when a prior DNS answer in this session mapped the IP.
- Repeated identical `(verdict, proto, ip, port)` tuples within 5s are suppressed
  after the first occurrence; a new destination always prints immediately.
- Startup banner (stderr, one line): `watching egress of sandbox for <pwd> (Ctrl+C to stop)`.
- Under `network.allow_all`: stream works, only `ALLOW`/`DNS` lines occur.

## Exit-code contract

| Outcome | Exit | Notes |
|---|---|---|
| User interrupt (Ctrl+C / SIGTERM) | 0 | normal way to end a stream; readers cleaned up |
| Sandbox stopped while watching | 1 | stderr: `sandbox stopped` |
| No running sandbox for `$PWD` | 1 | stderr: `no running sandbox for <pwd>, run 'kekkai up'` |
| Sandbox image predates the command | 1 | stderr: `sandbox image predates 'kekkai traffic'; run 'kekkai down' and 'kekkai up' to rebuild` |
| Capture unavailable (kernel lacks nflog, e.g. some macOS runtimes) | 1 | stderr names the failing capability; runtime name decorates the message when recognized |

## Guarantees

- Observe-only: the command never modifies firewall rules, verdicts, or container state.
- Ending the stream leaves no reader processes in the sandbox (explicit cleanup).
- Repeatable: consecutive sessions against the same sandbox behave identically.

## Firewall-script contract (§9)

Unchanged from feature 010 — NFLOG groups, placement, and the allow_all
observe-only block are as specified there. This feature touches script
comments only; rules are byte-for-byte identical.

## Help text

`kekkai help` shows:

```
  traffic     stream egress traffic of the running sandbox for $PWD
              connections labeled ALLOW/BLOCK, DNS queries inline
```
