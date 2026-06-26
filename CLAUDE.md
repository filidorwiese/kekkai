# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repo.

## What this repo is

Kekkai is a Go CLI (`./cmd/kekkai`) that runs Claude Code inside a per-folder Docker sandbox with a strict egress firewall. Users install one binary, then run `kekkai up` from any project directory.

This is **distribution infrastructure**, not application code. There is no unit-test suite of consequence; validation runs end-to-end against a real docker daemon:

```sh
go build -ldflags "-X main.version=v0.0.0-dev" -o /tmp/kekkai ./cmd/kekkai
cd ~/some-test-project && /tmp/kekkai up
```

## Architecture

### Subcommand dispatch
`cmd/kekkai/main.go` uses stdlib `flag` per subcommand. Each subcommand delegates into `internal/runtime/<name>.go`. Subcommands: `up`, `down`, `shell`, `ps`, `prune`, `doctor`, `version`, `help`.

### Config pipeline
`internal/config/`: two layers merged in order — embedded defaults (`embed/defaults.yml`), then `./.kekkai.{yml,yaml}` (both extensions accepted; having both is an error). There is no user-global config layer; per-project only. Strict YAML (`yaml.v3` + `KnownFields(true)`). Arrays append-only; scalars override; `claude.args` replaces. `~` and `${VAR}` expand after merge; unset `${VAR}` errors unless the surrounding mount has `optional: true`.

Top-level keys: `image`, `mounts`, `env`, `firewall`, `claude`, `docker_access`. `caps` and `firewall.allow_github_meta` are intentionally not user-configurable — `NET_ADMIN`/`NET_RAW` and GitHub meta CIDRs are non-negotiable for the firewall to work.

### Image identity (bake-time inputs)
- Dockerfile template (`embed/Dockerfile.tmpl`) is rendered with `image.*` config values.
- Image hash = `sha256(rendered Dockerfile + embed/init-firewall.sh)`, truncated to 12 hex chars.
- Tag: `kekkai:<hash>`. Built on demand by `internal/image/build.go` only when `docker image inspect` misses.

### Runtime inputs (NOT in image hash)
- `firewall.*` — rendered to `$TMPDIR/kekkai-<name>/firewall.conf`, bind-mounted read-only at `/etc/kekkai/firewall.conf` (sourced by `init-firewall.sh`).
- `docker_access` — when true, `runtime/up.go` stats `/var/run/docker.sock`, mounts it into the container, and adds `--group-add <socket-gid>` so the in-container user can write to it. The docker CLI is **always installed** in the image regardless of the flag.
- `mounts`, `env`, `claude.args` — all applied at `docker run` time.

Why this split: changing allowlist / mounts / docker access must not trigger a rebuild.

### Container identity
- Name: `kekkai-<sanitized-basename($PWD)>-<sha256($PWD)[:8]>`. Deterministic per host folder.
- Authoritative key: label `kekkai.cwd=$PWD`. All `down` / `shell` / `prune` look containers up by that label, **not** by name.
- Additional labels: `kekkai.image_hash`, `kekkai.version`.
- Per-folder bash history: named volume `kekkai-history-<sha256($PWD)[:8]>`.

### In-container environment
- The base image is `node:22`. The `node` user is renamed to `kekkai` at the top of the Dockerfile (`usermod -l … -d /home/kekkai -m`), preserving UID. Home is `/home/kekkai`. Workspace bind: `/workspace`.
- Only `init-firewall.sh` has a sudoers grant (`kekkai ALL=(root) NOPASSWD: /usr/local/bin/init-firewall.sh`). The container CMD is `sudo /usr/local/bin/init-firewall.sh && exec claude $CLAUDE_ARGS`.

### Lifecycle
`kekkai up` runs `docker run --rm -it`; the container is removed on Claude exit, SIGINT, or SIGTERM. Signals are forwarded by `internal/docker/exec.go`. If a container with the same `kekkai.cwd` label already exists, `up` refuses unless `--force` is passed.

## Editing rules

- **Image hash inputs**: only the rendered Dockerfile and `init-firewall.sh`. If you add a new bake-time input, include it in `internal/image/hash.go`. If you add a new runtime input, **do not** include it in the hash — pass it via mount/env.
- **Container identity**: keep `up` and `down`/`shell`/`prune` consistent. Anything that changes how containers are identified must update `internal/runtime/identity.go` and every consumer.
- **Firewall**: never disable the verification curl probes (`example.com` must fail; `api.github.com/zen` must succeed when GitHub meta is on). To allow a new destination, add to `firewall.allowed_domains` in defaults — not by relaxing the script.
- **Docker access bypasses the firewall**: when `docker_access: true`, network egress initiated via `docker pull/push/run` happens on the host network and isn't covered by our iptables/ipset rules. The default is `false`; keep it that way.
- **Sudo inside the container**: only `init-firewall.sh` is sudoers-granted. Don't add others without a strong reason.
- **Image size**: every project folder may build its own variant when `image.*` config changes. Don't add tooling that isn't actually needed by Claude Code sessions.
- **`--dangerously-skip-permissions` is the default for a reason**: the container is the security boundary (firewalled egress, `$PWD` is the only writable host bind). Removing the flag defeats the point.

## Release

`v*` tag pushes trigger `.github/workflows/release.yml`: matrix-build `linux/amd64` + `linux/arm64`, tarball each, generate `SHA256SUMS`, publish a GitHub release. `install.sh` reads the latest tag from the GH API (or honours `KEKKAI_VERSION=…`) and drops the binary in `~/.local/bin/`.

## Out of scope (do not add without discussion)

- macOS / Windows builds.
- `kekkai update` self-updater.
- VS Code / devcontainer-CLI integration.
- Multiple sandboxes per folder.
- Per-project Dockerfile override.
