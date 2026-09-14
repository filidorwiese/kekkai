# Contract: sandbox lifecycle with an init process

## `docker run` argument prefix (`buildRunArgs`, §7.3)

```
run --rm -it --init --name <name> --label kekkai.cwd=<P> --label kekkai.image_hash=<h> --label kekkai.version=<v> --cap-add NET_ADMIN --cap-add NET_RAW …
```

`--init` is fixed and non-configurable, same class as `--rm -it`. Everything after it is unchanged.

## Observable guarantees

| Guarantee | How verified (quickstart) |
|---|---|
| PID 1 in the sandbox is `docker-init`; claude is its child and the tty foreground process | `kekkai exec ps -o pid,ppid,pgid,tpgid,comm` |
| No `Z`-state processes accumulate over a session with subprocess-spawning statusline/hooks | host `ps` filtered on the container's claude PID; in-sandbox `ps` |
| Ctrl-C reaches claude only, once | pty `^C` test |
| `kekkai down` stops and removes within the same grace period | timing of `kekkai down` |
| Resize redraws claude | resize the terminal |
| `kekkai up` exit status = claude's; 128+S when signal-killed | `kekkai exec` is not a proxy for this; use `CLAUDE_ARGS`-free run ending in a known exit, or `docker inspect .State.ExitCode` |
| No image rebuild on upgrade | `docker images kekkai` unchanged after `up` |
| Daemon without docker-init fails before any sandbox process, naming `docker-init` | daemon-side error text (not reproducible on supported hosts; documented) |

## `SPECIFICATION.md` amendments (same commit)

- §7.2 Lifecycle: `docker run --rm -it --init`; state that docker-init (tini) is PID 1, reaps orphans, forwards signals, exits with claude's status; claude is its child and the tty foreground process group. Keep existing CMD description.
- §7.3 Run args assembly: prefix `--rm -it --init` before `--cap-add …`.

## Unchanged by design

`internal/docker/exec.go`, the Dockerfile CMD, `preflight_darwin.go`, `shell`/`exec`/`down`, config schema, README (no user-facing change; optional one-line mention not required).
