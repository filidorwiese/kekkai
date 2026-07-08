# Quickstart: validating `kekkai exec`

End-to-end validation against a real docker daemon (constitution IV). Contract
details: [contracts/exec-cli.md](contracts/exec-cli.md).

## Prerequisites

- Docker daemon running; `git`, `curl` on host.
- Build: `go build -o /tmp/kekkai-test ./cmd/kekkai`
- A test project dir with a sandbox up. Non-interactive tip (see project memory):
  `kekkai up` ends in `docker run -it`, so start it under a pseudo-TTY:

  ```sh
  cd <test-project>
  script -qec "/tmp/kekkai-test up" /dev/null </dev/null &
  # poll until: docker ps --filter label=kekkai.cwd=$PWD shows the container
  ```

## Scenarios

Run all from the test project dir. Expected outcomes per row; any deviation fails.

| # | Command | Expect |
|---|---|---|
| 1 | `/tmp/kekkai-test exec ls /workspace` | Project listing on stdout; exit 0 (US1) |
| 2 | `/tmp/kekkai-test exec false; echo $?` | `1` (FR-002) |
| 3 | `/tmp/kekkai-test exec sh -c 'exit 42'; echo $?` | `42` (FR-002) |
| 4 | `/tmp/kekkai-test exec ls -la /workspace` | Flag `-la` reaches `ls`, not kekkai/docker (FR-004) |
| 5 | `/tmp/kekkai-test exec -- ls` | Leading `--` stripped; listing; exit 0 |
| 6 | `echo hi \| /tmp/kekkai-test exec cat` | `hi` on stdout; exit 0; no TTY error (US2, R1) |
| 7 | `/tmp/kekkai-test exec sh -c 'echo out; echo err >&2' >o 2>e` | `o` contains `out`, `e` contains `err` (FR-003) |
| 8 | `/tmp/kekkai-test exec nosuchcmd; echo $?` | `127`, docker's not-found error on stderr |
| 9 | `/tmp/kekkai-test exec whoami && /tmp/kekkai-test exec pwd` | Sandbox user (not root, not host user); `/workspace` (FR-007) |
| 10 | `/tmp/kekkai-test exec curl -sI --max-time 5 https://example.com; echo $?` | Nonzero — firewall blocks non-allowlisted destination (FR-007) |
| 11 | `/tmp/kekkai-test exec sleep 300` then Ctrl+C (interactive terminal) | Prompt returns promptly, nonzero exit; `docker ps` still shows sandbox (FR-008) |
| 12 | `/tmp/kekkai-test exec` (no command) | `usage: kekkai exec [--] <command> [args...]` on stderr; exit 1 (FR-006) |
| 13 | From a dir with no sandbox: `/tmp/kekkai-test exec ls; echo $?` | `no running sandbox for <pwd>, run 'kekkai up'` on stderr; `1`; nothing ran (US3) |
| 14 | After all of the above: `docker ps --filter label=kekkai.cwd=$PWD` | Sandbox still running (FR-010) |

## Cleanup

```sh
docker rm -f $(docker ps -q --filter label=kekkai.cwd=$PWD)
docker volume rm <kekkai-history-...>   # volume created by the test `up`
```
