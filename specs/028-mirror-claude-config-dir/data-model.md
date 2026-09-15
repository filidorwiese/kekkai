# Data Model: Mirror Claude Config Dir at Host Path

No persisted data. Runtime values only.

## Host config dir

| Field | Type | Source | Rules |
|-------|------|--------|-------|
| path | absolute string | `$CLAUDE_CONFIG_DIR` (non-empty) else `<UserHomeDir>/.claude` | cleaned; relative env value made absolute against cwd; symlinks NOT resolved (research R1) |

Validation (all reported in the one-pass `up` report):

- not `/`
- not an ancestor of `/home/kekkai`, `/usr/local/bin`, `/commandhistory` (component-wise)
- no `:` or C0 control bytes
- created on the host (`0700`) if missing, before mounting

## Mirrored config dir

Same string as above, used three times:

| Use | Form |
|-----|------|
| bind | `-v <path>:<path>` (rw) |
| env | `-e CLAUDE_CONFIG_DIR=<path>` (builtin block, before user env) |
| darwin probe | `-v <path>:/kekkai-probe/claude:ro` |

Relationships:

- may nest inside the project bind or contain it (docker depth ordering, research R2)
- `disk.mounts[i].ContainerPath` MUST NOT equal it or descend from it

## Convenience symlink

| Field | Value |
|-------|-------|
| link | `/home/kekkai/.claude` |
| target | `$CLAUDE_CONFIG_DIR` as seen inside the container |
| created | CMD, first step, `ln -sfn` |
| skipped when | target equals link path |

State: exists only for the container lifetime; never in the image.
