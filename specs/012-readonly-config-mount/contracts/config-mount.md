# Contract: Config Mount & Typo Rejection

Exact user-visible strings and docker-level mount specs. Implementation must match byte-for-byte; e2e validation greps for these.

## 1. Typo error (`kekkai up`, any config consumer)

Trigger: `.kekkai.yml` directory entry exists at the workspace root (any file type).

```text
found .kekkai.yml - kekkai only reads .kekkai.yaml; rename it: mv .kekkai.yml .kekkai.yaml
```

- Emitted from `config.Discover`; surfaces on stderr via the existing `up` error path.
- Exit status: non-zero (1).
- Must fire before image/container/preflight work (it does: `config.Load` is the first step of `Up`).
- Fires also when `.kekkai.yaml` coexists. Supersedes and removes the old message `both .kekkai.yml and .kekkai.yaml exist, remove one`.

Related rejection — `.kekkai.yaml` exists but is not a regular file (directory, dangling symlink; the ro bind needs a regular file):

```text
.kekkai.yaml is not a regular file - remove or rename it
```

## 2. `kekkai init` refusals

| Precondition | Message (stderr, exit non-zero) |
|--------------|--------------------------------|
| `.kekkai.yaml` exists | `.kekkai.yaml already exists, not overwriting` (unchanged) |
| `.kekkai.yml` exists | `found .kekkai.yml - kekkai only reads .kekkai.yaml; rename it before running 'kekkai init'` |

Typo check first: with both present, the typo message wins.

## 3. Read-only config mount (config present)

Appended to `docker run` args immediately after the workspace bind, within builtin mounts (§7.3 order unchanged):

```text
-v <pwd>/.kekkai.yaml:/workspace/.kekkai.yaml:ro
```

Guarantees inside the container:

| Operation on /workspace/.kekkai.yaml | Result |
|--------------------------------------|--------|
| read | succeeds, host content at startup |
| write / truncate / append | fails (read-only file system) |
| rm / unlink | fails (mount point busy) |
| mv / rename over it | fails (mount point busy) |

## 4. Placeholder mount (config absent)

Host side: temp file (mode 0444) inside `os.MkdirTemp(<UserCacheDir>/kekkai, "config-")`, removed after the container exits. The user cache dir, not `os.TempDir()`: a bind source must be visible to the docker daemon, and on macOS only the home directory is shared into the runtime VM by every recognized runtime (colima does not share `$TMPDIR`). Mount:

```text
-v <cachedir>/kekkai/config-*/.kekkai.yaml:/workspace/.kekkai.yaml:ro
```

Placeholder content — exactly this line plus trailing newline:

```text
# no .kekkai.yaml in workspace - kekkai runs on defaults; create one on the host ('kekkai init') to customize
```

Same read-only guarantees as §3.

Mountpoint remnant: docker materializes the bind target as an empty file inside the workspace bind (i.e. on the host) for the duration of the run. At exit, `up` removes it — guarded: only an empty regular file is deleted, so a real `.kekkai.yaml` the user writes while the sandbox runs survives. After exit the workspace must contain no `.kekkai.yaml` and the cache dir no `config-*` entry.

## 5. Unchanged contracts (regression guards)

- No-config advisory on `up` stays exactly: `warning: no .kekkai.yaml found, using defaults - run 'kekkai init' to customize` (yellow rules unchanged).
- Comments-only/empty config = all defaults (§4.1) — placeholder parses as defaults.
- Image hash inputs unchanged: this feature must not cause rebuilds (§6.1).
- Secrets shadows still assemble after builtin mounts; `secrets.hide: [.kekkai.yaml]` overrides the config mount (later mount wins).
