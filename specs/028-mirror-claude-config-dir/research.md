# Research: Mirror Claude Config Dir at Host Path

## R1: Resolve the host config dir with or without symlink resolution?

- **Decision**: No symlink resolution. `$CLAUDE_CONFIG_DIR` (made absolute, cleaned) or `os.UserHomeDir()/.claude` verbatim.
- **Rationale**: Host Claude Code writes `installPath`/`installLocation` from its own notion of the config dir, which is `CLAUDE_CONFIG_DIR` or `os.homedir()`, neither resolved. Verified on this host: `plugins/installed_plugins.json` records `/home/fili/.claude/plugins/cache/...`. The mount destination must equal that literal prefix, so the unresolved form is the correct one. `$PWD` (specs/026) resolves because Claude keys projects by kernel cwd; the config dir has no such kernel-derived key.
- **Alternatives considered**: `EvalSymlinks` like `ProjectDir` — would break users whose home is a symlink. Mounting both resolved and unresolved — two rw binds of one dir, needless.

## R2: Nested bind mounts (config dir inside project, project inside config dir)

- **Decision**: Rely on docker mount ordering; no special handling.
- **Rationale**: Docker sorts bind destinations by path depth before mounting, so a child destination always lands on top of its parent regardless of `-v` argument order. Project at `$HOME` gives `/home/fili` (rw) with `/home/fili/.claude` (rw) nested; both writable, config dir not shadowed. Project inside the config dir is the reverse nesting, same rule. Secrets shadows (`/dev/null` or tmpfs) are project-relative and also nest deeper, unaffected.
- **Alternatives considered**: Refusing project == config dir ancestor — would refuse running kekkai from `$HOME`, which works today. Rejected.

## R3: Where does "disk mounts must not shadow the config dir" live?

- **Decision**: New check in `Up` next to `ValidateProjectPath`, comparing each non-skipped mount's `ContainerPath` against the resolved config dir (equal or descendant). `protectedContainerPaths` is not extended.
- **Rationale**: `protectedContainerPaths` guards one thing today: the project path must not be an ancestor of kekkai-owned container locations. Adding the config dir there would refuse project at `$HOME` (see R2). Disk-mount targets have no protection at all today; docker itself only rejects an exact duplicate destination with an opaque error. A targeted check gives the one-pass §4.4 message and covers descendants (which docker would silently accept, hiding part of the plugin cache). The config package cannot host it: it has no access to runtime identity, and `Up` already merges runtime path checks into the same report.
- **Alternatives considered**: Generic "no disk mount under any builtin destination" rule — broader behavior change, out of scope. Docker-error-only — poor UX, no descendant coverage.

## R4: Runtime symlink placement and the pre-created directory

- **Decision**: Remove `/home/kekkai/.claude` from the Dockerfile `mkdir -p`; add a path-free `ln -sfn "$CLAUDE_CONFIG_DIR" /home/kekkai/.claude` guarded by `[ "$CLAUDE_CONFIG_DIR" != /home/kekkai/.claude ]` as the first CMD step.
- **Rationale**: `ln -s` refuses to replace an existing directory, so the bake-time mkdir must go. The env var is the single source, so the template never contains a user path (§6.1). When the mirrored path already is `/home/kekkai/.claude` (host user named `kekkai`), docker creates the mountpoint and the guard skips the link. Running before the startup echo keeps the visible sequence identical. `ln -sfn` is idempotent, so `--force` recreation or a restarted container is safe.
- **Alternatives considered**: `docker run --entrypoint` override from Go — would move CMD logic into the binary, split the contract. Symlink via `kekkai shell` only — Claude's own subprocesses would not see it. Keeping the mkdir and using `rmdir` in CMD — two steps where one suffices.

## R5: Image hash impact

- **Decision**: Accept one template change (mkdir line + CMD prefix). Hash still derives from bake-time inputs only.
- **Rationale**: The path is carried by runtime env, never rendered. Users on different homes share one tag; switching `CLAUDE_CONFIG_DIR` triggers no rebuild. A release that edits the template always rebuilds once, which is the normal update path.
- **Alternatives considered**: None viable without moving CMD logic to the host binary (see R4).

## R6: Env precedence for `CLAUDE_CONFIG_DIR`

- **Decision**: Builtin env, before user env. Not added to `reservedEnvKeys`.
- **Rationale**: Matches today (`CLAUDE_CONFIG_DIR` is already a builtin, overridable value). A user override is their choice; the CMD symlink then points wherever they said, which is coherent.
- **Alternatives considered**: Reserving the key — new restriction for no reported need.

## R7: Darwin preflight

- **Decision**: Probe source = `ClaudeConfigDir()`, destination unchanged.
- **Rationale**: The probe exists to catch VM file-sharing gaps for exactly the paths the run will bind. A relocated config dir outside `/Users` is precisely the case that needs the probe.
- **Alternatives considered**: None.
