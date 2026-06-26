# Kekkai

![Kekkai mascot](kekkai-mascot.png)

> *Kekkai* (結界): a barrier/ward in Japanese folklore that confines spirits within a defined space.

A portable, configurable, sandboxed Claude Code container you can run against any project folder.

## What it does

Kekkai drops Claude Code into a sealed-off container so you can let it work autonomously on any project without worrying about what it might touch outside the folder you pointed it at. Every project gets its own isolated sandbox. Network access is locked down to an allowlist you control, so the agent can't reach out to anything you didn't approve. Your git identity, SSH agent, and Claude session carry over automatically. Each project can ship its own `.kekkai.yaml` to extend the allowlist, add mounts, or grant extra capabilities.

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
kekkai config      # show + validate the merged config; --yaml to emit valid YAML
kekkai doctor      # diagnose host setup
kekkai version
```

`kekkai up -- <claude args>` appends to the configured Claude args.

## Configuration

Two layers, merged in order:

1. Built-in defaults (baked into the binary).
2. `./.kekkai.yaml` — project overrides.

Drop any subset of the keys below into the project file. Everything is optional — omit a key to keep the default. Run `kekkai config` to see the merged result.

To scaffold a project file from the current merged config:

```sh
kekkai config --yaml > ./.kekkai.yaml
```

This writes a complete, valid snapshot (defaults + any existing overrides, with `~` and `${VAR}` resolved for this host) that you can trim down to just what you want to change. Re-merging it is idempotent, so it stays valid as-is.

```yaml
# ---- image (bake-time — changing any value triggers a rebuild) ----
# image:
#   base: node:22                          # base image; needs apt-get + a 'node' user to rename
#   claude_code_version: "latest"          # npm dist-tag or pinned version of @anthropic-ai/claude-code
#   docker_cli_version: "27.5.1"           # static docker CLI installed from download.docker.com
#   apt_packages:                          # extra packages appended to the baked-in list (deduped)
#     - less
#     - git
#     - procps
#     - sudo
#     - zsh
#     - gh
#     - iptables
#     - ipset
#     - iproute2
#     - dnsutils
#     - aggregate
#     - jq
#     - nano
#     - curl
#     - ca-certificates

# Example: override base image, append to apt package list.
image:
  base: node:lts
  apt_packages: [htop, golang]

# ---- mounts (runtime — host→container binds; appended across layers) ----
# Each entry: source (host path, ~ and ${VAR} expanded), target (path inside container),
# readonly (default false), optional (skip with warning if source missing; default false).
# $PWD → /workspace and a per-folder bash-history volume → /commandhistory are always added.
# mounts:
#   - { source: "~/.claude", target: "/home/kekkai/.claude" }
#   - { source: "~/.gitconfig", target: "/home/kekkai/.gitconfig", readonly: true, optional: true }
#   - { source: "~/.config/git/allowed_signers", target: "/home/kekkai/.config/git/allowed_signers", readonly: true, optional: true }
#   - { source: "${SSH_AUTH_SOCK}", target: "/ssh-agent", optional: true }

# Example: expose your AWS credentials read-only into the sandbox.
mounts:
  - { source: "~/.aws", target: "/home/kekkai/.aws", readonly: true, optional: true }

# ---- env (runtime — env vars inside the container; KEY=value list, appended across layers) ----
# Later entries override earlier ones with the same KEY. ${VAR} is expanded from your host env
# at load time. WORKSPACE is auto-injected and cannot be set.
# env:
#   - CLAUDE_CONFIG_DIR=/home/kekkai/.claude    # where Claude Code reads/writes its config
#   - NODE_OPTIONS=--max-old-space-size=4096    # node heap ceiling

# Example: add an environment variable
env:
  - NODE_ENV=development

# ---- firewall (runtime — egress allowlist; everything else is REJECTed) ----
# DNS, SSH, loopback, and GitHub CIDRs are always allowed and not configurable.
# firewall:
#   allow_host_lan: true                   # allow the /24 around the default gateway (host.docker.internal etc.)
#   allowed_domains:                       # resolved to A records at startup; appended across layers
#     - registry.npmjs.org
#     - api.anthropic.com

# Example: add the agent reach Terraform registries (appends to built-in list).
firewall:
  allowed_domains:
    - registry.terraform.io
    - releases.hashicorp.com

# ---- claude (runtime — args passed to the claude binary) ----
# Replaces (not appends) on override. `kekkai up -- <extra>` appends <extra> at runtime.
# claude:
#   args: "--dangerously-skip-permissions"

# ---- docker_access (runtime — mount the host docker socket into the sandbox) ----
# When true, bind-mounts /var/run/docker.sock and adds the kekkai user to the socket's GID.
# BYPASSES THE FIREWALL (the daemon does its own networking) and effectively grants host-root
# via the daemon. Enable per-project, not globally.
# docker_access: false
```

### Merge strategy

How merging works of built-in defaults and ./.kekkai.yaml:

- **Lists** under `mounts`, `env`, `firewall.allowed_domains`, and `image.apt_packages` are **added to** the defaults - your entries pile on, they don't replace.
- `env` entries are `KEY=value`; if the same `KEY` appears in a later layer it **overrides** the earlier one.
- **Plain scalars** (e.g. `image.base`, `firewall.allow_host_lan`, `docker_access`) **override** the previous layer.
- `claude.args` is the one exception: it's a string but it **replaces** wholesale, so include every flag you want.
- `~` and `${VAR}` are expanded in any string value. An unset `${VAR}` errors out unless the surrounding mount is marked `optional: true`.
- Unknown keys fail loudly with a line number - typos won't be silently ignored.
