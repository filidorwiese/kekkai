# Contract: CLI

**Source of truth**: SPECIFICATION.md §3, §4.5, §7. Stdlib `flag` per subcommand; dispatch in
`cmd/kekkai/main.go`; logic in `internal/runtime/<name>.go`.

## Subcommands

| Command | Flags / args | Behavior | Exit |
|---------|--------------|----------|------|
| `kekkai init` | — | Write starter `.kekkai.yaml` (active keys = defaults, optional sections commented, README-grade comments, `GH_TOKEN` example by `allow_github`) | 0; ≠0 if config already exists |
| `kekkai up` | `--force` recreate; `--verbose` plain buildkit progress; args after `--` appended to claude args | Validate full config (all violations, one pass) → resolve version → build image if hash miss → assemble run args → `docker run --rm -it` → firewall init → exec claude | ≠0 on: missing/invalid config, both yml+yaml, existing container without `--force`, ssh_agent without host socket, build/run failure, firewall probe failure |
| `kekkai down` | — | Stop + remove container labeled `kekkai.cwd=$PWD` | 0 also when nothing to remove (report) |
| `kekkai shell` | — | `docker exec -it <container> zsh` resolved by label | ≠0 if no running sandbox for `$PWD` |
| `kekkai ps` | — | List containers with `kekkai.cwd` label: name, cwd, image hash, status | 0 |
| `kekkai prune` | `--volumes` include history volumes; `--yes` skip prompt | List orphan kekkai containers + unused `kekkai:*` images (+ volumes); confirm; remove. Never touches running sandboxes' resources | 0 |
| `kekkai version` | — | Print version (`-ldflags "-X main.version=..."`) | 0 |
| `kekkai help` | — | Usage | 0 |

## Global rules

- No `config`/`doctor` subcommands; unknown subcommand → usage + exit ≠0.
- Errors → stderr, human-readable; normal output → stdout.
- `up` error for missing config is exactly: `no .kekkai.yaml found, run 'kekkai init'`.
- SIGINT/SIGTERM during a session forward to the container; container removal guaranteed (`--rm`).
