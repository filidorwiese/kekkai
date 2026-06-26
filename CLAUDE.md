# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repo.

## What this repo is

Kekkai is a Go CLI that runs Claude Code inside a per-folder Docker sandbox with a strict egress firewall. The user installs a single binary on the host, and runs `kekkai up` from any project directory.

This repo is **distribution infrastructure**, not application code. Validation runs end-to-end against the docker daemon — there is no significant unit-test suite. To verify changes:

```sh
go build -ldflags "-X main.version=v0.0.0-dev" -o /tmp/kekkai ./cmd/kekkai
cd ~/some-test-project && /tmp/kekkai up
```

## Architecture

### Subcommand dispatch
`cmd/kekkai/main.go` uses stdlib `flag` per subcommand. Each subcommand delegates into `internal/runtime/<name>.go`.

### Config pipeline
`internal/config/`: three layers merged in order — embedded defaults (`embed/defaults.yml`), `~/.kekkai.yml`, `./.kekkai.yml`. Strict YAML (`yaml.v3` with `KnownFields(true)`). Arrays append-only; scalars override; `claude.args` replaces. `~` and `${VAR}` expanded after merge; unset `${VAR}` errors unless the surrounding mount has `optional: true`.

### Image identity
- The Dockerfile template (`embed/Dockerfile.tmpl`) is rendered with `image.*` config values.
- Image hash = `sha256(rendered Dockerfile + init-firewall.sh)`, truncated to 12 hex chars.
- Tag: `kekkai:<hash>`. Built on demand by `internal/image/build.go` only if `docker image inspect` misses.
- `firewall.*` settings are **not** in the image hash — they are passed at runtime via a bind-mounted `/etc/kekkai/firewall.conf`. This avoids a rebuild every time the allowlist changes.

### Container identity
- Name: `kekkai-<sanitized-basename($PWD)>-<sha256($PWD)[:8]>`. Deterministic per folder.
- Authoritative key: label `kekkai.cwd=$PWD`. All `down` / `shell` / `prune` logic looks containers up by that label, **not** by name.
- Additional labels: `kekkai.image_hash`, `kekkai.version`.
- Per-folder bash history: named volume `kekkai-history-<sha256($PWD)[:8]>`.

### Lifecycle
`kekkai up` runs `docker run --rm -it` — the container is removed on Claude exit, SIGINT, or SIGTERM (we forward signals in `internal/docker/exec.go`). If a container for the same `$PWD` already exists, `up` refuses unless `--force` is passed.

### Firewall runtime
`embed/init-firewall.sh` is baked into the image. At `up` time, `internal/firewall/render.go` writes a config file to `$TMPDIR/kekkai-<name>/firewall.conf` and bind-mounts it read-only at `/etc/kekkai/firewall.conf`. The container's CMD is `sudo /usr/local/bin/init-firewall.sh && exec claude $CLAUDE_ARGS`. The sudoers grant for that single script is baked into the image.

## Editing rules

- **Image hash inputs**: only the rendered Dockerfile and the firewall script. If you add a new bake-time input (e.g. a new embedded file copied into the image), include it in `internal/image/hash.go`. If you add a new runtime input, **do not** include it in the hash — pass it via mount or env.
- **Container identity**: keep `up` and `down`/`shell`/`prune` consistent. Anything that changes how containers are identified must update the label scheme in `internal/runtime/identity.go` and every consumer.
- **Firewall**: never disable the verification curl probes (`example.com` must fail; `api.github.com/zen` must succeed when GitHub meta is on). To allow a new destination, add to `firewall.allowed_domains` in defaults — not by relaxing the script.
- **Sudo inside the container**: only `init-firewall.sh` has the sudoers grant. Do not add others without a strong reason.
- **Image size**: every project folder builds its own variant whenever config changes. Don't add tooling that isn't actually needed by Claude Code sessions.
- **Why `--dangerously-skip-permissions` is the default**: the container itself is the security boundary (firewalled egress, `$PWD` is the only writable host bind). The flag is what makes kekkai useful as an unattended Claude environment.

## Release

`v*` tag pushes trigger `.github/workflows/release.yml`, which builds `linux/amd64` and `linux/arm64` binaries, tars them, generates a `SHA256SUMS`, and publishes a GitHub release. The `install.sh` script pulls the latest tag from the GH API (or honours `KEKKAI_VERSION=…`).

## Out of scope (do not add without discussion)

- macOS / Windows builds.
- `kekkai update` self-updater.
- VS Code / devcontainer-CLI integration.
- Multiple sandboxes per folder.
- Per-project Dockerfile override.
