# Contract: `kekkai self-update` CLI

## Invocation

```
kekkai self-update
```

No flags, no arguments. Extra arguments: `kekkai: unknown command` handling does not apply; the subcommand rejects unexpected args with a usage error, exit 1.

Environment: `KEKKAI_REPO` overrides the `filidorwiese/kekkai` repo slug (testing hook, same as install.sh).

## Output contract (stdout, exact strings)

| Outcome | Output | Exit |
|---|---|---|
| Updated | `Updated kekkai <from> -> <to>` e.g. `Updated kekkai v0.3.0 -> v0.4.0` | 0 |
| Up to date | `You're on the latest version (<installed>)` | 0 |
| Ahead of latest | `You're ahead of the latest release (<installed> > <latest>)` | 0 |

Progress line before download (stdout): `downloading kekkai <tag> (<goos>/<goarch>)` (mirrors install.sh wording).

## Error contract (stderr, exit 1, binary untouched)

| Condition | Message |
|---|---|
| Dev build | `self-update is unavailable on dev builds; install a release: curl -fsSL https://raw.githubusercontent.com/filidorwiese/kekkai/main/install.sh \| sh` |
| Unwritable target | `cannot update: <path> is not writable; fix permissions or reinstall via install.sh` |
| Release lookup failed | `could not determine the latest release of <repo>: <cause>` |
| Rate limited | `GitHub API rate limit hit; try again later` |
| Missing platform artifact | `no <goos>/<goarch> artifact in release <tag>` |
| Checksum mismatch | `checksum verification FAILED for <tarball>` (matches install.sh wording) |
| Download/extract failure | `download failed: <cause>` |

Per cmd/kekkai/main.go convention, errors print without a `kekkai:` prefix.

## Guarantees

- Up-to-date and Ahead outcomes make zero filesystem writes (SC-003).
- Writability preflight runs before the first network byte of the tarball (FR-008).
- Replacement is a same-directory `rename`; interrupt at any point leaves the old or the new binary, never a truncated one (FR-004).
- Temp file `<dir>/.kekkai-update-<pid>` is removed on every failure path.

## Help text

`kekkai help` gains:

```
  self-update update kekkai to the latest release
```
