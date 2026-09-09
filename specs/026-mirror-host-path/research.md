# Research: Mirror the host project path

## R1. What Claude Code keys by cwd, and where it lives

**Finding**: Two stores, keyed by the absolute working directory:

| Store | Location | Keyed by | Contents |
|---|---|---|---|
| project dir | `$CLAUDE_CONFIG_DIR/projects/<sanitized-cwd>/` | cwd with `/` and `.` → `-` (`/home/fili/Projects/kekkai` → `-home-fili-Projects-kekkai`) | auto-memory (`memory/`), session transcripts |
| projects map | `$CLAUDE_CONFIG_DIR/.claude.json` → `projects[<cwd>]` | raw absolute cwd | trust dialog, legacy allowedTools, MCP servers, prompt history |
| project-local settings | `<cwd>/.claude/settings.local.json` | lives in the project | permission rules |

Host evidence (2026-09-09): `~/.claude/projects/` holds `-workspace` next to per-project keys; `~/.claude/.claude.json` (the sandbox's file, because `CLAUDE_CONFIG_DIR=/home/kekkai/.claude` maps there) and `~/.claude.json` (the host default) are two distinct files.

**Consequence**: with the mirrored path, the sandbox writes to `projects/-home-fili-Projects-kekkai/`, the same directory the host uses → memory and sessions are per-project and host-shared. The `.claude.json` projects map becomes per-project inside the sandbox (keyed by the real path), but it is host-shared only when the host also runs with `CLAUDE_CONFIG_DIR=~/.claude`; by default the host uses `~/.claude.json`. Project-local settings were already shared via the workspace bind. Spec story 2 / SC-002 amended to say exactly this.

**Decision**: no attempt to unify `.claude.json` with the host default file; out of scope (would mean binding a host file over a file inside an already-bound dir, and changes host Claude behavior).

## R2. Symlinked invocation directory

**Finding**: `os.Getwd()` returns `$PWD` when it still resolves to the cwd, so a symlinked path survives. Claude Code (node) uses `process.cwd()`, which is the kernel cwd, i.e. the resolved real path. Docker resolves bind sources itself but the destination and `-w` are taken literally.

**Decision**: one helper `projectDir()` = `filepath.EvalSymlinks(os.Getwd())`, used by every subcommand (`up`, `down`, `shell`, `exec`, `traffic`, `mpr`, `init`, preflight). The label `kekkai.cwd`, container name hash and history-volume hash all derive from the resolved path, so all consumers agree. Side effect: for symlinked invocations only, the container name and history volume name change once (new hash); documented.

**Alternatives**: resolve only for the mount and keep raw path for labels → two notions of "the project", subcommands could disagree. Rejected.

## R3. Mount syntax and unsupported characters

**Finding**: `docker run -v src:dst[:opts]` splits on `:`; a colon anywhere in the path is unrepresentable. `--mount type=bind,src=…,dst=…` splits on `,` (CSV, quoting possible but fragile). Spaces, unicode and most punctuation pass through untouched because args are exec'd, not shell-parsed.

**Decision**: keep `-v` everywhere (one syntax in the codebase; preflight and secret shadows already use it). Reject a project path containing `:` host-side, before any docker call (FR-009). Also reject `\n`/`\r` and other C0 control characters: docker labels and names are line-oriented in `docker ps` output that `ContainersByLabel` parses. Rejected-character set is a code constant and documented in the contract. Spaces and non-ASCII are supported and covered by quickstart.

**Alternatives**: `--mount` to allow colons → would need CSV quoting for commas, and secret shadows use `-v /dev/null:<path>:ro` too. Rejected for surface.

## R4. Paths that would overlay sandbox internals

**Finding**: docker creates missing parent directories of a bind destination (root-owned, 0755) and mounts over an existing directory. A project bound at `/home/kekkai` would hide `.bashrc`, `.nvm`, `.claude` mountpoint ordering issues; `/usr/local/bin` would hide `claude`, `init-firewall.sh`, `kekkai-mpr`; `/commandhistory` would collide with the history volume.

**Decision**: reject when the resolved path is `/` or is equal to or an ancestor of any protected in-container path. Protected list (code constant): `/home/kekkai`, `/usr/local/bin`, `/commandhistory`. Ancestor check by path components, not string prefix (`/home/kekkai2` is fine). Paths *below* protected dirs (e.g. `/home/kekkai/proj` for a host user named kekkai) are allowed: a nested mount hides nothing.

**Alternatives**: forbid all system dirs (`/usr`, `/etc`, …) → arbitrary, blocks nothing dangerous that the boundary cares about (the container is disposable; only kekkai's own tooling matters). Rejected.

## R5. Image: WORKDIR and the pre-created `/workspace`

**Finding**: `WORKDIR /workspace` and `mkdir -p /workspace` are in `embed/Dockerfile.tmpl`; the project path must never enter the template (constitution: bake-time inputs only, one image for all projects). `docker run -w` and `docker exec -w` (API ≥ 1.35, Docker ≥ 17.12; host has 29.x) both accept an arbitrary path that exists at exec time.

**Decision**: drop `/workspace` from `mkdir`/`chown`, set `WORKDIR /home/kekkai` (a neutral, always-present dir; only matters for exec paths that forget `-w`). Working directory is a runtime input: `docker run -w <path>` and `docker exec -w <path>` for `shell`/`exec`. Template text changes → one expected rebuild on upgrade (same as every template edit).

**Alternatives**: keep `WORKDIR /workspace` untouched to avoid the rebuild → stale dir contradicting FR-007. Rejected.

## R6. macOS preflight

**Finding**: preflight already binds `$PWD` read-only at `/kekkai-probe/workspace`. A shared-path failure is detected regardless of destination; but FR-011 wants the probe to exercise the real destination so path-shape problems (long paths, unusual chars the runtime rejects as a destination) surface in the same place with the same hint table.

**Decision**: probe binds `<pwd>:<pwd>:ro`; other probe binds unchanged. Failure text and hint table (`specs/002-macos-support/contracts/preflight.md`) unchanged. Linux: no probe; only the host-side checks from R3/R4.

## R7. Sandbox awareness prompt wording

**Finding**: prompt says "only the workspace … are visible" and ".kekkai.yaml in the workspace root". Both still read correctly, but "workspace" no longer names a path and the agent benefits from knowing the project sits at its host path (it may see it in memory files and session metadata).

**Decision**: replace the two occurrences with "the project directory (mounted at the same path as on the host)" and "in the project root". New text is pinned in `contracts/sandbox-layout.md`, superseding those lines in `specs/011-sandbox-awareness/contracts/sandbox-prompt.md`. Summary section unchanged.

## R8. Orphaned state

**Finding**: `~/.claude/projects/-workspace/` and `projects["/workspace"]` in `~/.claude/.claude.json` remain after upgrade, unused. Nothing reads them; nothing depends on them.

**Decision**: leave untouched, no notice (FR-012, spec assumption). README gets one sentence under the upgrade/limitations notes: safe to delete `~/.claude/projects/-workspace`.

## R9. Blast radius of `/workspace` literals

`grep -rn /workspace` (excluding specs): `internal/runtime/up.go` (mount targets ×4, secret shadows, `-w`), `embed/Dockerfile.tmpl` (mkdir/chown, WORKDIR), `internal/runtime/preflight_darwin.go` (probe path), `SPECIFICATION.md` §5.2/§7.3/§7.4/§8. Word "workspace" only: `sandboxprompt.go`, `init.go` template comment, `README.md` none, `embed/init-firewall.sh` none, `embed/kekkai-mpr.py` none, tests none. `WORKSPACE=<basename>` env stays (reserved key, §4.3).
