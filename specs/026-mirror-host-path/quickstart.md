# Quickstart: validate mirrored project path

Prerequisites: docker daemon, `task build` (binary at `/tmp/kekkai`), two throwaway projects `A` and `B` under `/tmp` (macOS: under `~/`). Pseudo-TTY trick for `up` per memory note. Rebuild once after the template change.

| # | Scenario | Steps | Expected |
|---|---|---|---|
| 1 | cwd mirrored | in `A`: `kekkai up`, then `kekkai exec pwd` | prints `A`'s host real path |
| 2 | project key | `kekkai exec ls /home/kekkai/.claude/projects` after a Claude turn | dir named after `A` (`-tmp-A`), no writes to `-workspace` (check mtime) |
| 3 | isolation | have Claude save a memory in `A`; `kekkai down`; `kekkai up` in `B`; ask Claude to recall | memory absent in `B`; `projects/<keyB>` created |
| 4 | host sharing | save a memory in `A` with host `claude`; `kekkai up` in `A`; ask to recall | memory present; same `projects/<keyA>/memory` dir |
| 5 | resume across sides | start a session in the sandbox, exit, `claude --resume` on host in `A` | session listed and resumable |
| 6 | symlink | `ln -s A A-link; cd A-link; kekkai up; kekkai exec pwd` | prints real path of `A`; `docker ps` label `kekkai.cwd` = real path; `kekkai down` from `A-link` and from `A` both find it |
| 7 | config ro | `kekkai exec sh -c 'echo x >> .kekkai.yaml'` | fails read-only; file readable at `P/.kekkai.yaml` |
| 8 | placeholder | project without `.kekkai.yaml`: `up`, `exec cat .kekkai.yaml`, `down` | comment-only placeholder shown; empty remnant removed on host after exit |
| 9 | git ro | `git.enabled: false`: `kekkai exec git log -1` / `git commit --allow-empty -m x` | log works, commit fails |
| 10 | secrets | `secrets.hide: [.env, certs/]`: `exec cat .env`, `exec ls certs` | empty file, empty dir |
| 11 | user mount | `disk.mounts: [{source: ~/.aws, readonly: true, optional: true}]` | mounted at `/home/kekkai/.aws` as before |
| 12 | shell/exec cwd | `kekkai shell` then `pwd`; `kekkai exec pwd` | both `P` |
| 13 | traffic/mpr/down | run each from `A` | resolve the sandbox as before |
| 14 | history | run commands in `kekkai shell`, `down`, `up`, `history` | previous commands present |
| 15 | image reuse | `docker images kekkai` before/after `up` in `A` and `B` | one tag, no rebuild for `B` |
| 16 | spaces/unicode | project at `/tmp/my proj ü`: `up`, `exec pwd` | starts; prints the exact path |
| 17 | colon rejected | project at `/tmp/a:b`: `up` | exits 1 before docker with the charset error |
| 18 | protected rejected | `cd /home && kekkai up` (or `/`) | exits 1 before docker naming `/home/kekkai` |
| 19 | prompt text | first Claude reply / `exec env` `KEKKAI_SYSTEM_PROMPT` | contains "project directory (mounted at the same path as on the host)" |
| 20 | firewall probes | every `up` above | §9.6 probes still run and pass |
| 21 | macOS unshared (Docker Desktop) | project in a folder removed from File Sharing: `up` | preflight `bind` failure naming `P` with the File Sharing hint |
| 22 | orphan untouched | after all runs | `~/.claude/projects/-workspace` unchanged (mtime) |

Also: `go build ./... && go vet ./...`; `grep -rn /workspace --exclude-dir=specs .` returns nothing outside git history.
