# Research: Reap orphaned processes inside the sandbox

All findings verified 2026-09-09 on the Linux host (snap Docker 29.6.1 daemon, `docker info` reports `InitBinary: docker-init`) against the current sandbox image `kekkai:776eec2fc399`. macOS (Docker Desktop) not testable here; deferred to quickstart like specs/026.

## R1. Reproduce the defect and prove the fix

**Finding**: node as PID 1 leaves orphans as zombies; `--init` does not.

```
# node -e 'spawn("sh",["-c","(sleep 0.3; exit 0) & exit 0"]); setTimeout(ps, 1500)'
without --init:   PID 1 MainThread   |  PID 14  PPID 1  STAT Z  sh     <- zombie
with    --init:   PID 1 docker-init  |  PID 7 MainThread, no Z entries
```

A plain `sh` as PID 1 does *not* show the defect (dash reaps adopted children), which is why the bug only appears once `exec claude` makes node PID 1. Any repro must use node (or the real claude) as the init-less PID 1.

**Decision**: `--init` is the fix. Rejected: baking `tini` via apt (changes bake-time inputs → rebuild for every user, adds a package for nothing docker does not already ship); a shell wrapper as PID 1 (`sh -c '... claude; wait'`) — dash reaps but then claude is no longer the direct foreground process and signal/exit plumbing has to be hand-written, exactly what tini already does.

## R2. Exit-code propagation through docker-init

**Finding**: `docker run --rm --init IMG sh -c 'exit 7'` → host `$?` = 7; `sh -c 'kill -9 $$'` → 137. tini exits with the child's status, or 128+signal when the child is signal-killed. `docker.Interactive` already returns the docker CLI's exit code, which is the container's exit code. Nothing to change in `internal/docker/exec.go`.

## R3. Signal delivery with a TTY (Ctrl-C, `kekkai down`, resize)

**Finding** (`docker run -it --init`, under `script` pty):

```
PID  PPID  PGID  TPGID  COMMAND
1    0     1     7      docker-init
7    1     7     7      sh          <- own process group, and the tty's foreground group
```

tini puts the child in its own process group and hands it the controlling tty (`tcsetpgrp`). Consequences, each verified:

- **Ctrl-C**: `^C` byte through the pty → kernel sends SIGINT to the foreground group = claude only (trap fired, exit 3 propagated to host). tini itself is not in the foreground group, so it neither dies nor double-delivers.
- **`docker kill -s INT` / `docker stop` (SIGTERM)** → delivered to tini, forwarded to the child (trap fired, exit 3, `State.ExitCode=3`). `kekkai down` uses docker stop/rm → unchanged.
- **SIGWINCH**: changing the container pty's window size (`stty cols 77 < /dev/pts/0` from a root exec) fired the child's WINCH trap. The docker CLI relays host resizes via the resize API onto that pty, so claude redraws as before.

**Decision**: no changes to signal handling on the host side. §7.2 wording gains "docker-init (tini) is PID 1; claude is its child and the tty foreground process".

## R4. Interaction with the CMD (firewall, mpr loop, `exec claude`)

**Finding**: the CMD is `sh -c '... && ( mpr loop ) & kekkai-mpr wait ... && exec claude'`. With `--init` the `sh` becomes tini's child (PID 7-ish) and `exec claude` replaces it in place, so claude keeps the foreground pgrp tini assigned. The backgrounded mpr supervisor subshell stays a child of the shell → of claude after exec. When claude exits, tini receives SIGCHLD for its only child, exits with claude's status, and the container's remaining processes (mpr loop, python proxy) are killed by the runtime on container teardown as today. A startup failure before `exec claude` (`init-firewall.sh` or `kekkai-mpr wait` non-zero) makes `sh` exit non-zero → tini exits with that status → `up` fails as today. No hang path introduced (FR-008).

**Decision**: CMD untouched. Image hash unchanged (FR-005) — `--init` is a run arg, not a bake-time input.

## R5. Availability of docker-init across supported hosts

**Finding**: `docker-init` ships with the Docker Engine packages (deb/rpm/static), with the snap (verified here: `InitBinary: docker-init` on the snap daemon), and with Docker Desktop on macOS (bundled in the VM). OrbStack and colima run stock dockerd builds that include it. If the binary is missing the daemon refuses container creation: `docker: Error response from daemon: ... docker-init: no such file or directory` — an immediate, named failure before any sandbox process starts.

**Decision**: rely on the daemon's own error for FR-007; no extra preflight probe (Principle III, and §7.4 already says zero docker calls added to the happy path). The darwin preflight container does **not** get `--init` either: it probes bind sharing, not init, and the real run fails fast anyway.

## R6. Where the flag goes

**Finding**: `buildRunArgs` (`internal/runtime/up.go`) opens with `run --rm -it --name … --label … --cap-add …`. §7.3 lists the assembly order starting at `--cap-add`.

**Decision**: add `"--init"` immediately after `-it`, before `--name`, as a fixed non-configurable arg alongside `--rm -it`. §7.2 (lifecycle) and §7.3 (assembly order prefix) amended in the same commit. `kekkai shell`/`exec` are `docker exec` into the running container and inherit its init; no change.
