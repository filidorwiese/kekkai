# Data Model: Read-Only Config Mount

No new persistent entities; this feature manipulates three file-shaped concepts and one error value.

## Config file (`.kekkai.yaml`)

- **Location**: workspace root, host-owned. The only recognized configuration name (FR-001).
- **In-container view**: `/workspace/.kekkai.yaml`, read-only bind, content = host content at startup.
- **States**: present (regular file, possibly empty/comments-only = defaults) | absent (→ placeholder takes its slot).
- **Validation**: unchanged (`config.Load` strict parse, one-pass report §4.4).

## Typo file (`.kekkai.yml`)

- **Definition**: any directory entry named `.kekkai.yml` at the workspace root — regular file, directory, symlink (incl. dangling), anything `os.Lstat` sees.
- **Behavior**: never read. `Discover` → typo error; `up` exits non-zero before docker work; `init` refuses to write.
- **Precedence**: typo error beats every other discovery outcome, including valid `.kekkai.yaml` present (FR-003) and no-config (placeholder never mounted while typo file exists — `up` already exited).

## Config placeholder

- **Content** (exact, one line + trailing newline):
  `# no .kekkai.yaml in workspace - kekkai runs on defaults; create one on the host ('kekkai init') to customize`
- **Host backing**: file in fresh temp dir under `<UserCacheDir>/kekkai/`, mode 0444 (cache dir, not `$TMPDIR` — macOS VM file-sharing, see research R3).
- **Lifecycle**: created in `buildRunArgs` only when `.kekkai.yaml` absent → mounted ro at `/workspace/.kekkai.yaml` → after `docker.Interactive` returns: temp dir removed, plus the empty mountpoint remnant docker materialized in the workspace (guarded: only an empty regular file is deleted).
- **Invariants**: kekkai itself never writes into the workspace; after normal exit the workspace has no `.kekkai.yaml` and no persistent host artifact remains; semantics identical to comments-only config (= all defaults, §4.1).

## Discovery error values (`internal/config`)

| Value | Trigger | Consumer behavior |
|-------|---------|-------------------|
| `ErrNoConfig` | neither file exists | `up`: warn + `Defaults()` (unchanged); others: hard error |
| typo error (new) | `.kekkai.yml` entry exists | fatal everywhere; `up` exits non-zero pre-docker |

State transition for `Discover(dir)`:

```text
.kekkai.yml entry exists ──────────────► typo error (regardless of .kekkai.yaml)
else .kekkai.yaml regular file exists ─► path, nil
else .kekkai.yaml entry exists ────────► "not a regular file" error (dir, dangling symlink)
else ──────────────────────────────────► "", ErrNoConfig
```
