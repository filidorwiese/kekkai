# Contract: `kekkai exec` CLI

## Invocation

```
kekkai exec [--] <command> [args...]
```

- No flags of its own. Every word after `exec` belongs to the user's command and is
  passed verbatim (FR-004); a single leading `--` is stripped if present.
- The container ID is always placed before the command in the underlying `docker exec`
  argv, so user words starting with `-` are never interpreted as docker flags.

## Execution contract

| Aspect | Behavior |
|---|---|
| Target | Running sandbox for `$PWD`, resolved by label `kekkai.cwd=$PWD` (same as `shell`) |
| Working dir | Container default `/workspace` (the mounted project) |
| User / env / network | Sandbox defaults — same user, env, and firewall restrictions as the claude workload (FR-007) |
| stdin | Always forwarded (`-i`) |
| TTY | Allocated (`-t`) only when kekkai's stdin is a terminal; piped stdin → no TTY |
| stdout / stderr | Streamed live to the caller's corresponding streams; kekkai adds nothing on stdout |
| Lifecycle | Never creates, restarts, or removes containers; sandbox keeps running after exec ends (FR-010) |

## Exit-code contract

| Outcome | Exit code |
|---|---|
| Command ran | The command's own exit code, verbatim (FR-002) |
| Command found but not executable | 126 (docker convention, passed through) |
| Command not found in sandbox | 127 (docker convention, passed through) |
| Docker daemon / exec setup failure | 125 (docker convention, passed through) |
| No running sandbox for `$PWD` | 1, nothing executed |
| No command given | 1, nothing executed |

## Error contract (stderr, exit 1, nothing executed)

| Condition | Message |
|---|---|
| No running sandbox | `no running sandbox for <pwd>, run 'kekkai up'` (same string family as `shell`) |
| No command given | `usage: kekkai exec [--] <command> [args...]` |

Per cmd/kekkai/main.go convention, errors print without a `kekkai:` prefix.

## Signals

- SIGINT/SIGTERM are forwarded to the docker CLI child (existing `docker.Interactive`
  behavior). With a TTY, ^C reaches the in-container command via the pty and it
  terminates; kekkai exits with the resulting nonzero code.
- **Known limitation** (non-TTY mode): on interrupt the docker CLI exits and the caller
  regains control with a nonzero exit, but the in-container process may keep running —
  `docker exec` does not proxy signals to exec'd processes (moby/moby#9098). The
  sandbox container is unaffected in all cases.

## Help text

`kekkai help` gains:

```
  exec        run a command in the running sandbox for $PWD
              args are passed verbatim; exits with the command's exit code
```
