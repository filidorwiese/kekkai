# Kekkai

![Kekkai mascot](kekkai-mascot.png)

> *Kekkai* (結界): a barrier/ward in Japanese folklore that confines spirits within a defined space.

A portable, sandboxed Claude Code container you can run against any project folder.

## What it does

Kekkai drops Claude Code into a sealed-off container so you can let it work autonomously on any project without worrying about what it might touch outside the folder you pointed it at. Every project gets its own isolated sandbox. Network access is locked down to an allowlist you control, so the agent can't reach out to anything you didn't approve. Your git identity, SSH agent, and Claude session carry over automatically. Each project can ship its own `.kekkai.yml` to extend the allowlist, add mounts, or grant extra capabilities.

One binary on your machine, one command per project.

## Prerequisites

- Docker
- `git`, `curl`.
- Linux x86_64 or aarch64.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/filidorwiese/kekkai/main/install.sh | bash
```

## Use

From any project folder:

```sh
kekkai up          # build + start sandbox, exec claude
kekkai down        # stop and remove the sandbox for this folder
kekkai shell       # open zsh in the running sandbox
kekkai ps          # list running kekkai containers
kekkai prune       # remove orphans (containers, images); --volumes for history vols
kekkai doctor      # diagnose host setup
kekkai version
```

`kekkai up -- <claude args>` appends to the configured Claude args.

## Configuration

Three layers, merged in order:

1. Built-in defaults (baked into the binary).
2. `~/.kekkai.yml` — user-wide overrides.
3. `./.kekkai.yml` — project overrides.

Merge rules: arrays under `image.apt_packages`, `firewall.allowed_domains`, `mounts`, and `caps` **append**. Maps under `env` merge with later values overriding. All other scalars override. `claude.args` **replaces** (not appended) on override. Unknown keys fail with a line number. `~` and `${VAR}` are expanded in every string value at load time; an unset `${VAR}` errors unless its surrounding mount has `optional: true`.

### `image` (bake-time — changing any value triggers a rebuild)

| Key | Type | Default | Notes |
|---|---|---|---|
| `base` | string | `node:22` | Base Docker image. Must include a working `apt-get` and have a `node` user to rename. |
| `apt_packages` | []string | `[less, git, procps, sudo, fzf, zsh, man-db, unzip, gnupg2, gh, iptables, ipset, iproute2, dnsutils, aggregate, jq, nano, vim, python3, python3-pip, python3-venv, pipx, wget, curl, ca-certificates]` | Installed via a single `apt-get install -y`. User entries append; deduped. |
| `git_delta_version` | string | `0.18.2` | [`dandavison/delta`](https://github.com/dandavison/delta) release version (no `v` prefix). |
| `zsh_in_docker_version` | string | `1.2.0` | [`deluan/zsh-in-docker`](https://github.com/deluan/zsh-in-docker) release version. |
| `tflint_version` | string | `0.54.0` | [`terraform-linters/tflint`](https://github.com/terraform-linters/tflint) release version. |
| `docker_cli_version` | string | `27.5.1` | Static docker CLI from `download.docker.com`. Always installed; whether the daemon is reachable is controlled by `docker_access`. |
| `claude_code_version` | string | `latest` | npm dist-tag or version for `@anthropic-ai/claude-code`. |

### `mounts` (runtime — appended across layers)

List of host→container bind mounts. Each entry:

| Subkey | Type | Default | Notes |
|---|---|---|---|
| `source` | string | — (required) | Host path. `~` and `${VAR}` expanded. |
| `target` | string | — (required) | Path inside the container. |
| `readonly` | bool | `false` | Mount read-only (`:ro`). |
| `optional` | bool | `false` | If `${VAR}` in `source` is unset OR the host path doesn't exist, skip with a warning instead of failing. |

Defaults:

| Source | Target | Flags |
|---|---|---|
| `~/.claude` | `/home/kekkai/.claude` | — |
| `~/.gitconfig` | `/home/kekkai/.gitconfig` | readonly, optional |
| `~/.config/git/allowed_signers` | `/home/kekkai/.config/git/allowed_signers` | readonly, optional |
| `${SSH_AUTH_SOCK}` | `/ssh-agent` | optional |

`$PWD → /workspace` and a per-folder bash-history volume → `/commandhistory` are always added and not configurable.

### `env` (runtime — map merge with override)

Environment variables set in the container. Default map:

| Var | Value |
|---|---|
| `CLAUDE_CONFIG_DIR` | `/home/kekkai/.claude` |
| `NODE_OPTIONS` | `--max-old-space-size=4096` |
| `POWERLEVEL9K_DISABLE_GITSTATUS` | `true` |
| `SSH_AUTH_SOCK` | `/ssh-agent` |
| `DEVCONTAINER` | `true` |
| `EDITOR` | `nano` |
| `VISUAL` | `nano` |

`${VAR}` references are expanded against your host environment at load time. `WORKSPACE` is auto-injected as `basename($PWD)` and cannot be set via config.

### `firewall` (runtime — egress allowlist)

Applied by `init-firewall.sh` inside the container at start.

| Key | Type | Default | Notes |
|---|---|---|---|
| `allow_github_meta` | bool | `true` | Fetch CIDRs from `api.github.com/meta` (`.web + .api + .git`) and allow. If GitHub is unreachable at startup the container fails fast. |
| `allow_host_lan` | bool | `true` | Allow the `/24` around the default gateway — handy for local services like `host.docker.internal`. |
| `allowed_domains` | []string | `[registry.npmjs.org, api.anthropic.com]` | Resolved to A records at startup and added to the ipset. Append-only across layers. |

Everything not on the allowlist is REJECTed. DNS (UDP/53), SSH (TCP/22), and loopback are always allowed.

### `caps` (runtime — appended across layers)

Linux capabilities added via `--cap-add`.

Default: `[NET_ADMIN, NET_RAW]` — required for the firewall script to run iptables/ipset. Removing these breaks the firewall.

### `claude` (runtime)

| Key | Type | Default | Notes |
|---|---|---|---|
| `args` | string | `--dangerously-skip-permissions` | Passed to the `claude` binary. **Replaces** on override (not appended). `kekkai up -- <extra>` appends `<extra>` at runtime. |

### `docker_access` (runtime, top-level)

| Key | Type | Default | Notes |
|---|---|---|---|
| `docker_access` | bool | `false` | When `true`, bind-mount `/var/run/docker.sock` and `--group-add` the socket's host GID so the `kekkai` user can run `docker`. **Bypasses the firewall** (the daemon does its own networking on the host) and effectively grants host-root via the daemon. Enable per-project, not globally. |

### Example `./.kekkai.yml`

```yaml
image:
  apt_packages: [htop, jq]

firewall:
  allowed_domains:
    - registry.terraform.io
    - releases.hashicorp.com

mounts:
  - { source: ~/.aws, target: /home/kekkai/.aws, readonly: true, optional: true }

docker_access: true
```
