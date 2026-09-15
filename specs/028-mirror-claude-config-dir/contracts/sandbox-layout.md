# Contract: Claude config dir layout (specs/028)

Supersedes the `~/.claude → /home/kekkai/.claude` bullet of §5.2 and the
`CLAUDE_CONFIG_DIR=/home/kekkai/.claude` value of §5.3.

## Resolution

```
CFG := $CLAUDE_CONFIG_DIR  if set and non-empty  (filepath.Abs + Clean)
    := $HOME/.claude       otherwise             (os.UserHomeDir, unresolved)
```

Helper: `ClaudeConfigDir()` in `internal/runtime/identity.go`. Every consumer
(`Up`, `buildRunArgs`, darwin `preflight`) calls it; no other code derives the path.

## Run args

```
-v CFG:CFG                      # rw, builtin mount block, after the .kekkai.yaml overlay, before history volume
-e CLAUDE_CONFIG_DIR=CFG        # builtin env block, first entry, user env may override
```

## Validation (one-pass report, §4.4 format)

| Condition | Message |
|-----------|---------|
| CFG == `/` | `claude config dir / cannot be mirrored into the sandbox (root directory)` |
| CFG ancestor of protected path P | `claude config dir CFG cannot be mirrored into the sandbox: it would overlay P` |
| CFG contains `:` or control char | `claude config dir CFG contains ':' or control characters, which the container runtime cannot express as a mount destination; move it or set CLAUDE_CONFIG_DIR` |
| disk.mounts[i] target == CFG or beneath | `disk.mounts[i]: target T would shadow the Claude config dir CFG` |

Project path messages keep their existing text; only the label word differs.

## Darwin preflight

Probe bind `CFG:/kekkai-probe/claude:ro` replaces `~/.claude:/kekkai-probe/claude:ro`.
Hint tables unchanged.

## Container start (CMD, first step)

```sh
if [ "$CLAUDE_CONFIG_DIR" != /home/kekkai/.claude ]; then ln -sfn "$CLAUDE_CONFIG_DIR" /home/kekkai/.claude; fi && <existing CMD>
```

Dockerfile `mkdir -p` line no longer creates `/home/kekkai/.claude`. The
template contains no host path; one image serves every user.

## Sandbox prompt filesystem line

```
- Filesystem: only the project directory and your Claude config directory
  (both mounted at the same path as on the host) plus explicitly configured
  mounts are visible.
  Some files may be shadowed (present but empty) because they contain secrets.
```

## Observable inside the sandbox

| Check | Expected |
|-------|----------|
| `echo $CLAUDE_CONFIG_DIR` | CFG |
| `readlink /home/kekkai/.claude` | CFG (or not a link when CFG is that path) |
| `claude plugin list` | identical to host |
| `ls CFG/plugins/installed_plugins.json` | present |
