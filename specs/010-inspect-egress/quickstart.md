# Quickstart: validating `kekkai watch`

End-to-end validation against a real docker daemon (constitution IV). Contract:
[contracts/watch-cli.md](contracts/watch-cli.md). Traffic is generated with
`kekkai exec` (feature 009).

## Prerequisites

- Docker daemon; `git`, `curl` on host.
- Build: `go build -o /tmp/kekkai-test ./cmd/kekkai`
- Test project dir; sandbox started under a pseudo-TTY
  (`script -qec "/tmp/kekkai-test up" /dev/null </dev/null &`, poll `docker ps`).
  This `up` rebuilds the image (tcpdump + firewall script are bake-time inputs) — expected.

## Scenarios

Watch runs in one terminal (`/tmp/kekkai-test watch > w.log` backgrounded for
scripted runs); traffic is generated from a second via `kekkai exec`. Expected
outcomes per row; any deviation fails.

| # | Action | Expect in stream |
|---|---|---|
| 1 | `exec curl -sI --max-time 5 https://example.com` | `DNS   query example.com`, `DNS   answer example.com -> <ip>...`, `BLOCK tcp <ip>:443 (example.com)` (US1, US2, FR-002/003/004) |
| 2 | `exec curl -sI --max-time 10 https://api.anthropic.com` | `DNS` lines + `ALLOW tcp <ip>:443 (api.anthropic.com)` (US1 all-traffic-labeled) |
| 3 | Add `example.org` to `.kekkai.yaml` allowed_domains (`example.com` itself must stay blocked — it is the §9.6 probe), `down` + `up`, `exec curl -sI --max-time 10 https://example.org` before and after | Destination flips `BLOCK` → `ALLOW` (US1 scenario 3, SC-004) |
| 4 | `exec curl --max-time 5 -sI http://1.1.1.1` (raw IP, no DNS) | `BLOCK tcp 1.1.1.1:80` with no hostname annotation (edge case) |
| 5 | Watch started only AFTER sandbox already ran for a while | Events still appear (FR-009 attach-anytime, no restart) |
| 6 | Ctrl+C the watch (pty `\003` trick from feature 009) | Exit 0; `docker exec <id> pgrep -x tcpdump` finds nothing (FR-006, cleanup) |
| 7 | Run watch again immediately | Streams again — repeatable (FR-006) |
| 8 | `watch` in a dir with no sandbox | `no running sandbox for <pwd>, run 'kekkai up'` on stderr, exit 1 (FR-007) |
| 9 | `watch extra-arg` | usage error, exit 1 (contract) |
| 10 | Sandbox with `network.allow_all: true`: repeat scenario 1 | `DNS` + `ALLOW` lines, no `BLOCK` anywhere (FR-010) |
| 11 | `exec sh -c 'for i in 1 2 3 4 5; do curl -sI --max-time 2 https://example.com; done'` | First BLOCK printed immediately; repeats within 5s suppressed; log stays readable (assumption: dedup) |
| 12 | While watch runs: `exec curl -sI https://api.anthropic.com` works; after `down`, watch exits | Verdicts unaffected by observation (FR-005); `sandbox stopped` on stderr, exit 1 |
| 13 | Regression: fresh `up` output | §9.6 probes still run and pass (example.com blocked, api.anthropic.com reachable); startup unchanged apart from image rebuild |
| 14 | Old-image case: retag a pre-feature image and start container from it (or skip if none available), run `watch` | `sandbox image predates 'kekkai watch'...` error, exit 1 (research R5) |

## Cleanup

```sh
docker rm -f $(docker ps -q --filter label=kekkai.cwd=$PWD)
docker volume rm <kekkai-history-hash-of-THIS-test-dir-only>   # never bulk-remove kekkai-history-*
```
