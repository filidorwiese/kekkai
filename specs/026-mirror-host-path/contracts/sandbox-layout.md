# Contract: sandbox filesystem layout (mirrored project path)

`P` is the resolved real path of the directory `kekkai` was invoked from (symlinks resolved). Every consumer derives it from one helper.

## `docker run` (§7.3 amendment)

```text
docker run --rm -it --name kekkai-<base>-<h8> --label kekkai.cwd=P … \
  -v P:P \
  -v P/.kekkai.yaml:P/.kekkai.yaml:ro            # or staged placeholder → same dst
  -v ~/.claude:/home/kekkai/.claude \
  -v kekkai-history-<h8>:/commandhistory \
  [-v P/.git:P/.git:ro]                           # git.enabled: false
  [disk.mounts …] \
  [-v /dev/null:P/<hide>:ro | --tmpfs P/<hide>] \
  [-e …] [--cpus …] [--memory …] \
  -w P <image>
```

`kekkai shell` → `docker exec -it -w P <id> bash`; `kekkai exec` → `docker exec -i [-t] -w P <id> <cmd…>`.

## Host-side validation (all platforms, before any docker call)

Runs in `up` right after config validation. All failures reported in one pass, exit 1.

```text
kekkai: project path / cannot be mirrored into the sandbox (root directory)
kekkai: project path /home cannot be mirrored into the sandbox: it would overlay /home/kekkai
kekkai: project path /tmp/a:b contains ':' or control characters, which the container runtime cannot express as a mount destination; move or rename the project
```

Protected in-container paths (code constant): `/home/kekkai`, `/usr/local/bin`, `/commandhistory`. Equal-or-ancestor is rejected; descendants are allowed.

Supported and mirrored byte-exactly: spaces, unicode, `,`, `'`, `"`, `$`, `~` inside the path.

## macOS preflight (§7.4 amendment)

Probe bind for the project becomes `-v P:P:ro`; everything else in `specs/002-macos-support/contracts/preflight.md` (other binds, probe command, failure format, hint table) is unchanged. An unshared `P` surfaces as the existing `bind` failure naming `P`.

## Image (§6.3 amendment)

- No pre-created `/workspace`; `WORKDIR /home/kekkai`.
- The project path is never rendered into the template; image hash unaffected by which project runs.
- One rebuild on upgrade (template text changed), same as any template edit.

## Sandbox awareness prompt (supersedes two lines in `specs/011-sandbox-awareness/contracts/sandbox-prompt.md`)

Line 4 of the pinned text becomes:

```text
- Filesystem: only the project directory (mounted at the same path as on the
  host) and explicitly configured mounts are visible.
  Some files may be shadowed (present but empty) because they contain secrets.
```

and "add to .kekkai.yaml in the workspace root" becomes "add to .kekkai.yaml in the project root". Nothing else in the prompt or summary changes.

## Environment

Unchanged: `WORKSPACE=<basename P>`, `CLAUDE_CONFIG_DIR=/home/kekkai/.claude`, all others.

## Upgrade note (README)

Per-project Claude state previously accumulated under `~/.claude/projects/-workspace/` (and the `/workspace` entry in `~/.claude/.claude.json`) is no longer read by any sandbox. It is left in place and safe to delete by hand. Sandboxes started from a symlinked directory get a new container/history-volume name once (hash of the resolved path).
