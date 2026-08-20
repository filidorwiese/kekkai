# Contract: builtin tool availability, canonical names, and identity

Scope: `internal/runtime/up.go` (constant), `embed/Dockerfile.tmpl` (symlink), image identity semantics.

## Package set

The builtin apt install line MUST include, in addition to the pre-existing builtins, exactly:

```
unzip zip xz-utils zstd bzip2 file ripgrep fd-find rsync
```

as a distinct comment-labeled group in `builtinAptPackages`. No other packages are added or removed by this feature.

## Canonical command names

From every exec path (`docker exec`, `sh -c`, subprocesses, interactive shell), each command MUST resolve and run in a fresh sandbox:

| Command | Provided by | Mechanism |
|---|---|---|
| `unzip`, `zip`, `xz`, `zstd`, `bzip2`, `file`, `rg`, `rsync` | package binaries on `/usr/bin` | apt install only |
| `fd` | `fd-find` (binary `fdfind`) | `ln -s /usr/bin/fdfind /usr/local/bin/fd` in the existing root symlink RUN block |

`fdfind` additionally keeps working (untouched Debian binary). The symlink line lives in the same RUN block as the `node`/`npm`/`npx`/`claude` links (§6.3 exec-path guarantee) — not a new layer.

## Identity

- `ImageTag` (sha256 of rendered Dockerfile + firewall script): changes once per config from the extended install line + the new symlink line ⇒ every project rebuilds exactly once on upgrade; unchanged configs thereafter reuse the tag. No formula change.
- `ConfigHash` (offline-fallback label): changes via its existing aptPackages input ⇒ the fallback can never serve a pre-upgrade image for the new definition. No formula change.
- A user `apt_packages` entry duplicating a builtin renders a repeated token in the install line; apt collapses it. Configs with such duplicates rebuild once like everyone else and are stable afterwards.

## Non-changes (guarantees)

- No new config keys; strict parsing rejects nothing new and accepts nothing new.
- `init-firewall.sh`, sudoers content, mounts, env, CMD: byte-identical.
- No daemons/services started by any added package; no setuid binaries relied upon.
- Builds without network access to Debian mirrors fail exactly as they do today (build-time fetches are host-network, §6.3).
