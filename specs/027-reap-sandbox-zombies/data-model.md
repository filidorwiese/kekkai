# Data Model: Reap orphaned processes inside the sandbox

No persistent data. The model is the in-container process tree.

## Process tree (after)

| PID role | Process | Parent | Process group | Responsibility |
|---|---|---|---|---|
| 1 | `docker-init` (tini) | runtime | own | adopt + reap orphans; forward signals it receives to its child; exit with child's status |
| child of 1 | `sh -c <CMD>` → `exec claude` | 1 | own, tty foreground | run firewall init, mpr supervisor, then become claude |
| descendants | mpr loop, proxy, hooks, statusline helpers | claude (or their spawner) | claude's | as today |

## State transitions

| Event | Before (claude = PID 1) | After (`--init`) |
|---|---|---|
| Grandchild's parent exits | orphan re-parented to claude; node never `wait`s it → zombie until container end | orphan re-parented to tini; reaped on exit |
| `^C` on the tty | SIGINT to foreground group (claude) | same (claude is the foreground group) |
| `docker stop` / `kekkai down` | SIGTERM to claude | SIGTERM to tini → forwarded to claude |
| terminal resize | SIGWINCH to foreground group (claude) | same |
| claude exits N | container exits N | tini exits N → container exits N |
| claude killed by signal S | container exits 128+S | tini exits 128+S |
| CMD fails before `exec claude` | container exits sh's status | same via tini |

## Validation rules

- `--init` is unconditional: not a config key, not a flag (Principle III).
- Not a bake-time input: image hash inputs (§6.1) untouched.
