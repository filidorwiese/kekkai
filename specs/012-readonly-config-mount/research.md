# Research: Read-Only Config Mount

No NEEDS CLARIFICATION markers remained in the Technical Context; research resolved the four implementation unknowns below against the existing codebase and docker semantics.

## R1: Where does the typo check live?

**Decision**: In `config.Discover` (internal/config/config.go). It stops recognizing `.kekkai.yml` as config and instead returns a dedicated error when a `.kekkai.yml` entry exists — checked with `os.Lstat` so directories, symlinks (including dangling), and other non-regular entries all trigger it.

**Rationale**: `Discover` is the single choke point for config discovery; every consumer (today: `up` via `Load`, `init` via its own stat loop) inherits FR-001 automatically. In `Up`, `Load` runs before any docker work, satisfying FR-002's "before any sandbox resources" for free. The typo error is distinct from `ErrNoConfig`, so the defaults fallback path in `up.go:61-64` cannot swallow it.

**Alternatives considered**:
- Separate check inside `Up` only — rejected: leaves `.kekkai.yml` silently readable by any future config consumer, violating FR-001's "any kekkai command".
- Treating `.kekkai.yml` as `ErrNoConfig` (silent defaults) — rejected by spec: silently dropping user restrictions is a security regression (Story 3 rationale).

## R2: How to expose `.kekkai.yaml` read-only when the workspace is already mounted rw?

**Decision**: A single-file bind mount layered over the workspace bind: `-v $PWD/.kekkai.yaml:/workspace/.kekkai.yaml:ro`, appended right after the `$PWD:/workspace` mount in `buildRunArgs` (builtin mounts group, §7.3 order preserved).

**Rationale**: Docker resolves nested mounts by mount order; the later, more specific file mount shadows the path inside the rw workspace bind. Writes, truncates, deletes, and renames against a ro file mount point all fail (EROFS/EBUSY), and the container has no SYS_ADMIN to remount — same enforcement pattern the project already uses for the `.git` ro bind (§5.2). Host file stat-gated with `fileExists` before adding the mount, so docker never creates host artifacts.

**Alternatives considered**:
- Copying the config into the image — rejected: runtime config must never trigger a rebuild (§6.1, constitution constraint).
- Mounting to a different path (e.g. `/etc/kekkai.yaml`) — rejected: agent (and its tooling) expects the config at the workspace root; also leaves the real workspace file writable, reopening the escalation path.

**Known caveat (accepted)**: single-file binds pin the host inode. Editors that save via rename (vim, most IDEs) break content sync into a *running* container. Spec explicitly does not require live tracking — config only applies at startup.

## R3: How to provide the comment-only placeholder without touching the workspace?

**Decision**: When no `.kekkai.yaml` exists, write the fixed comment line to a file in a fresh temp dir under `<UserCacheDir>/kekkai/` (mode 0444), bind-mount it ro to `/workspace/.kekkai.yaml`, and remove the temp dir after `docker.Interactive` returns. Under the cache dir rather than `os.TempDir()` because a bind source must be daemon-visible: on macOS only the home directory is shared into the runtime VM by all recognized runtimes (colima does not share `$TMPDIR`).

**Rationale**: A bind mount needs a host-side source; the workspace is off-limits (FR-005) and the image can't carry it (rebuild rule). `docker.Interactive` starts the docker CLI and waits (`cmd.Start` + wait loop in internal/docker/exec.go) — kekkai stays alive for the container's lifetime, so a deferred cleanup reliably removes the placeholder. Comments-only file = all defaults per existing §4.1 semantics, so the agent parsing it sees exactly the active configuration.

**Alternatives considered**:
- `/dev/null:...:ro` (existing secrets-shadow pattern) — rejected by clarification: placeholder must carry the explanatory comment; /dev/null reads empty.
- Persistent placeholder under `~/.local/state/kekkai/` — rejected: leaves a permanent host artifact for no benefit; temp + defer is strictly cleaner given Interactive waits.
- tmpfs mount — rejected: tmpfs targets directories, not files, and starts empty.

**Finding from e2e (handled)**: docker materializes the bind target as an empty file *inside the workspace bind*, i.e. on the host, for the duration of the run — the same filesystem reality that makes §8 secrets shadows stat-gated. The cleanup therefore also removes this remnant at exit, guarded to empty regular files only so a config the user writes mid-run is never deleted.

**Known caveat (accepted)**: if kekkai is SIGKILLed the deferred cleanup is skipped: a ~100-byte file lingers under the cache dir and the empty mountpoint remnant stays in the workspace until the next successful run or manual removal. Hard-kill residue is standard CLI behavior.

## R4: `kekkai init` behavior with a typo file present

**Decision**: Split the existing combined stat loop (init.go:97): `.kekkai.yaml` present keeps the current "already exists, not overwriting" error; `.kekkai.yml` present gets a typo-specific refusal instructing rename/removal.

**Rationale**: FR-008 — writing a fresh `.kekkai.yaml` next to a typo file would leave two config-looking files and the user unaware the old one was never read. Message text pinned in contracts/config-mount.md.

**Alternatives considered**: auto-rename `.kekkai.yml` → `.kekkai.yaml` — rejected: silent mutation of user files; also the file may be stale/legacy-schema and worth a conscious look.
