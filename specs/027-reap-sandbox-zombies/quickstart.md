# Quickstart: validate orphan reaping

Prerequisites: docker daemon, `task build` (binary at `/tmp/kekkai`), a scratch project under `$HOME` (not `/tmp`: snap daemon has a private `/tmp`). Pseudo-TTY trick for `up` per memory note. No rebuild expected.

Statusline repro for scenario 2: in the sandbox's Claude settings (`~/.claude/settings.json`, shared with host) point `statusLine.command` at a script that spawns a nested subprocess and exits, e.g. `sh -c 'sh -c "(sleep 0.2) & exit 0"; echo ok'`. Restore the original afterwards.

| # | Scenario | Steps | Expected |
|---|---|---|---|
| 1 | init is PID 1 | `kekkai up`; `kekkai exec ps -o pid,ppid,pgid,tpgid,comm` | PID 1 `docker-init`; `claude` PPID 1, PGID = TPGID = its own PID |
| 2 | no zombies (in-sandbox) | with the repro statusline, interact for ~2 min (or `kekkai exec claude -p "hi"` several times), then `kekkai exec ps -eo pid,ppid,stat,comm` | zero `Z` entries |
| 3 | no zombies (host view) | host: `hp=$(docker inspect -f '{{.State.Pid}}' <id>)` (PID 1 of the container), then `ps -eo pid,ppid,stat,cmd \| awk -v p=$hp '$2==p && $3 ~ /Z/'` — NOT `docker top`: exited tasks leave the cgroup, so it never lists zombies | empty (control run: 7 lines) |
| 4 | control (defect) | same as 2 against the previous binary (no `--init`) | `Z` entries present — proves the repro is valid |
| 5 | Ctrl-C | `(sleep 3; printf '\003') \| script -qec "/tmp/kekkai exec sleep 300" /dev/null` and, in an interactive session, `^C` at the claude prompt | sleep exits 130; claude shows its interrupt behavior as on host, sandbox stays up |
| 6 | down | `kekkai down` from another terminal while a session runs | container gone within the usual grace period (`docker rm -f`, sub-second); `up` returns 137 as before |
| 7 | resize | resize the terminal window during a session | claude redraws to the new size |
| 8 | exit status 0 | quit claude normally (`/exit`) | `up` exits 0 |
| 9 | exit status non-zero | `kekkai exec` a wrapper is not PID 1; instead run `docker run --rm -it --init <image> sh -c 'exit 7'` (mirrors the run prefix) | host `$?` = 7; `kill -9 $$` variant → 137 |
| 10 | startup failure still fails | temporarily set `network.allowed_domains: [example.com]` (§9.6 must-block probe) and `up` | firewall verification fails, container exits non-zero, no hang; revert |
| 11 | mpr still supervised | `kekkai exec pkill -f '^python3 /usr/local/bin/kekkai-mpr serve'`; then `kekkai exec claude -p "hi"` | proxy restarted by the loop; request captured by `kekkai mpr` |
| 12 | no rebuild | `docker images kekkai` before and after first `up` on the new binary | same tags, no build output |
| 13 | firewall probes | every `up` above | §9.6 probes still run and pass |
| 14 | macOS (Docker Desktop, deferred) | scenarios 1, 2, 5, 8 on Apple silicon | identical results |

Also: `go build ./... && go vet ./...`; `grep -n '\-\-init' internal/runtime/up.go SPECIFICATION.md` shows the arg and the §7.2/§7.3 text.

## Results (2026-09-14, linux/amd64, snap Docker 29.6.1, image kekkai:638775c910d9, claude 2.1.270)

| # | Result |
|---|---|
| 1 | PASS — PID 1 `docker-init`; claude PID 7, PPID 1, PGID = TPGID = 7 |
| 2 | PASS — 0 `Z` after one turn + 3 exec-spawned orphans; orphan `sleep` gone within 1.5 s |
| 3 | PASS — 0 defunct under the container's PIDs on the host |
| 4 | PASS (defect reproduced) — control binary: PID 1 claude, 7 `Z` (`git`, `sh`, 5× `sleep`) after the same procedure |
| 5 | PASS — pty `^C` → `exec sleep` rc 130, container up; `^C` at the claude prompt clears input, claude and container keep running |
| 6 | PASS — `down` 0.39 s, `up` returned 137 (same as pre-change: `rm -f`) |
| 7 | PASS — `tmux resize-window` 213×58 → 100×30 reflected by `stty size` on the container pty; claude redrew |
| 8 | PASS — `/exit` → `up` exit 0 |
| 9 | PASS — `exit 7` → 7, `kill -9 $$` → 137, `kill -TERM $$` → 143; `docker kill -s TERM` on a live sandbox → `up` exit 143 |
| 10 | PASS — `allowed_domains: [example.com]` → "verification failed — https://example.com is reachable", rc 1 in 1 s, no container left |
| 11 | PASS — proxy pid 71 → 483 under the same supervisor `sh`; `claude -p` succeeded; `kekkai mpr` captured `POST /v1/messages`; 0 `Z` |
| 12 | PASS — the scratch project's config hash tag was built once on its first `up`; the control (pre-change) binary on the same project printed no `building image` line, i.e. `--init` is not a hash input |
| 13 | PASS — §9.6 probes ran and passed on every `up` |
| 14 | DEFERRED — see below |

## Deferred validation (macOS, Docker Desktop on Apple silicon)

Maintainer-validated per constitution IV. Run scenarios 1, 2, 5 and 8 above; expect identical results. Before that, `docker info --format '{{.InitBinary}}'` must print `docker-init`. The darwin preflight probe (§7.4) is untouched by this feature and must still pass.
