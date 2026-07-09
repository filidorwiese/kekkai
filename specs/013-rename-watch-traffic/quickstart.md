# Quickstart: validating the `kekkai traffic` rename

End-to-end validation against a real docker daemon (constitution IV).
Contract: [contracts/traffic-cli.md](contracts/traffic-cli.md). This re-runs
the load-bearing feature 010 scenarios under the new name — full behavioral
coverage lives in [specs/010-inspect-egress/quickstart.md](../010-inspect-egress/quickstart.md).

## Prerequisites

- Docker daemon; `git`, `curl` on host.
- Build: `go build -o /tmp/kekkai-test ./cmd/kekkai`
- Test project dir; sandbox started under a pseudo-TTY
  (`script -qec "/tmp/kekkai-test up" /dev/null </dev/null &`, poll `docker ps`).
  This `up` rebuilds the image once (firewall script comments changed §6.1) — expected.
- Never add `example.com` to allowed_domains (it is the §9.6 probe target).

## Scenarios

| # | Action | Expect |
|---|---|---|
| 1 | `kekkai help` | `traffic` entry present with contract wording; no `watch` anywhere in output |
| 2 | `traffic` in a dir with no sandbox | `no running sandbox for <pwd>, run 'kekkai up'` on stderr, exit 1 |
| 3 | `traffic extra-arg` | `usage: kekkai traffic (no arguments)` on stderr, exit 1 |
| 4 | `watch` (with and without a running sandbox) | `unknown command "watch"` + usage listing `traffic`, exit 1 (US2) |
| 5 | With sandbox running: `traffic` + `exec curl -sI --max-time 5 https://example.com` | `DNS   query example.com`, `DNS   answer ...`, `BLOCK tcp <ip>:443 (example.com)` — formats identical to 010 (SC-001) |
| 6 | `exec curl -sI --max-time 10 https://api.anthropic.com` while streaming | `ALLOW tcp <ip>:443 ...` line; curl exit 0 (verdicts unaffected) |
| 7 | Ctrl+C the stream (pty `\003`) | exit 0; `exec pgrep -x tcpdump` finds nothing |
| 8 | `down` while a second `traffic` runs | `sandbox stopped` on stderr, exit 1 |
| 9 | Old-image case: container from a pre-013 image (or skip if none), run `traffic` | `sandbox image predates 'kekkai traffic'; run 'kekkai down' and 'kekkai up' to rebuild`, exit 1 |
| 10 | Fresh `up` output | §9.6 probes run and pass; firewall behavior unchanged (FR-005) |
| 11 | `grep -ri "kekkai watch" cmd/ internal/ embed/ SPECIFICATION.md README.md` | no hits (SC-003; specs/010 history exempt) |

## Cleanup

```sh
docker rm -f $(docker ps -q --filter label=kekkai.cwd=$PWD)
docker volume rm <kekkai-history-hash-of-THIS-test-dir-only>   # never bulk-remove kekkai-history-*
```
